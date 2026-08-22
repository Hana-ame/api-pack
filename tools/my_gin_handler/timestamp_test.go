package handler

import (
	"os"
	"testing"

	"github.com/Hana-ame/api-pack/tools/debug"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

// TestTimestamp 是手动运行的服务器示例（会一直阻塞），默认跳过；
// 需要手动验证时设置 RUN_SERVER_TESTS=1 再跑。
func TestTimestamp(t *testing.T) {
	if os.Getenv("RUN_SERVER_TESTS") == "" {
		t.Skip("手动服务器测试，设 RUN_SERVER_TESTS=1 启用")
	}

	// func main() {
	router := gin.Default()

	router.GET("/timestamp", GetTimestamp)
	router.GET("/timestamp/s", GetTimestampSeconds)
	router.GET("/timestamp/ms", GetTimestampMilliseconds)
	router.GET("/timestamp/us", GetTimestampMicroseconds)
	router.GET("/timestamp/ns", GetTimestampNanoseconds)

	router.Run()
	// }

}

func TestNewTimeStamp(t *testing.T) {
	var la int64
	for i := 0; i < 200000; i++ {
		a := tools.NewTimeStamp()
		// fmt.Println(a)
		if la == a {
			debug.F("equal")
		}
		la = a
	}
}
