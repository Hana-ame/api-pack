package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/Hana-ame/api-pack/utils/randomreader"
	tools "github.com/Hana-ame/api-pack/utils/utils"
	"github.com/gin-gonic/gin"
)

func CheckID(c *gin.Context) {
	auth, err := c.Cookie("auth")
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	parts := strings.SplitN(auth, ".", 2)
	if len(parts) != 2 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	id, hash := parts[0], parts[1]
	if subtle.ConstantTimeCompare([]byte(tools.Hash(id, os.Getenv("SALT"))), []byte(hash)) != 1 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	c.Set("id", id)
}

func SetCookie(c *gin.Context) {
	id := make([]byte, 8)
	if c.Query("id") != "" {
		id = []byte(c.Query("id"))
	} else {
		if _, err := randomreader.Read(id); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	hash := tools.Hash(string(id), os.Getenv("SALT"))
	auth := string(id) + "." + hash
	c.SetSameSite(http.SameSiteNoneMode)
	c.SetCookie("auth", auth, 3600*24*365*10, "/", "", true, true)
}
