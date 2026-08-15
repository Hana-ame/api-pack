package middleware

import (
	"net/http"

	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

// 使用 strings.Join 将切片连接成一个字符串
var headers = ([]string{
	"Accept",
	"Authorization",

	"X-Host",
	"X-Origin",
	"X-Referer",

	"X-Requested-With",

	"Cache-Control",
})

// 输出结果

// CORS 中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置 CORS 头
		c.Header("Access-Control-Allow-Origin", tools.Or(c.Request.Header.Get("Origin"), "*")) // 或者指定特定的源
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH, HEAD")
		// c.Header("Access-Control-Allow-Headers", strings.Join(headers, ", "))
		c.Header("Access-Control-Allow-Headers", c.GetHeader("access-control-request-headers"))
		c.Header("Access-Control-Allow-Credentials", "true") // 允许cookie传递

		c.Header("Access-Control-Expose-Headers", "*") // 放这里可以吗？

		// 处理预检请求
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent) // 返回 204 No Content
			return
		}

		// 将HEAD请求转换为GET请求
		// if c.Request.Method == http.MethodHead {
		// 	c.Request.Method = http.MethodGet
		// }

		c.Next() // 继续处理请求

		// 为什么自带的方法这么贵物
		// exposeHeaders := make([]string, 0, len(c.Writer.Header()))
		// for k, _ := range c.Writer.Header() {
		// 	exposeHeaders = append(exposeHeaders, k)
		// }
		// slices.Sort(exposeHeaders)
		// c.Writer.Header().Add("Access-Control-Expose-Headers", strings.Join(exposeHeaders, ", "))
		// c.Writer.Header().Add("Access-Control-Expose-Headers", "*") // 试试通配符

		// 必须override掉
		c.Header("cross-origin-resource-policy", "cross-origin")

	}
}
