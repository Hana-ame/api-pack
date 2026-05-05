package proxies

import (
	"fmt"
	"net/http"

	handler "github.com/Hana-ame/api-pack/tools/my_gin_handler"
	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	"github.com/gin-gonic/gin"
)

func TwimgProxy(addr string) error {
	if addr == "" {
		return fmt.Errorf("addr is empty")
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	headerProcesser := func(h http.Header) http.Header { return h }

	r.GET("/*any", handler.Proxy("https://pbs.twimg.com", headerProcesser))

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