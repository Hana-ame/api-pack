package proxies

import (
	"net/http"
	"strconv"
	"time"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch"
	handler "github.com/Hana-ame/api-pack/tools/my_gin_handler"
	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	"github.com/gin-gonic/gin"
)

func Echo() {

	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Any("/*any", handler.Echo)
	r.Run("127.25.23.100:8080")
}

func EchoJSON() {

	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Any("/*any", handler.EchoJSON)
	r.Run("127.25.23.101:8080")
}

func GetIP() {

	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.GET("/", handler.EchoCFIP)

	r.Any("/doh", func(c *gin.Context) {
		requestURL := c.Request.URL
		requestURL.Scheme = "https"
		requestURL.Host = "dns.cloudflare.com"
		requestURL.Path = "/dns-query"

		resp, err := myfetch.Fetch(c.Request.Method, requestURL.String(), c.Request.Header, c.Request.Body)
		if err != nil {
			c.Header("X-Error", err.Error())
			c.AbortWithError(http.StatusBadGateway, err)
			return
		}

		defer resp.Body.Close()

		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
	})

	r.GET("/timestamp", func(c *gin.Context) {
		ts := float64(time.Now().UnixNano()) * (float64(65536) / float64(1_000_000))
		c.String(http.StatusOK, strconv.Itoa(int(ts)))
	})
	r.GET("/timestamp/s", func(c *gin.Context) {
		ts := (time.Now().UnixNano()) / 1e9
		c.String(http.StatusOK, strconv.Itoa(int(ts)))
	})
	r.GET("/timestamp/ms", func(c *gin.Context) {
		ts := (time.Now().UnixNano()) / 1e6
		c.String(http.StatusOK, strconv.Itoa(int(ts)))
	})
	r.GET("/timestamp/us", func(c *gin.Context) {
		ts := (time.Now().UnixNano()) / 1e3
		c.String(http.StatusOK, strconv.Itoa(int(ts)))
	})
	r.GET("/timestamp/ns", func(c *gin.Context) {
		ts := (time.Now().UnixNano())
		c.String(http.StatusOK, strconv.Itoa(int(ts)))
	})

	r.Run("127.25.23.102:8080")
}
