package proxy

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

func sleepStart(t *testing.T, port string) {
	t.Helper()
	time.Sleep(400 * time.Millisecond)
}

// 注意：分流配置（twimg_v2.json / TWIMG_V2_CONFIG）与 videoGate 的接线已被移除，
// TwimgProxyV2 不再读取配置，因此这里不再有写配置文件的帮助函数

// requestWithReferer 带指定 Cf-Ipcountry 与 Referer 发请求；不跟随 302，直接看 Location。
// country / referer 为空串表示不带该头。
func requestWithReferer(t *testing.T, url, country, referer string, timeout time.Duration) (*http.Response, error) {
	t.Helper()
	hc := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不跟随 302,直接看 Location
		},
	}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if country != "" {
		req.Header.Set("Cf-Ipcountry", country)
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp, nil
}

// Referer 白名单防盗链（TwimgProxyV2 v2Handler）：
//   - 非白名单 Referer（含空 / 未知域名）→ 302 到 https://video.twimg.com<path>?<query>，
//     对 .mp4 与图片一视同仁，且不分 Cf-Ipcountry
//   - 白名单 Referer → 本体反代（不 302；上游不可达时超时，亦证明未被 302）
func TestTwimgV2RefererGate(t *testing.T) {
	go TwimgProxyV2("127.0.0.1:18091")
	sleepStart(t, "18091")

	paths := []string{
		"/tweet_video/1.mp4?tag=8",
		"/ext_tw_video/123/pu/vid/320x180/abc.mp4?tag=12",
		"/amplify_video/456/vid/avc1/720x1280/xyz.MP4",
		"/foo.jpg",
		"/media/abc?format=jpg&name=large",
		"/profile_images/1/x.png",
	}

	// 非白名单来源（含空 Referer）一律 302 到 video.twimg.com 同 path+query
	for _, referer := range []string{"", "https://example.com/", "https://evil.com/x"} {
		for _, country := range []string{"CN", "US", ""} {
			for _, p := range paths {
				resp, err := requestWithReferer(t, "http://127.0.0.1:18091"+p, country, referer, time.Second)
				if err != nil {
					t.Fatalf("referer=%q country=%q %s 应 302,却报错: %v", referer, country, p, err)
				}
				if resp.StatusCode != 302 {
					t.Fatalf("referer=%q country=%q %s 应 302, got %d", referer, country, p, resp.StatusCode)
				}
				if loc, want := resp.Header.Get("Location"), "https://video.twimg.com"+p; loc != want {
					t.Fatalf("referer=%q %s Location=%s,期望 %s", referer, p, loc, want)
				}
				if cc := resp.Header.Get("Cache-Control"); cc == "" {
					t.Fatal("302 必须带 Cache-Control 禁止缓存")
				}
			}
		}
	}

	// 白名单来源 → 不 302（本体反代；上游不可达时超时,符合预期）
	whitelist := []string{
		"https://x.moonchan.xyz/u/foo",
		"https://x.810114.xyz/",
		"https://xx.810114.xyz:443/",
		"https://x.4545810.xyz/media",
		"https://xx.4545810.xyz/",
	}
	for _, referer := range whitelist {
		for _, p := range paths {
			resp, err := requestWithReferer(t, "http://127.0.0.1:18091"+p, "", referer, 500*time.Millisecond)
			if err != nil {
				continue // 反代上游不可达 -> 超时,证明没被 302
			}
			if resp.StatusCode == 302 {
				t.Fatalf("白名单 referer=%q %s 不应 302,Location=%s", referer, p, resp.Header.Get("Location"))
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
