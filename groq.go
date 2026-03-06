package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch/v2"
	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

var groqAPIKey = os.Getenv("GROQ_API_KEY")
var groqEndpoint = "https://api.groq.com/openai/v1/chat/completions"

// 通用转发处理器
func groqProxyHandler(c *gin.Context) {
	// 读取原始请求体
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}
	c.Request.Body.Close()

	headers := tools.NewHeader(c.Request.Header)
	headers.Set("Authorization", "Bearer "+groqAPIKey)
	headers.Set("Content-Type", "application/json")

	// 使用 myfetch 发送请求（也可直接使用 http.DefaultClient）
	resp, err := myfetch.Fetch(
		c.Request.Method,
		groqEndpoint,
		headers.Header,
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// 复制响应头并返回响应体
	tools.PatchHeader(c, resp.Header)
	c.DataFromReader(
		resp.StatusCode,
		resp.ContentLength,
		resp.Header.Get("Content-Type"),
		resp.Body,
		map[string]string{"X-Service": "groq"},
	)
}

func Groq(addr string) {
	if addr == "" {
		return
	}
	if groqAPIKey == "" {
		log.Println("GROQ_API_KEY environment variable not set")
	}

	r := gin.Default()

	// 可选 CORS 中间件
	r.Use(middleware.CORSMiddleware())

	// 转发路由
	r.POST("/*any", groqProxyHandler)
	// r.POST("/v1/chat/completions", groqProxyHandler)
	// r.POST("/chat/completions", groqProxyHandler)

	// 健康检查
	// r.GET("/health", func(c *gin.Context) {
	// 	c.JSON(200, gin.H{"status": "ok"})
	// })

	r.Run(addr)
}
