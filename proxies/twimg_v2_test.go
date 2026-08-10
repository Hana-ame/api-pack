package proxies

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
