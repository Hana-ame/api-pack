package handler

import (
	"github.com/gin-gonic/gin"
)

func RedirectTo(code int, s string) func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Redirect(code, s)
	}
}
