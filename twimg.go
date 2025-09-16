package main

import (
	"fmt"

	handler "github.com/Hana-ame/api-pack/Tools/my_gin_handler"
	middleware "github.com/Hana-ame/api-pack/Tools/my_gin_middleware"
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

	r.GET("/*any", handler.Proxy("https://pbs.twimg.com"))

	return r.Run(addr)

}
