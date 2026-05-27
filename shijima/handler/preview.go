package handler

import (
	"image/jpeg"
	"net/http"
	"os"
	"slices"

	myfetch "github.com/Hana-ame/api-pack/utils/my_fetch"
	tools "github.com/Hana-ame/api-pack/utils/utils"
	"github.com/gin-gonic/gin"
	"github.com/nfnt/resize"
)

func (h *Handler) Preview(c *gin.Context) {
	path := c.Param("path")
	host := tools.Or(c.Query("proxy_host"), c.Query("host"))
	query := c.Request.URL.Query()
	query.Del("host")
	header := tools.NewHeader(c.Request.Header)
	header.Set("Referer", c.Query("proxy_referer"))

	url := "https://" + host + path + "?" + query.Encode()
	if host == "upload.moonchan.xyz" && tools.HasEnv("AZURE") {
		url = "http://" + os.Getenv("AZURE") + path + "?" + query.Encode()
	}

	resp, err := myfetch.Fetch(http.MethodGet, url, header.Header, nil)
	if err != nil {
		c.String(http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	if !slices.Contains(
		[]string{"image/jpeg", "image/jpg", "image/webp", "image/png", "image/gif", "image/avif"},
		resp.Header.Get("Content-Type")) {
		c.String(http.StatusBadRequest, "unsupported image format")
		return
	}

	img, err := tools.DecodeResponseToImage(resp)
	if err != nil {
		c.String(http.StatusBadGateway, err.Error())
		return
	}

	thumbnail := resize.Thumbnail(480, 480, img, resize.Lanczos3)
	c.Writer.Header().Set("Content-Type", "image/jpeg")
	jpeg.Encode(c.Writer, thumbnail, &jpeg.Options{Quality: 80})
}
