package functions

import (
	"api-pack/Tools/myfetch"

	"github.com/gin-gonic/gin"
)

// 没测试
func ListenAndServe(host string, path func(path string) string) func(c *gin.Context) {
	// return this function
	return func(c *gin.Context) {
		// path function
		if path == nil {
			path = func(path string) string {
				return path
			}
		}
		// generate url
		url := c.Request.URL
		url.Scheme = "https" // 给https访问但是被nginx反代的时候这里是啥。
		url.Host = host
		url.Path = path(c.Request.URL.Path)

		resp, err := myfetch.Fetch(c.Request.Method, url.String(), c.Request.Header, nil)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}

		defer resp.Body.Close()

		extraHeaders := make(map[string]string)
		for k, vs := range resp.Header {
			for i, v := range vs {
				if i > 0 {
					continue
				}
				extraHeaders[k] = v
			}
		}

		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, extraHeaders)
	}
}
