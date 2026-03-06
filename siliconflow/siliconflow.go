package siliconflow

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"

	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

var (
	siliconflowAPIKey   = os.Getenv("SILICONFLOW_API_KEY")
	siliconflowEndpoint = "https://api.siliconflow.cn/v1/chat/completions"
	freeModels          = map[string]bool{
		"deepseek-ai/DeepSeek-OCR":              true,
		"THUDM/GLM-4-9B-0414":                   true,
		"THUDM/GLM-Z1-9B-0414":                  true,
		"THUDM/glm-4-9b-chat":                   true,
		"deepseek-ai/DeepSeek-R1-0528-Qwen3-8B": true,
		"Qwen/Qwen3-8B":                         true,
	}

	client = &http.Client{}
)

// 通用转发处理器：解析并检查 model，通过后使用原始 JSON 转发
func siliconflowProxyHandler(c *gin.Context) {
	// 1. 读取原始请求体
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}
	c.Request.Body.Close()

	// 2. 解析为 orderedmap 用于检查 model
	reqMap, err := tools.ReaderToJSON(bytes.NewReader(bodyBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	modelVal := reqMap.GetOrDefault("model", nil)
	if modelVal == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model field is required"})
		return
	}
	model, ok := modelVal.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model must be a string"})
		return
	}

	// 3. 检查模型是否免费
	if !freeModels[model] {
		c.JSON(http.StatusForbidden, gin.H{"error": "model not allowed"})
		return
	}

	// 4. 使用原始 bodyBytes 创建转发请求（不重新 marshal）
	req, err := http.NewRequestWithContext(
		c.Request.Context(),
		c.Request.Method,
		siliconflowEndpoint,
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create proxy request"})
		return
	}

	// 5. 设置必要的请求头
	req.Header.Set("Authorization", "Bearer "+siliconflowAPIKey)
	req.Header.Set("Content-Type", "application/json")
	for key, values := range c.Request.Header {
		if key != "Authorization" { // 避免覆盖
			for _, v := range values {
				req.Header.Add(key, v)
			}
		}
	}

	// 6. 发送请求并返回响应
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, v := range values {
			c.Writer.Header().Add(key, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

func Run(addr string) {

	if addr == "" {
		return
	}

	if siliconflowAPIKey == "" {
		log.Println("SILICONFLOW_API_KEY environment variable not set")
		return
	}

	r := gin.Default()

	// CORS 中间件
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 通用转发（包含模型检查）

	// 转发路由
	r.POST("/*any", siliconflowProxyHandler)
	// r.POST("/v1/chat/completions", siliconflowProxyHandler)
	// r.POST("/chat/completions", siliconflowProxyHandler)

	// 健康检查（可选）
	// r.GET("/health", func(c *gin.Context) {
	// 	c.JSON(200, gin.H{"status": "ok"})
	// })

	r.Run(addr)
}
