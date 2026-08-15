package tools

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// 客户端访问本代理所用的 scheme:优先 X-Forwarded-Proto,否则按 TLS 推断
func ClientScheme(c *gin.Context) string {
	if s := c.GetHeader("X-Forwarded-Proto"); s != "" {
		return s
	}
	if c.Request.TLS != nil {
		return "https"
	}
	return "http"
}

// 把上游 3xx 的 Location 重写为经由本代理访问的 URL,让重定向留在代理内。
//   - proxyHost: 客户端访问本代理用的 host(如 c.Request.Host)
//   - clientScheme: 客户端访问本代理用的 scheme
//   - curHost/curScheme/curPath/curQuery: 当前转发给上游的 host/scheme/path/query,用于解析相对 Location
//   - carry: 需要保留带到重定向目标的代理参数(如 proxy_origin),拼成 query
//
// 协议相对(//host/...)按客户端访问代理的 scheme 解析;相对路径按 RFC 3986 相对当前 URL 解析。
func RewriteLocation(location, proxyHost, clientScheme, curHost, curScheme, curPath string, curQuery, carry url.Values) string {
	ref, err := url.Parse(strings.TrimSpace(location))
	if err != nil {
		return location
	}

	base := &url.URL{Scheme: curScheme, Host: curHost, Path: curPath, RawQuery: curQuery.Encode()}
	if ref.Scheme == "" && ref.Host != "" {
		base.Scheme = clientScheme
	}
	resolved := base.ResolveReference(ref)

	targetHost := resolved.Host
	if targetHost == "" {
		targetHost = curHost
	}
	targetScheme := resolved.Scheme
	if targetScheme == "" {
		targetScheme = clientScheme
	}

	q := url.Values{}
	for k, vs := range resolved.Query() {
		if strings.HasPrefix(strings.ToLower(k), "proxy_") {
			continue
		}
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	q.Set("proxy_host", targetHost)
	q.Set("proxy_scheme", targetScheme)
	for k, vs := range carry {
		for _, v := range vs {
			q.Set(k, v)
		}
	}

	out := &url.URL{
		Scheme:   clientScheme,
		Host:     proxyHost,
		Path:     resolved.Path,
		RawQuery: q.Encode(),
		Fragment: resolved.Fragment,
	}
	return out.String()
}
