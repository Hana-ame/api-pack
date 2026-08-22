package proxies

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func sleepStart(t *testing.T, port string) {
	t.Helper()
	time.Sleep(400 * time.Millisecond)
}

func writeCfg(t *testing.T, path string, threshold int, ratio float64, srcs []configSource) {
	t.Helper()
	cfg := twimgV2Config{Sources: srcs}
	if threshold >= 0 {
		cfg.QPSThreshold = &threshold
	}
	if ratio >= 0 {
		cfg.MaxDivertRatio = &ratio
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// 发一个请求；CN 且非 302 时会走反代（上游不可达，客户端超时）
func request(t *testing.T, url string, timeout time.Duration) (*http.Response, error) {
	t.Helper()
	hc := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不跟随 302,直接看 Location
		},
	}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Cf-Ipcountry", "CN")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp, nil
}

// 302 分流 + 计数器：limit=3 的节点集中用满,前 3 次 302,第 4 次起 fallback 反代
func TestTwimgV2DivertAndCounter(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.json")
	writeCfg(t, cfgPath, 0, 1.0, []configSource{ // qps_threshold=0 + ratio=1 -> 全分流
		{Domain: "mirror.test", Limit: 3, Enabled: true},
	})
	t.Setenv("TWIMG_V2_CONFIG", cfgPath)
	go TwimgProxyV2("127.0.0.1:18081")
	sleepStart(t, "18081")

	for i := 1; i <= 3; i++ {
		// 带 query 参数的 .mp4 也要正确命中分流,且 302 Location 保留原 query
		resp, err := request(t, "http://127.0.0.1:18081/tweet_video/1.mp4?tag=8&x=1", time.Second)
		if err != nil {
			t.Fatalf("第 %d 次应 302,却报错: %v", i, err)
		}
		if resp.StatusCode != 302 {
			t.Fatalf("第 %d 次应 302,got %d", i, resp.StatusCode)
		}
		if loc := resp.Header.Get("Location"); loc != "https://mirror.test/tweet_video/1.mp4?tag=8&x=1" {
			t.Fatalf("302 Location 错误: %s", loc)
		}
		if cc := resp.Header.Get("Cache-Control"); cc == "" {
			t.Fatal("302 必须带 Cache-Control 禁止缓存")
		}
	}

	// 计数器到 limit:第 4 次不应再 302(fallback 自建反代,上游不可达 -> 超时)
	if resp, err := request(t, "http://127.0.0.1:18081/tweet_video/1.mp4?tag=8", 1500*time.Millisecond); err == nil {
		if resp.StatusCode == 302 {
			t.Fatal("limit 用尽后不应再 302")
		}
	}
}

// 集中使用：节点按配置顺序排队,前一个用满 limit 才切下一个
func TestTwimgV2Concentrated(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.json")
	writeCfg(t, cfgPath, 0, 1.0, []configSource{
		{Domain: "a.mirror.test", Limit: 2, Enabled: true},
		{Domain: "b.mirror.test", Limit: 100, Enabled: true},
	})
	t.Setenv("TWIMG_V2_CONFIG", cfgPath)
	go TwimgProxyV2("127.0.0.1:18085")
	sleepStart(t, "18085")

	want := []string{
		"https://a.mirror.test/tweet_video/1.mp4",
		"https://a.mirror.test/tweet_video/1.mp4",
		"https://b.mirror.test/tweet_video/1.mp4",
		"https://b.mirror.test/tweet_video/1.mp4",
	}
	for i, w := range want {
		resp, err := request(t, "http://127.0.0.1:18085/tweet_video/1.mp4", time.Second)
		if err != nil {
			t.Fatalf("第 %d 次应 302,却报错: %v", i+1, err)
		}
		if loc := resp.Header.Get("Location"); loc != w {
			t.Fatalf("第 %d 次 Location=%s,期望 %s", i+1, loc, w)
		}
	}
}

