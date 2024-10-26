package google

import (
	"bytes"
	"net/http"

	mycurl "github.com/Hana-ame/api-pack/Tools/my_curl"
	"github.com/gin-gonic/gin"
)

const HOST = "https://www.google.com.hk"
const COOKIE_FILE = "cookie.txt"

var router *gin.Engine

func Router() *gin.Engine {
	return router
}

func init() {

	router = gin.Default()

	router.Any("/*path", func(c *gin.Context) {
		method := c.Request.Method
		path := c.Request.URL.String()
		headers := c.Request.Header
		body, err := c.GetRawData()
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		requestHeaders := make(mycurl.Headers, 0).LoadFromHttpHeader(headers)
		status, responseHeaders, resp, err := mycurl.Curl(method, c.Request.UserAgent(), requestHeaders, COOKIE_FILE, HOST+path, (body),
			"-L")
		// "--interface", "2001:470:c:6c::5")
		if err != nil {
			c.AbortWithError(http.StatusBadGateway, err)
			return
		}
		// c.Status(status)
		responseHeaders.DumpToHttpHeader(c.Writer.Header())
		c.DataFromReader(status, int64(len(resp)), responseHeaders.Get("Content-Type"), bytes.NewReader(resp), nil)
	})
}
