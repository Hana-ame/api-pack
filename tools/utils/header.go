package tools

import (
	"net/http"
	"strings"

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

// 过滤 Hop-by-hop 头
func IsHopByHop(header string) bool {
	hopHeaders := []string{
		"Connection",
		"Proxy-Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
	}
	for _, h := range hopHeaders {
		if strings.EqualFold(header, h) {
			return true
		}
	}
	return false
}

// 过滤客户端可伪造的来源 IP 头，避免误导上游的限流/封禁判断
func IsClientIPHeader(header string) bool {
	switch strings.ToLower(header) {
	case "x-forwarded-for", "x-real-ip", "forwarded", "true-client-ip", "cf-connecting-ip", "x-client-ip":
		return true
	}
	return false
}

// 过滤客户端可伪造/无意义的转发痕迹头
func IsProxySpoofHeader(header string) bool {
	switch strings.ToLower(header) {
	case "via", "x-forwarded-proto", "x-forwarded-host", "x-forwarded-port":
		return true
	}
	return false
}

// 过滤源站专属的权限/SSL 控制头，这些头属于源站而非代理，透传会干扰客户端
func IsOriginServerHeader(header string) bool {
	switch strings.ToLower(header) {
	case "strict-transport-security",
		"expect-ct",
		"public-key-pins",
		"content-security-policy",
		"content-security-policy-report-only",
		"x-content-security-policy",
		"x-webkit-csp",
		"x-frame-options",
		"x-content-type-options",
		"x-xss-protection",
		"permissions-policy",
		"feature-policy",
		"referrer-policy",
		"cross-origin-embedder-policy",
		"cross-origin-opener-policy",
		"cross-origin-resource-policy",
		"origin-agent-cluster",
		"clear-site-data",
		"report-to",
		"reporting-endpoints",
		"nel",
		"alt-svc",
		"link",
		"timing-allow-origin":
		return true
	}
	return false
}
