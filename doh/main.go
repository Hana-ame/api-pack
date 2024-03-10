package doh

import (
	"api-pack/Tools/myfetch"
	"net/http"

	"github.com/gin-gonic/gin"
)

var router *gin.Engine

func Router() *gin.Engine {
	return router
}

func init() {
	router = gin.Default()

	router.GET("/", func(c *gin.Context) {
		ip := c.GetHeader("CF-Connecting-IP")
		c.String(http.StatusOK, ip)
	})

	router.GET("/doh", func(c *gin.Context) {
		// resp, err := http.Get("https://dns.cloudflare.com/dns-query")
		requestURL := c.Request.URL
		requestURL.Scheme = "https"
		requestURL.Host = "dns.cloudflare.com"
		requestURL.Path = "/dns-query"

		header := make(map[string]string)
		for k, v := range c.Request.Header {
			if len(v) < 1 {
				continue
			}
			header[k] = v[0]
		}

		resp, err := myfetch.Fetch(http.MethodGet, requestURL.String(), header, nil)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}

		defer resp.Body.Close()

		c.DataFromReader(http.StatusOK, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
	})
	router.POST("/doh", func(c *gin.Context) {
		// resp, err := http.Get("https://dns.cloudflare.com/dns-query")
		requestURL := c.Request.URL
		requestURL.Scheme = "https"
		requestURL.Host = "dns.cloudflare.com"
		requestURL.Path = "/dns-query"

		resp, err := http.Post("https://moonchan.xyz/doh", c.ContentType(), c.Request.Body)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		defer resp.Body.Close()

		c.DataFromReader(http.StatusOK, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
	})
}
