package exhentai

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch/v2"
	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/antchfx/htmlquery"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// 配置常量
var (
	OldDomain = []string{"nmbyd1.top"}
	NewDomain = []string{"ex.nmbyd2.top"}
)

const (
	ConfigURL     = "https://config.810114.xyz/exhentai/settings"
	TargetHost    = "exhentai.org"
	StaticHost    = "page.moonchan.xyz"
	MigrationDate = "2025-04-10T00:00:00Z"
)

type ProxyHandler struct {
	*IPRotator
}

func ExhProxy(rotator *IPRotator, addr string) {
	godotenv.Load(".env")

	// 1. 初始化网络与 Fetcher
	p := &ProxyHandler{rotator}

	// 2. 初始化 Gin
	r := gin.Default()

	// 全局中间件
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	// 3. 路由分发

	// B. 特殊路径处理 (API/Legacy)
	special := r.Group("/")
	{
		special.GET("/api/*any", func(c *gin.Context) { c.AbortWithStatus(http.StatusGone) })
		special.GET("/archiver.php", func(c *gin.Context) {
			c.Redirect(301, "https://exhentai.org"+c.Request.URL.String())
		})
		special.GET("/uconfig.php", p.handleUConfig)
		special.GET("/image/*any", p.handleImageLegacy)
	}

	// D. 核心代理逻辑 (包含屏蔽逻辑和内容注入)
	r.NoRoute(p.accessControlMiddleware(), p.mainProxyHandler)

	r.Run(addr)
}

// --- 中间件部分 ---

// 访问控制中间件
func (p *ProxyHandler) accessControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 1. 静态资源与特殊参数放行
		if c.Query("redirect_to") != "" || strings.HasPrefix(path, "/static/") {
			c.Next()
			return
		}

		// 2. 内部阅读器放行
		referer := c.Request.Referer()
		if strings.Contains(referer, "moonchan") || strings.Contains(referer, "nmbyd") {
			c.Next()
			return
		}

		// 3. 中文用户放行
		if strings.Contains(c.GetHeader("accept-language"), "zh") {
			c.Next()
			return
		}

		// 4. 功能级封禁
		if path == "/fullimg" || path == "/mytags" {
			c.AbortWithStatus(http.StatusForbidden)
			c.Abort()
			return
		}

		// 5. GeoIP 屏蔽 (非中国 IP 且无通行证)
		country := c.GetHeader("Cf-Ipcountry")
		if !slices.Contains([]string{"CN", ""}, country) {
			c.String(http.StatusForbidden, "请使用大陆IP. Current Region: "+country)
			c.Abort()
			return
		}

		// 6. PHP 攻击防护
		allowPHP := []string{"/gallerytorrents.php", "/favorites.php", "/torrents.php", "/gallerypopups.php"}
		if strings.HasSuffix(path, ".php") && !slices.Contains(allowPHP, path) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}

// --- 处理函数部分 ---

