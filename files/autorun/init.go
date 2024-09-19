package autorun

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func init() {
	r := gin.Default()
	err := r.Run("127.24.9.18:8080")
	fmt.Println(err)
}