// 本体兜底：ratio=0.5 时请求交错抽样,偶数次 302、奇数次由本体自扛(反代,上游不可达 -> 超时)
func TestTwimgV2OriginShare(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.json")
	writeCfg(t, cfgPath, 0, 0.5, []configSource{
		{Domain: "mirror.test", Limit: 100, Enabled: true},
	})
	t.Setenv("TWIMG_V2_CONFIG", cfgPath)
	go TwimgProxyV2("127.0.0.1:18086")
	sleepStart(t, "18086")

	for i := 1; i <= 5; i++ {
		resp, err := request(t, "http://127.0.0.1:18086/tweet_video/1.mp4", 1500*time.Millisecond)
		if i%2 == 0 {
			if err != nil || resp.StatusCode != 302 {
				t.Fatalf("第 %d 次应 302,got err=%v resp=%v", i, err, resp)
			}
			if loc := resp.Header.Get("Location"); loc != "https://mirror.test/tweet_video/1.mp4" {
				t.Fatalf("第 %d 次 Location 错误: %s", i, loc)
			}
		} else if err == nil && resp.StatusCode == 302 {
			t.Fatalf("第 %d 次是本体兜底,不应 302", i)
		}
	}
}

// 禁用节点:enabled=false 后不走 302
func TestTwimgV2DisableNode(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.json")
	writeCfg(t, cfgPath, 0, 1.0, []configSource{
		{Domain: "mirror.test", Limit: 10, Enabled: false},
	})
	t.Setenv("TWIMG_V2_CONFIG", cfgPath)
	go TwimgProxyV2("127.0.0.1:18082")
	sleepStart(t, "18082")

	if resp, err := request(t, "http://127.0.0.1:18082/tweet_video/1.mp4", 1*time.Second); err == nil {
		if resp.StatusCode == 302 {
			t.Fatal("被禁用的节点不应产生 302")
		}
	}
}

