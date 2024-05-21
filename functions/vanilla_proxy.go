package functions

import (
	"api-pack/Tools/myfetch"
	"net/http"

	"github.com/gin-gonic/gin"
)

func isSkip(key string) bool {
	return key == "access-control-allow-origin" ||
		key == "access-control-allow-credentials" ||
		key == "content-security-policy" ||
		key == "content-security-policy-report-only" ||
		key == "clear-site-data"
}

func ProxyPath(path string) string {
	return path
}

// 不能用，path行了，mime不行。
func Proxy(host string, path func(path string) string, extraHeaders http.Header) func(c *gin.Context) {
	if path == nil {
		path = ProxyPath
	}
	// return this function
	return func(c *gin.Context) {

		// generate url
		url := c.Request.URL
		url.Scheme = "https" // 给https访问但是被nginx反代的时候这里是啥。
		url.Host = host
		// log.Println(c.Request.URL.String()) // 不是为啥啊。
		// log.Println(c.Request.URL.Path)
		url.Path = path(c.Request.URL.Path)

		for k, vs := range extraHeaders {
			c.Request.Header[k] = vs
		}

		resp, err := myfetch.Fetch(c.Request.Method, url.String(), c.Request.Header, nil)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		defer resp.Body.Close()

		extraHeaders := make(map[string]string)
		for k, vs := range resp.Header {
			if isSkip(k) {
				continue
			}
			for i, v := range vs {
				if i > 0 {
					break
				}
				extraHeaders[k] = v
			}
		}
		extraHeaders["access-control-allow-origin"] = "*"

		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, extraHeaders)
	}
}
