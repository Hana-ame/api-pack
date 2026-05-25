package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestProxyAll(t *testing.T) {
	r := gin.Default()

	// 代理所有路径
	r.Any("/*path", ProxyAll())

	r.Run(":8080")
}
