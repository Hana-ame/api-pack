package handler

import (
	"io"
	"net/http"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

var client = &http.Client{}

func Proxy(target string, header func(requestHeader http.Header) http.Header) gin.HandlerFunc {
	// 创建目标 URL
	// target, err := url.Parse(targetUrl)
	// if err != nil {
	// 	panic(err)
	// }

	return func(c *gin.Context) {

		// 发送请求到目标服务器
		resp, err := myfetch.Fetch(http.MethodGet, target+c.Request.URL.String(), header(c.Request.Header), nil)
		if tools.AbortWithError(c, 500, err) {
			return
		}
		defer resp.Body.Close()

		// 设置响应头和状态码
		for key, value := range resp.Header {
			c.Header(key, value[0])
		}
		c.Status(resp.StatusCode)

		// 流式传输响应数据
		_, err = io.Copy(c.Writer, resp.Body)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error copying response body")
		}
	}
}
