package main

import (
	"net/http"
	"os"
	"strconv"

	_ "github.com/joho/godotenv/autoload"

	"github.com/Hana-ame/api-pack/pastejson"
	"github.com/Hana-ame/api-pack/qwen"
	shijima "github.com/Hana-ame/api-pack/shijima"
	"github.com/Hana-ame/api-pack/tools/debug"
	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch"
	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/Hana-ame/api-pack/tools/wasm/v"
	"github.com/gin-gonic/gin"
)

func main() {

	debug.LogLevel = debug.Trace

	if os.Getenv("SEX_PROXY") != "" {
		go SProxy() //127.25.23.3:8080
	}
	if os.Getenv("EX_PROXY") != "" {
		go ExhProxy() //EX_PROXY="127.25.23.2:8080"
	}
	if tools.HasEnv("EX_ORIGIN_PROXY") {
		go ExOrigin("127.25.23.2:8080")
	}
	if os.Getenv("NYAA_PROXY") != "" {
		go NyaaProxy() //127.25.23.4:8080
	}
	if os.Getenv("SUKEBEI_PROXY") != "" {
		go SukebeiProxy() //127.25.23.5:8080
	}
	if tools.HasEnv("GROQ_PROXY") {
		go OpenaiProxy(os.Getenv("GROQ_PROXY")) //127.25.2.9:8080
		go OpenaiProxyAlt("127.25.11.6:8080")   //127.25.2.9:8080
	}
	if tools.HasEnv("SHIJIMA") {
		go shijima.Run(os.Getenv("SHIJIMA"))
	}
	// go EhProxy()      //127.25.23.6:8080
	go pastejson.Run(os.Getenv("PASTEJSON"), os.Getenv("PASTEJSON_CONN_STR")) // 127.25.9.10:8080

	go TwimgProxy(os.Getenv("TWIMG")) // 127.25.9.15:8080
	go PximgProxy(os.Getenv("PXIMG")) // 127.25.9.16:8080

	go EchoJSON() // 127.25.23.101:8080

	go qwen.Run(os.Getenv("QWEN_PROXY")) // 127.25.12.16:8080

	//127.24.11.16:8080
	// 创建 Gin 引擎
	r := gin.Default()

	// 设置 CORS 头
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	// 定义一个简单的 GET 路由
	r.Any("/*any", func(c *gin.Context) {
		c.Header("X-Debug-Request-Host", c.Request.Host)     // 要设置 Host $http_host
		c.Header("X-Debug-Header-Host", c.GetHeader("Host")) // never

		if c.Request.Body != nil {
			defer c.Request.Body.Close()
		}

		v := v.V(c.Request.URL.Path)
		if strconv.Itoa(int(v)) != c.Query("v") {
			// 届时添加阻止内容
		}

		// 读取 URL 参数
		path := c.Request.URL.Path

		host := tools.Or(c.Query("proxy_host"), c.GetHeader("X-Host"))

		if host == "" {
			if path == "/favicon.ico" {
				c.Redirect(http.StatusFound, "https://moonchan.xyz/favicon.ico")
				return
			} else {
				c.Header("X-Error", "host not found")
				c.Redirect(http.StatusFound, "https://page.moonchan.xyz/")
				return
			}
		} else if c.Request.Host == host {
			c.Header("X-Error", c.Request.Host)
			c.Redirect(http.StatusFound, "https://page.moonchan.xyz/")
			return
		}

		header := tools.NewHeader(c.Request.Header)

		header.Set("Host", host)
		header.Set("Origin", tools.Or(c.Query("proxy_origin"), c.GetHeader("X-Origin"), c.GetHeader("Origin")))
		header.Set("Referer", tools.Or(c.Query("proxy_referer"), c.GetHeader("X-Referer") /*c.GetHeader("Referer")*/))
		header.Set("Cookie", tools.Or(c.Query("proxy_cookie"), c.GetHeader("X-Cookie"), header.Get("Cookie")))

		scheme := tools.Or(c.Query("proxy_scheme"), c.GetHeader("X-Scheme"), "https")

		search := c.Request.URL.Query()
		search.Del("proxy_host")
		search.Del("proxy_origin")
		search.Del("proxy_referer")
		search.Del("proxy_cookie")

		newUrl := scheme + "://" + host + path + tools.Ternary(len(search) > 0, "?", "") + search.Encode()

		resp, err := myfetch.Fetch(c.Request.Method, newUrl,
			(header.Header), c.Request.Body)
		if tools.AbortWithError(c, 500, err) {
			return
		}
		defer resp.Body.Close()

		// 为什么自带的方法这么贵物
		// exposeHeaders := make([]string, 0, len(resp.Header)) // move to middle ware
		for k, vs := range resp.Header {
			// exposeHeaders = append(exposeHeaders, k)
			if c.Writer.Header().Get(k) != "" {
				continue
			}
			for _, v := range vs {
				c.Writer.Header().Add(k, v)
			}
		}
		c.Writer.Header().Set("cross-origin-resource-policy", "cross-origin")

		// slices.Sort(exposeHeaders)
		// c.Writer.Header().Add("Access-Control-Expose-Headers", strings.Join(exposeHeaders, ", "))

		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
			"X-Host":    host,
			"X-Origin":  header.Get("Origin"),
			"X-Referer": header.Get("Referer"),
			"X-Cookie":  header.Get("Cookie"),
			"V":         strconv.Itoa(int(v)),
		})
	})

	// 启动服务器
	r.Run("127.24.11.16:8080") // 在 8080 端口启动服务
}
