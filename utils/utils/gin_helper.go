// 这东西真的能用吗
// 本来是为了合并postform方法和json方法的.

package tools

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
)

// NewExtractor 初始化并返回一个 extractor 实例
// 根据请求的 Content-Type 提取数据
func NewExtractor(c *gin.Context) (*Extractor, error) {
	extractor := &Extractor{
		cache: nil,
		c:     c,
	}

	if c.ContentType() == "application/json" {
		extractor.cache = make(map[string]string)
		decoder := json.NewDecoder(c.Request.Body)
		if err := decoder.Decode(&extractor.cache); err != nil {
			return extractor, fmt.Errorf("error encoding body while application/json %v", err)
		}
	}

	return extractor, nil
}

// Extractor 是一个从请求中提取数据的工具
type Extractor struct {
	cache map[string]string
	c     *gin.Context
}

// Get 从 extractor 中获取指定键的值
// 优先从缓存中获取，否则从 POST 表单中获取
func (e *Extractor) Get(key string) string {
	if e.cache == nil {
		return e.c.PostForm(key)
	} else {
		return e.cache[key]
	}
}

func AbortWithError(c *gin.Context, code int, err error) bool {
	if err != nil {
		c.Header("X-Error", err.Error())
		c.AbortWithStatusJSON(code, gin.H{
			"error": err.Error(),
		})
		// c.AbortWithStatusJSON(code, gin.H{"error": err.Error()
		return true
	}
	return false
}
