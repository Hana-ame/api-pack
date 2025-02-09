package main

import (
	"net/http"
	"os"

	tools "github.com/Hana-ame/api-pack/Tools"
	myfetch "github.com/Hana-ame/api-pack/Tools/my_fetch"
	middleware "github.com/Hana-ame/api-pack/Tools/my_gin_middleware"
	"github.com/gin-gonic/gin"
)

func GroqProxy() {

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

		// 添加需要的APIKEY
		headers := tools.NewHeader(c.Request.Header)
		headers.Add("Authorization", "Bearer "+os.Getenv("GROQ_API_KEY"))
		headers.Add("Content-Type", "application/json")

		resp, err := myfetch.Fetch(
			c.Request.Method, "https://api.groq.com/openai/v1/chat/completions",
			(headers.Header), c.Request.Body)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		defer resp.Body.Close()

		// 必须有，不然会乱码
		for k, vs := range resp.Header {
			// if c.Writer.Header().Get(k) != "" {
			// 	continue
			// }
			for _, v := range vs {
				c.Writer.Header().Add(k, v)
			}
		}

		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{})

	})

	// 启动服务器
	r.Run("127.25.2.9:8080") // 在 8080 端口启动服务

}
