package functions

import (
	"api-pack/Tools/myfetch"
	"image"
	"image/jpeg"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mat/besticon/v3/ico"
	"github.com/nfnt/resize"
)

func Icon(c *gin.Context) {
	host := c.Param("host")
	url := "https://" + host + "/favicon.ico"
	icon, err := getIcon(url)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, err)
	}

	newImage := resize.Resize(32, 32, icon, resize.Lanczos2)
	header := c.Writer.Header()
	header.Set("Content-Type", "image/jpeg")
	err = jpeg.Encode(c.Writer, newImage, nil)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
	}
}

func getIcon(url string) (image.Image, error) {

	resp, err := myfetch.Fetch(http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// file, err := os.Create("favicon.ico")
	// if err != nil {
	// 	return nil, err
	// }
	// defer file.Close()

	// _, err = io.Copy(file, resp.Body)
	// if err != nil {
	// 	return nil, err
	// }

	return ico.Decode(resp.Body)
}
