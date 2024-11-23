package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestXxx111(t *testing.T) {

	// 创建 Gin 引擎
	r := gin.Default()

	r.Any("/*any", func(ctx *gin.Context) {
		fmt.Println(1)
		ctx.Next()
		ctx.Header("2", "2")
		fmt.Println(1)
	}, func(ctx *gin.Context) {
		fmt.Println(2)
		ctx.Next()
		ctx.DataFromReader(200, 0, "plain/text", bytes.NewReader([]byte{}), map[string]string{"1": "1"})
		fmt.Println(2)
	})

	r.Run("127.24.11.23:8080")
}

func TestExhProxy(t *testing.T) {
	ExhProxy()
}
