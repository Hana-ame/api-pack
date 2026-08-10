package proxies

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	"github.com/gin-gonic/gin"
)

// StreamProxy 创建一个原生的流式反向代理
// 1. 实现 TCP 背压：当 Client 接收慢时，Write 阻塞导致 Read 阻塞，迫使上游 Fetch 自动降速。
// 2. 实现 级联取消：客户端断开时自动掐断发往目标服务器的 TCP 请求。
// 3. 使用 Go 1.20+ 推荐的 Rewrite 替代被弃用的 Director，提升安全性。
func StreamProxy(targetURL string, headerProcesser func(http.Header) http.Header) gin.HandlerFunc {
	target, err := url.Parse(targetURL)
	if err != nil {
		panic(err) // 启动时地址配置错误直接抛出
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			// SetURL 自动路由到目标地址，并正确处理原 URL 的 Path 和 Query
			pr.SetURL(target)

			// 代理到外部 CDN 时，重写 Host 为目标服务器极其重要，否则会被 CDN 拒绝
			pr.Out.Host = target.Host

			// 处理自定义 Header（例如修改 Referer 防盗链）
			if headerProcesser != nil {
				pr.Out.Header = headerProcesser(pr.Out.Header)
			}

			// （可选）如果你想把客户端的真实 IP 传给目标，可以解除下一行的注释
			// pr.SetXForwarded()
		},
	}

	return func(c *gin.Context) {
		// 检查客户端是否在代理开始前就已经断开连接
		select {
		case <-c.Request.Context().Done():
			// 499 (Client Closed Request) 表示客户端提前断开
			c.AbortWithStatus(499)
			return
		default:
		}

		// 执行流式转发，自带断开检测和防非对称消耗能力
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func TwimgProxy(addr string) error {
	// dailyLimiter := NewIPRateLimiter(2000)

	if addr == "" {
		return fmt.Errorf("addr is empty")
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())
	// 挂载我们自己写的限流中间件
	// r.Use(dailyLimiter.Middleware())

	headerProcesser := func(h http.Header) http.Header {
		h.Set("Referer", "https://x.com")
		return h
	}

	twimgProxy := StreamProxy("https://pbs.twimg.com", headerProcesser)
	videoProxy := StreamVideoProxy("https://video.twimg.com", headerProcesser)

	handler := func(c *gin.Context) {
		path := c.Request.URL.Path
		country := c.GetHeader("Cf-Ipcountry")
		var host string
		var isVideo bool

		if strings.HasPrefix(path, "/tweet_video/") || strings.HasPrefix(path, "/ext_tw_video/") || strings.HasPrefix(path, "/amplify_video/") {
			host = "video.twimg.com"
			isVideo = true
		} else {
			host = "pbs.twimg.com"
		}

		// Cloudflare 地域分流检测
		if country != "" && country != "CN" {
			// 重要：禁止缓存重定向响应
			c.Header("Cache-Control", "no-cache, no-store, private")
			c.Redirect(http.StatusFound, "https://"+host+c.Request.URL.String())
			return
		}

		if isVideo {
			videoProxy(c)
		} else {
			twimgProxy(c)
		}
	}

	r.GET("/*any", handler)
	r.HEAD("/*any", handler)

	return r.Run(addr)
}

// ---- TwimgProxyV2：镜像分流版 twimg 反代 ----

const (
	// 分流源默认上限
	redirectSourceLimit = 140_000
	// 默认触发 302 分流的 QPS 阈值（配置缺省时使用）
	defaultDivertQPS = 10
	// QPS 超过阈值后的默认最大分流比例：即使 QPS 爆掉，本体也自扛 20%
	defaultMaxDivertRatio = 0.8
	// 分流域禁用窗口起点（UTC 小时）：UTC 20:00~24:00 仅自建反代 + 非CN 302
	divertBlockStartHour = 20
	// 分流配置文件（可用环境变量 TWIMG_V2_CONFIG 指定路径）。
	// 服务器每日重启，启动时加载一次即可：增删节点 / 改 limit / 改阈值 / 禁用节点
	// 都只改这个 JSON，无需重新编译
	defaultConfigPath = "twimg_v2.json"
)

// configSource 分流配置中的一个节点
type configSource struct {
	Domain  string `json:"domain"`
	Limit   int64  `json:"limit"`
	Enabled bool   `json:"enabled"`
}

// twimgV2Config 分流配置文件结构：
//
//	{
//	  "qps_threshold": 10,      // 空闲时全部由本体代理；QPS 超过此值后，只把超过的部分分流
//	  "max_divert_ratio": 0.8,  // 最多分流比例：即使 QPS 爆掉，本体也始终自扛 1-该值 的流量；填 0 表示不分流
//	  "sources": [
//	    {"domain": "twimg.810114.xyz", "limit": 100000, "enabled": true},
//	    {"domain": "114514.beer", "limit": 100000, "enabled": true}
//	  ]
//	}
type twimgV2Config struct {
	QPSThreshold   *int           `json:"qps_threshold"`
	MaxDivertRatio *float64       `json:"max_divert_ratio"`
	Sources        []configSource `json:"sources"`
}

// redirectSource 一个 302 分流节点（镜像域名）。
// limit/enabled 启动时从配置读取，count 只增不减，直到每日 UTC 0 点重置
type redirectSource struct {
	domain  string
	limit   atomic.Int64
	enabled atomic.Bool
	count   atomic.Int64
}

func (s *redirectSource) disabled() bool {
	return !s.enabled.Load() || s.count.Load() >= s.limit.Load()
}

// hit 记录一次 302 跳转，首次到达上限时打印日志
func (s *redirectSource) hit() {
	n := s.count.Add(1)
	if n == s.limit.Load() {
		log.Printf("[twimg-v2] source %s 跳转次数达到 %d，已禁用", s.domain, n)
	}
}

// divertWindowAllows 判断当前时间是否在分流域禁用窗口（UTC 20:00~24:00）之外
func divertWindowAllows(now time.Time) bool {
	return now.UTC().Hour() < divertBlockStartHour
}

// divertManager 管理分流节点配置与计数。
// 配置在启动时读取一次（服务器每日重启），此后 sources 不再变更，
// 请求路径只读、resetLoop 只写 count（atomic），无需加锁
type divertManager struct {
	sources        []*redirectSource // 保持配置顺序
	qpsThreshold   int
	divertPermille int          // 最大分流千分比（max_divert_ratio*1000），超出 QPS 阈值部分分流的封顶值
	tick           atomic.Int64 // 分流抽样计数器
}

// newDivertManager 启动时读取一次分流配置；
// 文件缺失或解析失败时回退到内置默认节点
func newDivertManager(path string) *divertManager {
	m := &divertManager{
		qpsThreshold:   defaultDivertQPS,
		divertPermille: int(defaultMaxDivertRatio * 1000),
	}

	var cfg twimgV2Config
	data, err := os.ReadFile(path)
	switch {
	case err != nil:
		log.Printf("[twimg-v2] 读取分流配置 %s 失败: %v，使用内置默认节点", path, err)
		cfg.Sources = []configSource{
			{Domain: "twimg.810114.xyz", Limit: redirectSourceLimit, Enabled: true},
			{Domain: "114514.beer", Limit: redirectSourceLimit, Enabled: true},
		}
	case json.Unmarshal(data, &cfg) != nil:
		log.Printf("[twimg-v2] 解析分流配置 %s 失败: %v，使用内置默认节点", path, err)
		cfg.Sources = []configSource{
			{Domain: "twimg.810114.xyz", Limit: redirectSourceLimit, Enabled: true},
			{Domain: "114514.beer", Limit: redirectSourceLimit, Enabled: true},
		}
	}
	if cfg.QPSThreshold != nil {
		m.qpsThreshold = *cfg.QPSThreshold
	}
	if cfg.MaxDivertRatio != nil {
		ratio := *cfg.MaxDivertRatio
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		m.divertPermille = int(ratio * 1000)
	}
	for _, cs := range cfg.Sources {
		if cs.Domain == "" {
			continue
		}
		limit := cs.Limit
		if limit <= 0 {
			limit = redirectSourceLimit
		}
		s := &redirectSource{domain: cs.Domain}
		s.limit.Store(limit)
		s.enabled.Store(cs.Enabled)
		m.sources = append(m.sources, s)
	}
	return m
}

// resetCounters 清零所有计数（每日 UTC 0 点调用）
func (m *divertManager) resetCounters() {
	for _, s := range m.sources {
		s.count.Store(0)
	}
	log.Printf("[twimg-v2] 分流计数已全部重置")
}

// resetLoop 定时在 UTC 0 点重置所有节点计数
func (m *divertManager) resetLoop() {
	for {
		now := time.Now().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		time.Sleep(time.Until(next))
		m.resetCounters()
	}
}

// divertPermilleFor 计算当前 QPS 下应分流的千分比：
// 空闲（QPS<=阈值）时全部走本体；QPS 超阈值后只分走“超过的那部分”流量
// （(qps-threshold)/qps），且不超过 max_divert_ratio，保证本体永远自扛一部分
func (m *divertManager) divertPermilleFor(currentQPS int) int {
	if currentQPS <= m.qpsThreshold {
		return 0
	}
	p := (currentQPS - m.qpsThreshold) * 1000 / currentQPS
	if p > m.divertPermille {
		p = m.divertPermille
	}
	return p
}

// shouldDivert 判断当前请求是否走 302 分流：
// 分流窗口内、有未禁用的节点，且按“超过阈值的部分”抽样命中。
// 节点集中使用：按配置顺序取第一个未禁用的节点，用满 limit 再切下一个
func (m *divertManager) shouldDivert(c *gin.Context, currentQPS int) bool {
	if !divertWindowAllows(time.Now()) {
		return false
	}

	p := m.divertPermilleFor(currentQPS)
	if p <= 0 {
		return false
	}

	// 按比例交错抽样：每 1000 个请求恰好 p 个走 302，
	// 均匀散布而非整段聚集，高峰时本体始终承接 1-max_divert_ratio 的流量
	n := m.tick.Add(1)
	if (n*int64(p))%1000 >= int64(p) {
		return false
	}

	var chosen *redirectSource
	for _, s := range m.sources {
		if !s.disabled() {
			chosen = s
			break
		}
	}
	if chosen == nil {
		return false
	}

	chosen.hit()
	c.Header("Cache-Control", "no-cache, no-store, private")
	c.Redirect(http.StatusFound, "https://"+chosen.domain+c.Request.URL.String())
	return true
}

// qpsTracker 滑动窗口 QPS 统计器
type qpsTracker struct {
	mu     sync.Mutex
	window time.Duration
	times  []time.Time
}

func newQPSTracker(window time.Duration) *qpsTracker {
	return &qpsTracker{window: window}
}

// record 记录一次请求，返回当前滑动窗口内的 QPS
func (t *qpsTracker) record() int {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := now.Add(-t.window)
	keep := t.times[:0]
	for _, ts := range t.times {
		if ts.After(cutoff) {
			keep = append(keep, ts)
		}
	}
	t.times = append(keep, now)
	return len(t.times)
}

// TwimgProxyV2 镜像分流版 twimg 反代：
//   - 仅 .mp4 请求走 video.twimg.com 并参与分流；无扩展名 / .jpg / query 含 =jpg 的
//     请求一律直接代理 pbs.twimg.com，绝不分流
//   - 空闲（QPS<=阈值）时 .mp4 全部由本体反代 video.twimg.com；
//     QPS 超阈值后只把超过的部分按比例 302 分流到镜像节点
//     （配置文件 twimg_v2.json 启动时加载，改配置等每日重启生效），禁止缓存；
//     且分流比例不超 max_divert_ratio，即使 QPS 爆掉本体也始终自扛一部分；
//     每个节点独立计数，超过 limit 后禁用该节点，每日 UTC 0 点重置
//   - UTC 20:00~24:00 为分流域禁用窗口，只走自建反代
//   - 非 CN 请求一律 302 到官方源直连，不缓存
//   - StreamProxy 基于 request context，客户端断开会级联取消上游请求，避免流量空跑
func TwimgProxyV2(addr string) error {
	if addr == "" {
		addr = "127.26.8.10:8080"
		// return fmt.Errorf("[Twimg v2] addr is empty")
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	cfgPath := os.Getenv("TWIMG_V2_CONFIG")
	if cfgPath == "" {
		cfgPath = defaultConfigPath
	}
	m := newDivertManager(cfgPath)
	go m.resetLoop()

	qps := newQPSTracker(time.Second)

	headerProcesser := func(h http.Header) http.Header {
		h.Set("Referer", "https://x.com")
		return h
	}
	videoProxy := StreamVideoProxy("https://video.twimg.com", headerProcesser)
	imageProxy := StreamProxy("https://pbs.twimg.com", headerProcesser)

	v2Handler := func(c *gin.Context) {
		// 所有请求都参与 QPS 统计
		currentQPS := qps.record()

		// 只有扩展名为 .mp4 的请求才属于视频，允许走 video.twimg.com 和参与分流；
		// 无扩展名、.jpg、query 里出现 =jpg（如 format=jpg）等一律 pbs.twimg.com 直接代理。
		// URL.Path 不含 query，因此 /abc.mp4?tag=8 之类带参数的 .mp4 也能正确命中
		isVideo := strings.HasSuffix(strings.ToLower(c.Request.URL.Path), ".mp4")
		host := "pbs.twimg.com"
		if isVideo {
			host = "video.twimg.com"
		}

		// 非 CN 请求直接 302 到官方源，由客户端直连，不缓存
		if country := c.GetHeader("Cf-Ipcountry"); country != "" && country != "CN" {
			c.Header("Cache-Control", "no-cache, no-store, private")
			c.Redirect(http.StatusFound, "https://"+host+c.Request.URL.String())
			return
		}

		// 空闲时本体代理优先；QPS 超过阈值后，只有超过的部分走 302 分流。
		// 仅 .mp4 参与分流，图片请求绝不分流，始终直接代理 pbs.twimg.com
		if isVideo && m.shouldDivert(c, currentQPS) {
			return
		}

		if isVideo {
			videoProxy(c)
		} else {
			imageProxy(c)
		}
	}
	r.GET("/*any", v2Handler)
	r.HEAD("/*any", v2Handler)

	return r.Run(addr)
}

func PximgProxy(addr string) error {
	if addr == "" {
		return fmt.Errorf("addr is empty")
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	headerProcesser := func(h http.Header) http.Header {
		h.Set("referer", "https://pixiv.net/")
		return h
	}

	r.GET("/*any", StreamProxy("http://i.pximg.net", headerProcesser))

	return r.Run(addr)
}
