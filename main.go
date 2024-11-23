package main

import (
	"net/http"

	tools "github.com/Hana-ame/api-pack/Tools"
	myfetch "github.com/Hana-ame/api-pack/Tools/my_fetch"
	middleware "github.com/Hana-ame/api-pack/Tools/my_gin_middleware"
	"github.com/gin-gonic/gin"
)

func main() {

	go ExhProxy()
	go SProxy()
	go NyaaProxy()
	go SukebeiProxy()

	// 创建 Gin 引擎
	r := gin.Default()

	// 设置 CORS 头
	r.Use(middleware.CORSMiddleware())

	// 定义一个简单的 GET 路由
	r.Any("/*any", func(c *gin.Context) {
		c.Header("X-Debug-Request-Host", c.Request.Host)     // 要设置 Host $http_host
		c.Header("X-Debug-Header-Host", c.GetHeader("Host")) // never

		if c.Request.Body != nil {
			defer c.Request.Body.Close()
		}

		// 读取 URL 参数
		path := c.Request.URL.String()

		host := tools.NewSlice(
			c.Query("proxy_host"),
			c.GetHeader("X-Host"),
		).FirstNonDefaultValue("")
		if host == "" {
			c.Header("X-Error", "host not found")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		if c.Request.Host == host {
			c.Header("X-Error", c.Request.Host)
			c.AbortWithStatus(http.StatusLoopDetected)
		}

		header := tools.NewHeader(nil)

		header.Add("Host", host)
		header.Add("Origin", c.Query("proxy_origin"))
		header.Add("Origin", c.GetHeader("X-Origin"))
		header.Add("Referer", c.Query("proxy_referer"))
		header.Add("Referer", c.GetHeader("X-Referer"))
		header.Set("Cookie", tools.NewSlice(
			c.Query("proxy_cookie"),
			c.GetHeader("X-Cookie"),
			header.Get("Cookie"),
		).FirstNonDefaultValue(""))
		// header.Add("Cookie", c.GetHeader("X-Cookie")) // 这个是candidates传的

		resp, err := myfetch.Fetch(c.Request.Method, "https://"+host+path,
			(header.Header), c.Request.Body)
		if err != nil {
			c.Header("X-Error", err.Error())
			c.AbortWithError(http.StatusBadRequest, err)
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
		// slices.Sort(exposeHeaders)
		// c.Writer.Header().Add("Access-Control-Expose-Headers", strings.Join(exposeHeaders, ", "))

		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
			"X-Host":    host,
			"X-Origin":  header.Get("Origin"),
			"X-Referer": header.Get("Referer"),
			"X-Cookie":  header.Get("Cookie"),
		})
	})

	// 启动服务器
	r.Run("127.24.11.16:8080") // 在 8080 端口启动服务
}
