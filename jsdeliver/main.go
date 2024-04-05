package jsdeliver

import (
	"api-pack/functions"

	"github.com/gin-gonic/gin"
)

var router *gin.Engine

func Router() *gin.Engine {
	return router
}

func init() {
	router = gin.Default()
	router.GET("/*whatever", functions.Proxy("www.jsdelivr.com", nil, nil))
}
