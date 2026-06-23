package proxies

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// IPRateLimiter 定义基于内存的 IP 限流器
type IPRateLimiter struct {
	mu     sync.Mutex
	counts map[string]int
	limit  int
}

// NewIPRateLimiter 创建一个新的限流器
func NewIPRateLimiter(limit int) *IPRateLimiter {
	return &IPRateLimiter{
		counts: make(map[string]int),
		limit:  limit,
	}
}

// Middleware Gin 限流中间件
func (rl *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先获取 Cloudflare 传递的用户真实 IP
		ip := c.GetHeader("Cf-Connecting-Ip")
		if ip == "" {
			// 退化为获取直连 IP 或传统代理头 IP
			ip = c.ClientIP()
		}

		rl.mu.Lock()
		// 如果访问次数已达上限，拦截请求并返回 429
		if rl.counts[ip] >= rl.limit {
			rl.mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Maximum 1000 requests per day.",
			})
			return
		}
		// 次数加 1
		rl.counts[ip]++
		rl.mu.Unlock()

		// 继续处理请求
		c.Next()
	}
}
