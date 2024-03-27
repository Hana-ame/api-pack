package functions

import (
	"log"

	"github.com/gin-gonic/gin"
)

func FileHandler(filepath func() string) func(c *gin.Context) {
	return func(c *gin.Context) {
		ip := c.GetHeader("CF-Connecting-IP")
		log.Println(ip)
		c.File(filepath())
	}
}
