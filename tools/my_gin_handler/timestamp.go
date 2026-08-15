package handler

import (
	"net/http"
	"strconv"

	timestamp "github.com/Hana-ame/api-pack/tools/utils" // 替换为你的项目路径

	"github.com/gin-gonic/gin"
)

func GetTimestamp(c *gin.Context) {
	ts := timestamp.GetTimestamp()
	c.String(http.StatusOK, strconv.Itoa(int(ts)))
}

func GetTimestampSeconds(c *gin.Context) {
	ts := timestamp.GetTimestampSeconds()
	c.String(http.StatusOK, strconv.Itoa(int(ts)))
}

func GetTimestampMilliseconds(c *gin.Context) {
	ts := timestamp.GetTimestampMilliseconds()
	c.String(http.StatusOK, strconv.Itoa(int(ts)))
}

func GetTimestampMicroseconds(c *gin.Context) {
	ts := timestamp.GetTimestampMicroseconds()
	c.String(http.StatusOK, strconv.Itoa(int(ts)))
}

func GetTimestampNanoseconds(c *gin.Context) {
	ts := timestamp.GetTimestampNanoseconds()
	c.String(http.StatusOK, strconv.Itoa(int(ts)))
}

func NewTimeStamp(c *gin.Context) {
	ts := timestamp.NewTimeStamp()
	c.String(http.StatusOK, strconv.FormatInt(ts, 10))
}
