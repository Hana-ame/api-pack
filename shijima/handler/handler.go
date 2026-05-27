package handler

import (
	"net/http"
	"os"

	"github.com/Hana-ame/api-pack/shijima/repo"
	"github.com/Hana-ame/api-pack/utils/randomreader"
	tools "github.com/Hana-ame/api-pack/utils/utils"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	Repo         *repo.Repo
	newReactions *lruCache
}

func New(r *repo.Repo) *Handler {
	return &Handler{
		Repo:         r,
		newReactions: newLRUCache(50),
	}
}

type lruCache struct{ cache *tools.LRUCache[int, bool] }

func newLRUCache(cap int) *lruCache { return &lruCache{cache: tools.NewLRUCache[int, bool](cap)} }
func (c *lruCache) put(k int)       { c.cache.Put(k, true) }
func (c *lruCache) delete(k int)    { c.cache.Delete(k) }
func (c *lruCache) getOrder() []int { return c.cache.GetOrder() }

func CookieHandler(c *gin.Context) {
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
	c.SetCookie("auth", auth, 3600*24*365*10, "/", "", false, false)
	c.String(http.StatusOK, "ok")
}
