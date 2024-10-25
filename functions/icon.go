package functions

import (
	"fmt"
	tools "github.com/Hana-ame/api-pack/Tools"
	"github.com/Hana-ame/api-pack/Tools/myfetch"
	"image"
	"image/png"
	"net/http"
	"net/url"

	"github.com/antchfx/htmlquery"
	"github.com/buckket/go-blurhash"
	"github.com/gin-gonic/gin"
)

type myKV struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"not null"`
}

// init a db
var iconDB *MyDBInterface

func Icon(c *gin.Context) {
	host := c.Param("host")

	imageBlurhash, err := getIconUrl(host)
	if err != nil {
		image, err := getIcon(host)
		if err != nil {
			c.AbortWithError(http.StatusNotFound, err)
			return
		}
		imageBlurhash, err = blurhash.Encode(16, 16, image)
		if err != nil {
			c.AbortWithError(http.StatusNotFound, err)
			return
		}
		updateIconUrl(host, imageBlurhash)
	}

	image, err := blurhash.Decode(imageBlurhash, 16, 16, 1)
	if err != nil {
		c.AbortWithError(http.StatusNotFound, err)
		return
	}
	header := c.Writer.Header()
	header.Set("Content-Type", "image/png")
	err = png.Encode(c.Writer, image)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
	}
}

// func CreateIcon(c *gin.Context) {
// 	host := c.Param("host")

// 	// Read the request body as a byte slice
// 	body, err := io.ReadAll(c.Request.Body)
// 	if err != nil {
// 		// Handle the error
// 		c.String(http.StatusInternalServerError, "Failed to read request body")
// 		return
// 	}

// 	// Convert the byte slice to a string
// 	iconUrl := string(body)

// 	if err := updateIconUrl(host, iconBlurHash); err != nil {
// 		c.String(http.StatusInternalServerError, "Failed to save kv")
// 	}

// 	c.Status(200)
// }

func updateIconUrl(host, iconUrl string) error {
	tx := iconDB.Begin()

	if err := Update(tx, &myKV{host, iconUrl}); err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()

	return nil
}

func getIconUrl(host string) (string, error) {
	mykv := &myKV{Key: host}

	tx := iconDB.Begin()

	err := Read(tx, mykv)

	tx.Commit()

	// if err != nil || mykv.Value == "" {
	// 	return url
	// }

	return mykv.Value, err
}

// func createIconUrl(host, iconUrl string) error {
// 	tx := iconDB.Begin()

// 	if err := Update(tx, &myKV{host, iconUrl}); err != nil {
// 		tx.Rollback()
// 		return err
// 	}

// 	tx.Commit()

// 	return nil
// }

func getIcon(host string) (image.Image, error) {
	u := url.URL{
		Scheme: "https",
		Host:   host,
		Path:   "/favicon.ico",
	}

	resp, err := myfetch.Fetch(http.MethodGet, u.String(), nil, nil)
	if err == nil {
		defer resp.Body.Close()
		return tools.DecodeResponseToImage(resp)
	}

	u.Path = "/"
	resp, err = myfetch.Fetch(http.MethodGet, u.String(), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 解析 HTML
	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	// 使用 XPath 查找 <link rel="icon"> 标签
	links := htmlquery.Find(doc, "//link[@rel='icon']")
	for _, link := range links {
		href := htmlquery.SelectAttr(link, "href")
		// mimeType := htmlquery.SelectAttr(link, "type")

		resp, err := myfetch.Fetch(http.MethodGet, href, nil, nil)
		if err == nil {
			defer resp.Body.Close()
			return tools.DecodeResponseToImage(resp)
		}

		// return nil, err
	}

	return nil, fmt.Errorf("not found")
}

// file, err := os.Create("favicon.ico")
// if err != nil {
// 	return nil, err
// }
// defer file.Close()

// _, err = io.Copy(file, resp.Body)
// if err != nil {
// 	return nil, err
// }

// return ico.Decode(resp.Body)
// }
