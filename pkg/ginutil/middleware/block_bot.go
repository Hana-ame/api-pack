package middleware

import (
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

// botRegex 命中主流搜索引擎爬虫的 User-Agent。
var botRegex = regexp.MustCompile(`(?i)(Googlebot|bingbot|Baiduspider|YandexBot)`)

// sensitiveBids 是不希望被搜索引擎收录的版块（bid）：命中后对爬虫返回空数据。
var sensitiveBids = map[string]bool{
	"46":  true,
	"48":  true,
	"103": true,
}

// BlockBotMiddleware 对敏感版块（?bid=46/48/103）的爬虫请求返回空数据，阻止抓取。
func BlockBotMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		bid := c.Query("bid")
		userAgent := c.GetHeader("User-Agent")

		if sensitiveBids[bid] && botRegex.MatchString(userAgent) {
			// 对爬虫返回空数据，阻止抓取
			c.AbortWithStatusJSON(http.StatusOK, gin.H{
				"code":    0,
				"data":    nil,
				"message": "Empty",
			})
			return
		}

		c.Next()
	}
}
