package proxies

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

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
	videoProxy := StreamProxy("https://video.twimg.com", headerProcesser)

	r.GET("/*any", func(c *gin.Context) {
		path := c.Request.URL.Path
		country := c.GetHeader("Cf-Ipcountry")
		// fmt.Printf("Cf-Ipcountry=%s Cf-Connecting-Ip=%s\n", country, c.GetHeader("Cf-Connecting-Ip"))
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
	})

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
