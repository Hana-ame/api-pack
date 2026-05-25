package tools

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 防止添加""作为header的外包装
type Header struct {
	http.Header
}

// 不会添加空字符串
func (h Header) Add(key, value string) {
	if value == "" {
		return
	}
	h.Header.Add(key, value)
}

// 空字符串 = 删除
func (h Header) Set(key, value string) {
	if value == "" {
		h.Header.Del(key)
	} else {
		h.Header.Set(key, value)
	}
}

func (h Header) GetAllKeys() []string {
	s := make([]string, 0, len(h.Header))
	for k := range h.Header {
		s = append(s, k)
	}
	return s
}

func (h Header) ToMap() map[string]string {
	m := make(map[string]string, len(h.Header))
	for k := range h.Header {
		m[k] = h.Get(k)
	}
	return m
}

// 仅为了防止“”作为header被添加
func NewHeader(header http.Header) Header {
	if header == nil {
		header = http.Header{}
	}
	return Header{Header: header}
}

// 只影响尚未设置的.
func PatchHeader(c *gin.Context, header http.Header) {
	for k, vs := range header {
		if c.Writer.Header().Get(k) != "" {
			continue
		}
		for _, v := range vs {
			c.Writer.Header().Add(k, v)
		}
	}
}
