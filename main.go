// 26.01.11
// 适配myfetchv2
package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"github.com/Hana-ame/api-pack/chatto_reg"
	"github.com/Hana-ame/api-pack/exhentai"
	"github.com/Hana-ame/api-pack/exhentai_modify"
	"github.com/Hana-ame/api-pack/exhentai_stream"
	"github.com/Hana-ame/api-pack/proxies"
	"github.com/Hana-ame/api-pack/qwen"
	shijima "github.com/Hana-ame/api-pack/shijima"
	"github.com/Hana-ame/api-pack/tools/debug"
	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

func localTCPAddrFromEnv() *net.TCPAddr {
	if ipStr := os.Getenv("LOCAL_IP"); ipStr != "" {
		if ip := net.ParseIP(ipStr); ip != nil {
			return &net.TCPAddr{IP: ip}
		}
	}
	// 返回 nil 默认让系统选择（解决 127.0.0.2 无法访问公网的问题）
	return nil
}

func main() {

	// 我不行了。
	http.DefaultClient = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				LocalAddr: localTCPAddrFromEnv(),
				Timeout:   15 * time.Second,
				KeepAlive: 90 * time.Second,
			}).DialContext,
			MaxIdleConns:        256,
			IdleConnTimeout:     10 * time.Second,
			TLSHandshakeTimeout: 30 * time.Second,
		},
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 保持原样，将 301/302 转发给客户端
		},
	}

	debug.LogLevel = debug.Trace

	if os.Getenv("NYAA_PROXY") != "" {
		go proxies.NyaaProxy() //127.25.23.4:8080
	}
	if os.Getenv("SUKEBEI_PROXY") != "" {
		go proxies.SukebeiProxy() //127.25.23.5:8080
	}

	if tools.HasEnv("GROQ_PROXY") {
		go proxies.RunProxyRouter(os.Getenv("GROQ_PROXY"), proxies.ProxyConfig{
			Name:          "groq",
			Endpoint:      "https://api.groq.com",
			APIKey:        os.Getenv("GROQ_API_KEY"),
			FreeModelsAll: true,
		})
	}
	if tools.HasEnv("SILICONFLOW_PROXY") {
		go proxies.RunProxyRouter(os.Getenv("SILICONFLOW_PROXY"), proxies.ProxyConfig{
			Name:     "siliconflow",
			Endpoint: "https://api.siliconflow.cn",
			APIKey:   os.Getenv("SILICONFLOW_API_KEY"),
			FreeModels: map[string]bool{
				"Qwen/Qwen3.5-4B":                         true,
				"PaddlePaddle/PaddleOCR-VL-1.5":           true,
				"deepseek-ai/DeepSeek-R1-Distill-Qwen-7B": true,
				"THUDM/GLM-4.1V-9B-Thinking":              true,
				"PaddlePaddle/PaddleOCR-VL":               true,
				"deepseek-ai/DeepSeek-OCR":                true,
				"Qwen/Qwen3-8B":                           true,
				"tencent/Hunyuan-MT-7B":                   true,
				"deepseek-ai/DeepSeek-R1-0528-Qwen3-8B":   true,
				"THUDM/GLM-Z1-9B-0414":                    true,
				"Qwen/Qwen2.5-7B-Instruct":                true,
				"THUDM/GLM-4-9B-0414":                     true,
				"internlm/internlm2_5-7b-chat":            true,
			},
		})
	}
	if tools.HasEnv("GEMINI_PROXY") {
		go proxies.RunProxyRouter(os.Getenv("GEMINI_PROXY"), proxies.ProxyConfig{
			Name:          "gemini",
			Endpoint:      "https://generativelanguage.googleapis.com",
			FreeModelsAll: true,
		})
	}

	if tools.HasEnv("SHIJIMA") {
		go shijima.Run(os.Getenv("SHIJIMA"))
	}

	if tools.HasEnv("CHATTO_REG") {
		go chatto_reg.Run(os.Getenv("CHATTO_REG")) // 127.26.8.2:8080
	}

	// go EhProxy() //127.25.23.6:8080
	// go pastejson.Run(os.Getenv("PASTEJSON"), os.Getenv("PASTEJSON_CONN_STR")) // 127.25.9.10:8080

	go proxies.TwimgProxy(os.Getenv("TWIMG")) // 127.25.9.15:8080
	go proxies.PximgProxy(os.Getenv("PXIMG")) // 127.25.9.16:8080

	go proxies.EchoJSON() // 127.25.23.101:8080

	go qwen.Run(os.Getenv("QWEN_PROXY")) // 127.25.12.16:8080

	go exhentai.Run(os.Getenv("EX_PROXY"))
	go exhentai_stream.Run(os.Getenv("EX_STREAM"))
	go exhentai_modify.Run(os.Getenv("EX_MODIFY")) // env: EXHENTAI_ENDPOINT

	// proxy.moonchan.xyz
	//127.24.11.16:8080
	// 创建 Gin 引擎
	r := gin.Default()

	// 设置 CORS 头
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	// 2. 路由处理函数
	r.Any("/*any", func(c *gin.Context) {
		path := c.Request.URL.Path
		host := tools.Or(c.Query("proxy_host"), c.GetHeader("X-Host"))

		// --- 参数校验 ---
		if host == "" {
			if path == "/favicon.ico" {
				c.Redirect(http.StatusFound, "https://moonchan.xyz/favicon.ico")
			} else {
				c.Redirect(http.StatusFound, "https://moonchan.xyz/")
			}
			return
		}

		// 构造新的 URL
		scheme := tools.Or(c.Query("proxy_scheme"), c.GetHeader("X-Scheme"), "https")
		clientScheme := tools.ClientScheme(c)
		search := c.Request.URL.Query()
		// 删除代理专用参数，避免传给后端
		search.Del("proxy_host")
		search.Del("proxy_origin")
		search.Del("proxy_referer")
		search.Del("proxy_cookie")
		search.Del("proxy_scheme")

		targetURL := fmt.Sprintf("%s://%s%s", scheme, host, path)
		if len(search) > 0 {
			targetURL += "?" + search.Encode()
		}

		// --- 构造请求 ---
		// 必须使用 http.NewRequest 来手动控制
		req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// --- 处理请求头 (Request Headers) ---
		for k, vv := range c.Request.Header {
			// 跳过逐段传输头 (Hop-by-hop headers)、客户端可伪造的 IP/转发痕迹头、
			// 以及浏览器偏好头 Upgrade-Insecure-Requests（会诱导源站 301 跳 https）
			if tools.IsHopByHop(k) || tools.IsClientIPHeader(k) || tools.IsProxySpoofHeader(k) || strings.EqualFold(k, "Upgrade-Insecure-Requests") {
				continue
			}
			for _, v := range vv {
				req.Header.Add(k, v)
			}
		}

		// 关键：设置正确的 Host
		// 在 Go 中，req.Header.Set("Host", ...) 会被忽略，必须直接设置 req.Host
		req.Host = host

		// 覆盖特定的 Header
		req.Header.Set("Origin", tools.Or(c.Query("proxy_origin"), c.GetHeader("X-Origin")))
		req.Header.Set("Referer", tools.Or(c.Query("proxy_referer"), c.GetHeader("X-Referer")))
		if cookie := tools.Or(c.Query("proxy_cookie"), c.GetHeader("X-Cookie")); cookie != "" {
			req.Header.Set("Cookie", cookie)
		}

		// --- 执行请求 ---
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			c.String(http.StatusBadGateway, "Proxy Error: %v", err)
			return
		}
		defer resp.Body.Close()

		// --- 转发响应头 (Response Headers) ---
		// 只补尚未设置的头：CORS 中间件已设的 ACAO 优先，避免上游 CORS 头叠加成多值；
		// 源站专属的权限/SSL 控制头（HSTS/CSP/X-Frame-Options 等）不透传
		for k, vv := range resp.Header {
			if tools.IsHopByHop(k) || tools.IsOriginServerHeader(k) {
				continue
			}
			if c.Writer.Header().Get(k) != "" {
				continue
			}
			for _, v := range vv {
				c.Writer.Header().Add(k, v)
			}
		}

		// --- 重写 3xx 的 Location,让重定向留在代理内,避免裸 301 引发无限循环 ---
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			if loc := resp.Header.Get("Location"); loc != "" {
				carry := url.Values{}
				if v := tools.Or(c.Query("proxy_origin"), c.GetHeader("X-Origin")); v != "" {
					carry.Set("proxy_origin", v)
				}
				if v := tools.Or(c.Query("proxy_referer"), c.GetHeader("X-Referer")); v != "" {
					carry.Set("proxy_referer", v)
				}
				if v := tools.Or(c.Query("proxy_cookie"), c.GetHeader("X-Cookie")); v != "" {
					carry.Set("proxy_cookie", v)
				}
				c.Writer.Header().Set("Location", tools.RewriteLocation(loc, c.Request.Host, clientScheme, host, scheme, c.Request.URL.Path, search, carry))
			}
		}

		// 自定义 Header
		c.Writer.Header().Set("X-Proxy-Status", "success")
		c.Writer.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		c.Writer.Header().Set("Timing-Allow-Origin", "*")

		// --- 返回响应内容 ---
		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
	})

	r.Run(os.Getenv("PROXY"))
}
