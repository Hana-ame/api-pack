package handler

import (
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestProxyAll 是手动运行的服务器示例（会一直阻塞），默认跳过；
// 需要手动验证时设置 RUN_SERVER_TESTS=1 再跑。
func TestProxyAll(t *testing.T) {
	if os.Getenv("RUN_SERVER_TESTS") == "" {
		t.Skip("手动服务器测试，设 RUN_SERVER_TESTS=1 启用")
	}
	r := gin.Default()

	// 代理所有路径
	r.Any("/*path", ProxyAll())

	r.Run(":8080")
}
