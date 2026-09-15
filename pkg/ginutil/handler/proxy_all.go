package handler

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// 不知道什么时候写的,感觉不能用
func ProxyAll() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头中获取 Referer
		referer := c.GetHeader("Referer")
		if referer == "" {
			c.String(http.StatusBadRequest, "Referer header is required")
			return
		}

		// 解析 Referer URL 以提取主机部分
		refererUrl, err := url.Parse(referer)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid Referer URL")
			return
		}

		// 获取目标主机
		targetHost := refererUrl.Host

		// 构建目标 URL
		target := &url.URL{
			Scheme:   refererUrl.Scheme,
			Host:     targetHost,
			Path:     c.Request.URL.Path,     // 使用 Gin 接收到的请求路径
			RawQuery: c.Request.URL.RawQuery, // 保留查询参数
		}

		// 创建请求
		proxy := c.Request.Clone(c.Request.Context())
		proxy.URL = target
		proxy.Host = targetHost

		// 发送请求到目标服务器
		client := &http.Client{}
		resp, err := client.Do(proxy)
		if err != nil {
			c.String(http.StatusBadGateway, "Bad Gateway")
			return
		}
		defer resp.Body.Close()

		// 设置响应头和状态码
		for key, value := range resp.Header {
			c.Header(key, strings.Join(value, ","))
		}
		c.Status(resp.StatusCode)

		// 流式传输响应数据
		_, err = io.Copy(c.Writer, resp.Body)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error copying response body")
		}
	}
}
