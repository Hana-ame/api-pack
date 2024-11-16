package main

import (
	"net/http"

	tools "github.com/Hana-ame/api-pack/Tools"
	myfetch "github.com/Hana-ame/api-pack/Tools/my_fetch"
	middleware "github.com/Hana-ame/api-pack/Tools/my_gin_middleware"
	"github.com/gin-gonic/gin"
)

func main() {
	// 创建 Gin 引擎
	r := gin.Default()

	// 设置 CORS 头
	r.Use(middleware.CORSMiddleware())

	// 定义一个简单的 GET 路由
	r.Any("/*any", func(c *gin.Context) {
		if c.Request.Body != nil {
			defer c.Request.Body.Close()
		}

		path := c.Request.URL.String()
		host := c.GetHeader("X-Host")
		if host == "" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		// origin := c.GetHeader("X-Origin")
		// referer := c.GetHeader("X-Referer")
		// cookie := c.GetHeader("X-Cookie")

		// o := orderedmap.New()
		// o.Set("path", path)
		// o.Set("host", host)
		// o.Set("origin", origin)
		// o.Set("referer", referer)
		// o.Set("cookie", cookie)
		header := tools.NewHeader(nil)
		header.Add("Accept", c.GetHeader("Accept"))
		header.Add("Authorization", c.GetHeader("Authorization"))
		header.Add("Content-Type", c.GetHeader("X-Content-Type"))
		header.Add("Content-Length", c.GetHeader("X-Content-Length"))
		header.Add("Host", c.GetHeader("X-Host"))
		header.Add("Origin", c.GetHeader("X-Origin"))
		header.Add("Referer", c.GetHeader("X-Referer"))
		header.Add("Cookie", c.GetHeader("X-Cookie"))

		resp, err := myfetch.Fetch(c.Request.Method, "https://"+host+path,
			(header.Header), c.Request.Body)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		defer resp.Body.Close()

		// 为什么自带的方法这么贵物
		for k, vs := range resp.Header {
			for _, v := range vs {
				c.Writer.Header().Add(k, v)
			}
		}
		c.DataFromReader(http.StatusOK, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
	})

	// 启动服务器
	r.Run("127.24.11.16:8080") // 在 8080 端口启动服务
}
