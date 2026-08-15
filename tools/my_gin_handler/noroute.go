package handler

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// NoRoute 实现 SPA 路由逻辑
// rootDir: 静态文件根目录，例如 "/var/www/moonchan"
// fallbackFile: 找不到文件时的兜底文件，例如 "index.html"
func NoRoute(rootDir, fallbackFile string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取请求的文件路径 (例如 /assets/logo.png 或 /user/dashboard)
		fsPath := c.Request.URL.Path

		// 2. 拼接出服务器上的绝对路径
		// filepath.Clean 和 Join 会自动处理 "../" 等路径穿越攻击
		fullPath := filepath.Join(rootDir, filepath.Clean(fsPath))

		// 3. 检查文件是否存在
		// os.Stat 会去磁盘看一眼
		info, err := os.Stat(fullPath)

		// 4. 判断逻辑
		// 如果没有错误(文件存在) 且 不是目录(防止用户请求 /var/www/moonchan/ 导致暴露目录结构)
		if err == nil && !info.IsDir() {
			// 找到了具体的静态文件，直接发送给浏览器
			c.File(fullPath)
			return
		}

		// 5. 没找到文件 (说明这是一个前端路由，或者是真正的 404)
		// 按照要求，返回 fallback 文件 (index.html)
		// 这样前端 JS 加载后会根据 URL 渲染正确的页面
		fallbackPath := filepath.Join(rootDir, fallbackFile)

		// 这一步是否需要设置状态码取决于你的需求
		// 通常 SPA 返回 index.html 时应该是 200 OK，否则浏览器可能会标红
		c.Status(http.StatusOK)
		c.File(fallbackPath)
	}
}
