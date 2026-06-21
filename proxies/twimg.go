package proxies

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	handler "github.com/Hana-ame/api-pack/tools/my_gin_handler"
	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	"github.com/gin-gonic/gin"
)

func TwimgProxy(addr string) error {
	if addr == "" {
		return fmt.Errorf("addr is empty")
	}
	r := gin.Default()
	// r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	headerProcesser := func(h http.Header) http.Header { return h }

	twimgProxy := handler.Proxy("https://pbs.twimg.com", headerProcesser)
	videoProxy := handler.Proxy("https://video.twimg.com", headerProcesser)

	r.GET("/*any", func(c *gin.Context) {
		path := c.Request.URL.Path
		country := c.GetHeader("Cf-Ipcountry")
		log.Printf("Cf-Ipcountry=%s Cf-Connecting-Ip=%s\n", country, c.GetHeader("Cf-Connecting-Ip"))
		var host string
		var isVideo bool
		if strings.HasPrefix(path, "/tweet_video/") || strings.HasPrefix(path, "/ext_tw_video/") || strings.HasPrefix(path, "/amplify_video/") {
			host = "video.twimg.com"
			isVideo = true
		} else {
			host = "pbs.twimg.com"
		}

		if country != "" && country != "CN" {
			// 重要：禁止缓存重定向响应
			c.Header("Cache-Control", "no-cache, no-store, private")
			c.Redirect(http.StatusFound, "https://"+host+c.Request.URL.String())
			return
		}

		if isVideo {
			videoProxy(c)
		} else {
			twimgProxy(c)
		}

	})

	return r.Run(addr)

}

func PximgProxy(addr string) error {
	if addr == "" {
		return fmt.Errorf("addr is empty")
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	headerProcesser := func(h http.Header) http.Header { h.Set("referer", "https://pixiv.net/"); return h }

	r.GET("/*any", handler.Proxy("http://i.pximg.net", headerProcesser))

	return r.Run(addr)

}
