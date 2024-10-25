package files

import (
	"io"
	"net/http"
	"os"

	db "github.com/Hana-ame/api-pack/Tools/db_filehash"

	"path/filepath"

	"github.com/gin-gonic/gin"
)

func LocalFile(c *gin.Context) {
	sha1sum := c.Param("sha1sum")
	path, err := db.ReadPathByHash(sha1sum)
	if err != nil {
		c.AbortWithError(http.StatusNotFound, err)
		return
	}

	file, err := os.Open(path)
	if err != nil {
		c.AbortWithError(http.StatusNotFound, err)
		return
	}
	defer file.Close()

	filename := filepath.Base(path)
	c.Header("Content-Disposition", ("attachment; filename=" + filename))
	c.Header("Content-Type", "application/octet-stream")

	_, err = io.Copy(c.Writer, file)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
}