// 非 CN 直接 302 官方源;CN 低 QPS 用 /tweet_video/1.mp4 验证走自建反代(不是 302)
func TestTwimgV2NonCNAndSelfProxy(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.json")
	writeCfg(t, cfgPath, 10, 1.0, []configSource{
		{Domain: "mirror.test", Limit: 100, Enabled: true},
	})
	t.Setenv("TWIMG_V2_CONFIG", cfgPath)
	go TwimgProxyV2("127.0.0.1:18084")
	sleepStart(t, "18084")

	// 非 CN -> 直接 302 官方源
	hc := &http.Client{
		Timeout: time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不跟随 302,直接看 Location
		},
	}
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:18084/foo.jpg", nil)
	req.Header.Set("Cf-Ipcountry", "US")
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 302 || resp.Header.Get("Location") != "https://pbs.twimg.com/foo.jpg" {
		t.Fatalf("非CN 302 错误: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	req, _ = http.NewRequest(http.MethodGet, "http://127.0.0.1:18084/tweet_video/1.mp4?x=1", nil)
	req.Header.Set("Cf-Ipcountry", "JP")
	resp, err = hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 302 || resp.Header.Get("Location") != "https://video.twimg.com/tweet_video/1.mp4?x=1" {
		t.Fatalf("非CN video 302 错误: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}

	// CN 低 QPS -> 自建反代(favicon.ico,上游不可达 -> 超时,必非 302)
	resp, err = request(t, "http://127.0.0.1:18084/tweet_video/1.mp4", 1500*time.Millisecond)
	if err == nil && resp.StatusCode == 302 {
		t.Fatal("CN 低 QPS 不应 302 分流")
	}
}

// 非 mp4 绝不分流：即使 threshold=0 + ratio=1.0（mp4 全分流），
// .jpg / 无扩展名 / query 含 =jpg 的请求也一律直接代理 pbs.twimg.com（上游不可达 -> 超时）
func TestTwimgV2ImageNeverDiverts(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.json")
	writeCfg(t, cfgPath, 0, 1.0, []configSource{
		{Domain: "mirror.test", Limit: 100, Enabled: true},
	})
	t.Setenv("TWIMG_V2_CONFIG", cfgPath)
	go TwimgProxyV2("127.0.0.1:18087")
	sleepStart(t, "18087")

	paths := []string{"/foo.jpg", "/media/abc", "/media/abc?format=jpg&name=large"}
	for _, p := range paths {
		if resp, err := request(t, "http://127.0.0.1:18087"+p, 1*time.Second); err == nil {
			if resp.StatusCode == 302 {
				t.Fatalf("非 mp4 请求 %s 不应 302 分流,Location=%s", p, resp.Header.Get("Location"))
			}
		}
	}
}

// 分流比例：空闲全部本体；QPS 超阈值后只分流超过的部分,且不超过 max_divert_ratio
func TestDivertPermilleFor(t *testing.T) {
	m := &divertManager{qpsThreshold: 10, divertPermille: 800} // 阈值 10,最大比例 80%
	cases := []struct {
		qps  int
		want int
	}{
		{5, 0},     // 空闲,不分流
		{10, 0},    // 恰好阈值,不分流
		{15, 333},  // 只分流超过的 5/15
		{20, 500},  // 10/20
		{50, 800},  // 40/50=800,到上限
		{100, 800}, // 90/100=900,封顶 800
		{1000, 800},
	}
	for _, c := range cases {
		if got := m.divertPermilleFor(c.qps); got != c.want {
			t.Fatalf("qps=%d divertPermille=%d,期望 %d", c.qps, got, c.want)
		}
	}

	// 阈值 0:所有请求都属于"超过的部分",比例由 max_divert_ratio 决定
	m0 := &divertManager{qpsThreshold: 0, divertPermille: 500}
	if got := m0.divertPermilleFor(1); got != 500 {
		t.Fatalf("threshold=0 qps=1 应分流 500,got %d", got)
	}
	if got := m0.divertPermilleFor(100); got != 500 {
		t.Fatalf("threshold=0 qps=100 应分流 500,got %d", got)
	}
}

// 计数器模块:上限禁用、清零
func TestRedirectSourceUnit(t *testing.T) {
	s := &redirectSource{domain: "x"}
	s.limit.Store(3)
	s.enabled.Store(true)
	for i := 0; i < 3; i++ {
		if s.disabled() {
			t.Fatal("limit 内不应禁用")
		}
		s.hit()
	}
	if !s.disabled() {
		t.Fatal("到达 limit 后应禁用")
	}
	// 提高 limit 复活
	s.limit.Store(99)
	if s.disabled() {
		t.Fatal("提高 limit 后应恢复")
	}
	// 清零后重新计数
	s.count.Store(0)
	s.limit.Store(3)
	if s.disabled() {
		t.Fatal("清零后不应禁用")
	}
	// enabled=false 立即禁用
	s.enabled.Store(false)
	if !s.disabled() {
		t.Fatal("enabled=false 应禁用")
	}
}

// 分流域禁用窗口:UTC 20:00~24:00 关闭
func TestDivertWindow(t *testing.T) {
	utc := func(hour int) time.Time {
		return time.Date(2026, 1, 1, hour, 30, 0, 0, time.UTC)
	}
	if !divertWindowAllows(utc(0)) {
		t.Fatal("0:00 应允许")
	}
	if !divertWindowAllows(utc(19)) {
		t.Fatal("19:59 应允许")
	}
	if divertWindowAllows(utc(20)) {
		t.Fatal("20:00 应禁止")
	}
	if divertWindowAllows(utc(23)) {
		t.Fatal("23:00 应禁止")
	}
}

// videoGate 单元测试（按每 IP 视频累计下载量判定）：
// 累计未达配额直接放行；累计达到配额当场永久进池；
// 池内只有 1 个服务槽，其余请求挂起等待（不发响应），等槽期间客户端断开则放弃
// （发现背景：twimgV2 按用户要求实现「每 IP 10GB 配额、超限进限速池、
// 池内单并发、hangup+keepalive 排队、不允许 429、无全局设限」，
// 验证字节累计与槽位归还、达配额即进池、ctx 取消退出排队）
func TestVideoGate(t *testing.T) {
	newGate := func(quota int64) *videoGate {
		return &videoGate{
			perIPQuota: quota,
			perIP:      make(map[string]int64),
			pool:       make(map[string]struct{}),
			poolSlots:  make(chan struct{}, 1),
		}
	}

	t.Run("配额关闭直接放行", func(t *testing.T) {
		g := newGate(0)
		r1, pooled := g.acquire(context.Background(), "a")
		if r1 == nil || pooled {
			t.Fatal("配额关闭应直接放行且非池内")
		}
		r1(100)
		if _, ok := g.pool["a"]; ok {
			t.Fatal("配额关闭不应进池")
		}
	})

	t.Run("累计未达配额不进池", func(t *testing.T) {
		g := newGate(1000)
		r1, pooled := g.acquire(context.Background(), "a")
		if r1 == nil || pooled {
			t.Fatal("配额内应直接放行")
		}
		r1(600)
		r2, pooled := g.acquire(context.Background(), "a")
		if r2 == nil || pooled {
			t.Fatal("累计未达配额应继续放行")
		}
		r2(300) // 累计 900 < 1000,仍未达
		if _, ok := g.pool["a"]; ok {
			t.Fatal("未达配额不应进池")
		}
	})

	t.Run("累计达配额当场进池且永久", func(t *testing.T) {
		g := newGate(1000)
		r1, _ := g.acquire(context.Background(), "a")
		r1(1000) // 达配额 -> 当场进池
		if _, ok := g.pool["a"]; !ok {
			t.Fatal("达配额应进池")
		}
		// 进池后后续请求走池（一直有效），槽空闲立即拿到
		_, pooled := g.acquire(context.Background(), "a")
		if !pooled {
			t.Fatal("池内成员后续请求应仍然走池")
		}
		// 0 字节不触发进池
		g2 := newGate(1000)
		r2, _ := g2.acquire(context.Background(), "b")
		r2(0)
		if _, ok := g2.pool["b"]; ok {
			t.Fatal("0 字节不应进池")
		}
	})

	t.Run("池内单并发,其余挂起排队", func(t *testing.T) {
		g := newGate(0) // 先手动塞池,等价于已超配额
		g.pool["a"] = struct{}{}
		g.pool["b"] = struct{}{}

		type result struct {
			release func(int64)
			pooled  bool
		}
		// a 拿到唯一槽
		r1, pooled := g.acquire(context.Background(), "a")
		if r1 == nil || !pooled {
			t.Fatal("池内应走池")
		}
		// b 请求：槽被占 -> 挂起等待，不得返回
		ch1 := make(chan result, 1)
		go func() {
			r2, pooled := g.acquire(context.Background(), "b")
			ch1 <- result{r2, pooled}
		}()
		select {
		case <-ch1:
			t.Fatal("槽被占用时应挂起等待")
		case <-time.After(100 * time.Millisecond):
		}

		// 客户端断开（ctx 取消）-> 放弃排队返回 nil
		ctx, cancel := context.WithCancel(context.Background())
		ch2 := make(chan result, 1)
		go func() {
			r3, pooled := g.acquire(ctx, "b")
			ch2 <- result{r3, pooled}
		}()
		time.Sleep(50 * time.Millisecond)
		cancel()
		select {
		case res := <-ch2:
			if res.release != nil {
				t.Fatal("ctx 取消后 acquire 应返回 nil release")
			}
		case <-time.After(time.Second):
			t.Fatal("ctx 取消后排队应退出")
		}

		// 槽释放 -> 排队者拿到槽照常服务
		r1(0)
		select {
		case res := <-ch1:
			if res.release == nil || !res.pooled {
				t.Fatal("等槽成功应返回 pooled release")
			}
			res.release(0)
		case <-time.After(time.Second):
			t.Fatal("槽释放后排队者应被唤醒")
		}
	})

	t.Run("不同IP配额独立", func(t *testing.T) {
		g := newGate(1000)
		r1, _ := g.acquire(context.Background(), "a")
		r1(1000) // a 达配额进池
		if _, ok := g.pool["a"]; !ok {
			t.Fatal("a 应进池")
		}
		r2, pooled := g.acquire(context.Background(), "b")
		if r2 == nil || pooled {
			t.Fatal("b 配额独立,应正常放行")
		}
		r2(999)
		if _, ok := g.pool["b"]; ok {
			t.Fatal("b 未达配额不应进池")
		}
	})
}

// videoGate 端到端：视频上游可注入时，IP 累计下载超配额（在流结束时判定）
// 后永久进池；池内第 2 路占用唯一槽，第 3 路挂起排队（不达上游）；
// 槽释放后照常服务，全程无 429/5xx
func TestTwimgV2VideoGateE2E(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.json")
	cfg := twimgV2Config{
		Sources:        []configSource{{Domain: "mirror.test", Limit: 100, Enabled: true}},
		QPSThreshold:   intPtr(1000), // 高阈值:不触发 302 分流,走本体反代
		MaxDivertRatio: float64Ptr(1.0),
		VideoIPQuotaGB: float64Ptr(0.00001), // 约 10KB 配额:第一路就超
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TWIMG_V2_CONFIG", cfgPath)

	// 假上游：首个请求全速写完；后续请求写满 2MB 后阻塞等 release
	// （模拟长视频下载中，让"排队中"可被观察）
	const size int64 = 4 << 20
	var reqCount atomic.Int64
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock) // 失败路径也要放行,否则 httptest.Close 等挂起连接卡死测试
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := reqCount.Add(1)
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		w.WriteHeader(http.StatusOK)
		written := int64(0)
		for written < size {
			chunk := int64(1 << 20)
			if n > 1 && written >= 2<<20 {
				<-release
			}
			if size-written < chunk {
				chunk = size - written
			}
			wn, err := w.Write(make([]byte, chunk))
			written += int64(wn)
			if err != nil {
				return
			}
		}
	}))
	defer upstream.Close()
	t.Setenv("TWIMG_V2_UPSTREAM_VIDEO", upstream.URL)

	go TwimgProxyV2("127.0.0.1:18089")
	sleepStart(t, "18089")

	client := func(ip string) (int, error) {
		req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:18089/tweet_video/a.mp4", nil)
		req.Header.Set("Cf-Ipcountry", "CN")
		req.Header.Set("Cf-Connecting-Ip", ip)
		resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if err != nil {
			return 0, err
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp.StatusCode, nil
	}

	// 第 1 路：配额内放行，全速完成 -> 流结束时累计超配额，当场进池
	done1 := make(chan int, 1)
	go func() { s, _ := client("9.9.9.9"); done1 <- s }()
	select {
	case s := <-done1:
		if s != http.StatusOK {
			t.Fatalf("第 1 路应 200, got %d", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("第 1 路超时未完成")
	}

	// 第 2 路：已进池 -> 槽空闲,立即占用,开始服务（上游阻塞在 2MB）
	done2 := make(chan int, 1)
	go func() { s, _ := client("9.9.9.9"); done2 <- s }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && reqCount.Load() < 2 {
		time.Sleep(20 * time.Millisecond)
	}
	if reqCount.Load() < 2 {
		t.Fatal("第 2 路未开始服务")
	}

	// 第 3 路：池内且槽被占 -> 挂起排队,短时间内不应到达上游
	done3 := make(chan int, 1)
	go func() { s, _ := client("9.9.9.9"); done3 <- s }()
	select {
	case s := <-done3:
		t.Fatalf("第 3 路应挂起排队,却已完成(status=%d)", s)
	case <-time.After(500 * time.Millisecond):
	}
	if got := reqCount.Load(); got != 2 {
		t.Fatalf("排队期间不应有第 3 路到达上游, reqCount=%d", got)
	}

	// 释放第 2 路 -> 第 3 路拿到槽,照常服务完成(200),全程无 429/5xx
	unblock()
	select {
	case s := <-done2:
		if s != http.StatusOK {
			t.Fatalf("第 2 路应 200, got %d", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("第 2 路超时未完成")
	}
	select {
	case s := <-done3:
		if s != http.StatusOK {
			t.Fatalf("第 3 路等槽后应 200, got %d", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("第 3 路等槽后超时未完成")
	}
}

func intPtr(v int) *int             { return &v }
func float64Ptr(v float64) *float64 { return &v }
