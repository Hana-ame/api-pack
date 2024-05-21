package doh

import (
	"api-pack/Tools/myfetch"
	"api-pack/functions"
	"net/http"
	"path"
	"strconv"
	"time"

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

		resp, err := myfetch.Fetch(http.MethodGet, requestURL.String(), c.Request.Header, nil)
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

	// timestamps
	router.GET("/timestamp", func(c *gin.Context) {
		ts := float64(time.Now().UnixNano()) * (float64(65536) / float64(1_000_000))
		c.String(http.StatusOK, strconv.Itoa(int(ts)))
	})
	router.GET("/timestamp/s", func(c *gin.Context) {
		ts := (time.Now().UnixNano()) / 1e9
		c.String(http.StatusOK, strconv.Itoa(int(ts)))
	})
	router.GET("/timestamp/ms", func(c *gin.Context) {
		ts := (time.Now().UnixNano()) / 1e6
		c.String(http.StatusOK, strconv.Itoa(int(ts)))
	})
	router.GET("/timestamp/us", func(c *gin.Context) {
		ts := (time.Now().UnixNano()) / 1e3
		c.String(http.StatusOK, strconv.Itoa(int(ts)))
	})
	router.GET("/timestamp/ns", func(c *gin.Context) {
		ts := (time.Now().UnixNano())
		c.String(http.StatusOK, strconv.Itoa(int(ts)))
	})

	// spy pic
	router.GET("/1x1", functions.FileHandler(func() string {
		return path.Join("/root", "1x1.png")
	}))

	// spy pic
	router.GET("/echo", functions.Echo)
	router.GET("/icon/:host", functions.Icon)
	router.POST("/icon/:host", functions.CreateIcon)
}
