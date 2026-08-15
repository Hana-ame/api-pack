package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	curl "github.com/Hana-ame/api-pack/tools/my_curl"
)

// 目的未达成
func CurlCode(c *gin.Context) {
	host := c.Query("host")
	// u, err := url.Parse(host)
	code, _, _, err := curl.Curl(c.Request.Method, "myfetch/250818", nil, "", host, nil, "-x", "")

	c.JSON(http.StatusOK, gin.H{
		"code": code,
		"err":  err.Error(),
	})

}
