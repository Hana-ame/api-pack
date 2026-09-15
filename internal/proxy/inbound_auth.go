package proxy

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// InboundAuthMiddleware 前置入站 auth 检查（单人自用门禁）。
//
// expected 为从 env 读到的正确入站 token：客户端必须在 Authorization 头携带它
// （"Bearer <token>"，容错裸 <token> 与 scheme 大小写），否则 401，请求不碰上游。
// OPTIONS 预检放行（RunProxyRouter 里 CORSMiddleware 通常已先行 204 终结，双保险）。
//
// 注意：expected 为空绝不要装这个中间件（由 RunProxyRouter 判空门控），
// 且 bearerTokenEquals 对空 expected 恒 false——空配置永不放通任何请求。
func InboundAuthMiddleware(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		if bearerTokenEquals(c.GetHeader("Authorization"), expected) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"message": "invalid or missing inbound token",
				"type":    "authentication_error",
				"code":    "invalid_api_key",
			},
		})
	}
}

// bearerTokenEquals 从 Authorization 头提取 token 值并与 expected 常量时间比较。
// 兼容 "Bearer <t>"（scheme 大小写不敏感）与裸 "<t>"（curl 手搓）。
func bearerTokenEquals(auth, expected string) bool {
	if expected == "" {
		return false
	}
	v := strings.TrimSpace(auth)
	if len(v) >= 7 && strings.EqualFold(v[:7], "bearer ") {
		v = strings.TrimSpace(v[7:])
	}
	if v == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(v), []byte(expected)) == 1
}
