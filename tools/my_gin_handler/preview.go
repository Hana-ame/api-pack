package handler

import (
	"image/jpeg"
	"net/http"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
	"github.com/nfnt/resize"
)

func Preview(c *gin.Context) {
	url := c.Query("url")
	resp, err := myfetch.Fetch(http.MethodGet, url, nil, nil)
	if err != nil {
		c.String(http.StatusBadGateway, err.Error())
		return
	}

	img, err := tools.DecodeResponseToImage(resp)
	if err != nil {
		c.String(http.StatusBadGateway, err.Error())
		return
	}

	// 4. 生成缩略图（保持宽高比）
	thumbnail := resize.Thumbnail(320, 320, img, resize.Lanczos3)

	// 输出JPEG格式
	c.Writer.Header().Set("Content-Type", "image/jpeg")
	err = jpeg.Encode(c.Writer, thumbnail, &jpeg.Options{Quality: 80})
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
}
