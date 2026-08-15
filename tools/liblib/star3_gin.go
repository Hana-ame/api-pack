package liblib

import (
	"encoding/json"
	"net/http"

	"github.com/Hana-ame/api-pack/tools/orderedmap"
	"github.com/gin-gonic/gin"
)

func Img2Img(c *gin.Context) {
	request := orderedmap.New()
	err := json.NewDecoder(c.Request.Body).Decode(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	o, err := Image2Image(
		request.GetOrDefault("prompt", "1 girl,cat girl,masterpiece,best quality,finely detail,highres,8k,beautiful and aesthetic,no watermark").(string),
		request.GetOrDefault("image", "https://upload.moonchan.xyz/api/01LLWEUU57EY6SVEWVQBAL26RQYVPFEBTZ/20250310-200604.png").(string),
		request.GetOrDefault("width", 768/4).(int),
		request.GetOrDefault("height", 1024/4).(int),
		request.GetOrDefault("img_count", 1).(int),
		request.GetOrDefault("steps", 30).(int),
		// request.GetOrDefault("controlNet", orderedmap.New()).(*orderedmap.OrderedMap),
		nil,
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, o)
}
func Text2Img(c *gin.Context) {
	request := orderedmap.New()
	err := json.NewDecoder(c.Request.Body).Decode(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	o, err := Text2Image(
		request.GetOrDefault("prompt", "1 girl,cat girl,masterpiece,best quality,finely detail,highres,8k,beautiful and aesthetic,no watermark").(string),
		request.GetOrDefault("width", 768).(int),
		request.GetOrDefault("height", 1024).(int),
		request.GetOrDefault("img_count", 1).(int),
		request.GetOrDefault("steps", 30).(int),
		nil, // request.GetOrDefault("controlNet", nil).(*orderedmap.OrderedMap),
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, o)
}

func GetStatus(c *gin.Context) {
	request := orderedmap.New()
	err := json.NewDecoder(c.Request.Body).Decode(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	uuid := request.GetOrDefault("generateUuid", "").(string)
	if uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "generateUuid is required",
		})
		return
	}
	o, err := Status(uuid)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, o)
}