func (p *ProxyHandler) handleUConfig(c *gin.Context) {
	resp, err := defaultClient.Get(ConfigURL)
	if err != nil {
		tools.AbortWithError(c, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()
	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
	c.Abort()
}

func (p *ProxyHandler) handleImageLegacy(c *gin.Context) {
	path := strings.TrimPrefix(c.Request.URL.String(), "/image")
	u, _ := url.Parse(path)
	q := u.Query()
	q.Set("redirect_to", "image")
	u.RawQuery = q.Encode()
	c.Redirect(301, u.String())
	c.Abort()
}

func (p *ProxyHandler) mainProxyHandler(c *gin.Context) {
	path := c.Request.URL.Path
	host := TargetHost

	if c.Query("host") != "" {
		host = c.Query("host")
	}

	// 静态资源路由映射
	if strings.HasPrefix(path, "/static/") || path == "/sw.js" || path == "/manifest.json" {
		host = StaticHost
	}

	p.doProxy(c, "https://"+host+c.Request.URL.String(), false)
}

// 核心转发逻辑
func (p *ProxyHandler) doProxy(c *gin.Context, targetURL string, isEH bool) {
	reqHeader := p.prepareHeaders(c, isEH)

	resp, err := p.Fetch(c.Request.Method, targetURL, reqHeader, c.Request.Body)
	if err != nil {
		tools.AbortWithError(c, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()

	// 2. 特殊文件直接流式返回 (Torrent/JS)
	if strings.HasPrefix(targetURL, "/torrent") || strings.HasPrefix(targetURL, "/z/") {
		copyHeaders(c, resp.Header)
		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
		return
	}

	body, err := myfetch.ResponseToReader(resp)
	if err != nil {
		tools.AbortWithError(c, http.StatusBadGateway, err)
		return
	}

	// 3. 读取并修改 HTML 内容
	bodyData, err := io.ReadAll(body)
	if err != nil || len(bodyData) == 0 {
		c.Status(resp.StatusCode)
		return
	}

	// 4. 处理 "redirect_to" 逻辑 (解析 HTML)
	if p.handleSpecialRedirects(c, bodyData) {
		return
	}

	// 内容替换逻辑
	finalData := p.transformContent(c, bodyData, targetURL)

	// 5. 响应客户端 (Gzip 压缩)
	copyHeaders(c, resp.Header)
	c.Header("Content-Length", "") // 强制 Chunked
	c.Header("Content-Encoding", "gzip")
	c.Status(resp.StatusCode)

	gz := gzip.NewWriter(c.Writer)
	defer gz.Close()
	gz.Write(finalData)
}

// --- 辅助工具函数 ---

func (p *ProxyHandler) prepareHeaders(c *gin.Context, isEH bool) http.Header {
	h := c.Request.Header.Clone()

	// 设置 Cookie
	cookie := tools.NewSlice(c.GetHeader("X-Cookie"), os.Getenv("EXHENTAI_PROXY_COOKIE")).FirstUnequal("")
	h.Set("Cookie", cookie)

	// User-Agent 修正
	if strings.Contains(h.Get("User-Agent"), "Mobile") {
		h.Set("User-Agent", "myfetch/2025.4.14")
	}

	// Referer 修正
	refURL, err := url.Parse(c.Request.Referer())
	if err == nil && refURL.Host != "" {
		refURL.Scheme = "https"
		refURL.Host = TargetHost
		h.Set("Referer", refURL.String())
	} else {
		h.Set("Referer", "https://exhentai.org/")
	}

	return h
}

func (p *ProxyHandler) transformContent(c *gin.Context, data []byte, targetURL string) []byte {
	// 基础 URL 替换
	data = bytes.ReplaceAll(data, []byte("https://exhentai.org"), []byte(""))
	data = bytes.ReplaceAll(data, []byte("https://s.exhentai.org"), []byte("https://ehgt.org"))

	// 注入脚本和按钮
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/s/") {
		data = []byte(addWaterFallViewButton(string(data)))
	} else if !strings.HasPrefix(path, "/g/") {
		data = addReloadCoverButton(data)
	}

	data = addFloatingIframeAtRightBottom(data)
	if !strings.Contains(targetURL, StaticHost) {
		data = addInlineChatRoom(data)
	}

	return data
}

func (p *ProxyHandler) handleSpecialRedirects(c *gin.Context, data []byte) bool {
	redir := c.Query("redirect_to")
	if redir == "" {
		return false
	}

	doc, _ := htmlquery.Parse(bytes.NewReader(data))
	if doc == nil {
		return false
	}

	switch redir {
	case "image":
		if img, err := findOneAndSelectAttr(doc, "//img[@id='img']", "src"); err == nil {
			c.Redirect(http.StatusFound, img)
			return true
		}
	case "cover":
		hrefs := findAll(doc, "//a", "href")
		for _, h := range hrefs {
			if strings.HasPrefix(h, "/s/") {
				c.Redirect(http.StatusMovedPermanently, h+"?redirect_to=image")
				return true
			}
		}
	}
	return false
}

func copyHeaders(c *gin.Context, h http.Header) {
	for k, vs := range h {
		if k == "Content-Length" || k == "Content-Encoding" {
			continue
		}
		for _, v := range vs {
			c.Writer.Header().Add(k, v)
		}
	}
}
