package autorun

import (
	"api-pack/files"

	"github.com/gin-gonic/gin"
)

func init() {
	r := gin.Default()
	r.GET("/ws/server", files.ServerHandler)
	r.GET("/ws", files.ClientWsHandler)
	r.GET("/api/:sha1sum/:filename", files.ClientRESTHandler)
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	go r.Run("127.24.9.18:8080")
}
