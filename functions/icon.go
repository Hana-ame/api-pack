package functions

import (
	"api-pack/Tools/myfetch"
	"image"
	"image/png"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mat/besticon/v3/ico"
	"github.com/nfnt/resize"
	"gorm.io/gorm"
)

type myKV struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"not null"`
}

// init a db
var iconDB = func() *MyDBInterface {
	db := NewDBInterface("ginpack/icons.db", &gorm.Config{})
	db.AutoMigrate(new(myKV))
	return db
}()

func Icon(c *gin.Context) {
	host := c.Param("host")

	url := getIconUrl(host)

	icon, err := getIcon(url)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, err)
	}

	newImage := resize.Resize(16, 16, icon, resize.Lanczos2)
	header := c.Writer.Header()
	header.Set("Content-Type", "image/png")
	err = png.Encode(c.Writer, newImage)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
	}
}

func CreateIcon(c *gin.Context) {
	host := c.Param("host")

	// Read the request body as a byte slice
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		// Handle the error
		c.String(http.StatusInternalServerError, "Failed to read request body")
		return
	}

	// Convert the byte slice to a string
	iconUrl := string(body)

	if err := updateIconUrl(host, iconUrl); err != nil {
		c.String(http.StatusInternalServerError, "Failed to save kv")
	}

	c.Status(200)
}

func updateIconUrl(host, iconUrl string) error {
	tx := iconDB.Begin()

	if err := Update(tx, &myKV{host, iconUrl}); err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()

	return nil
}

func getIconUrl(host string) string {
	url := "https://" + host + "/favicon.ico"

	mykv := &myKV{Key: host}

	tx := iconDB.Begin()

	err := Read(tx, mykv)

	tx.Commit()

	if err != nil || mykv.Value == "" {
		return url
	}

	return mykv.Value
}

func createIconUrl(host, iconUrl string) error {
	tx := iconDB.Begin()

	if err := Update(tx, &myKV{host, iconUrl}); err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()

	return nil
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
