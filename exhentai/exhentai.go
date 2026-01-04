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
	// 预定义一些拒绝访问的敏感路径或关键词
	forbiddenKeywords := []string{
		"/.git", "/.env", "/.svn", "/.vscode",
		"/phpmyadmin", "/wp-admin", "/wp-content",
		"/config", "/backup", "/etc/passwd",
		"/fullimg", "/mytags",
		"/actuator/",
	}

	// 预定义非法后缀
	forbiddenSuffixes := []string{
		".cgi", ".asp", ".aspx", ".jsp", ".jspx",
		".sh", ".py", ".pl", ".sql", ".bak", ".log", ".swp",
	}

	forbiddenExact := []string{
		"/fullimg", "/mytags", "/uconfig.php",
		"/login",
	}

	return func(c *gin.Context) {
		// 使用 Path 而不是 RequestURI，Path 不包含查询参数，更安全
		path := strings.ToLower(c.Request.URL.Path)

		// 1. 静态资源与特殊参数放行 (保持原样)
		if c.Query("redirect_to") != "" || strings.HasPrefix(path, "/static/") {
			c.Next()
			return
		}

		// 2. 路径遍历防护 (Basic Directory Traversal)
		if strings.Contains(path, "..") {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		// 3. 常见攻击路径/关键词屏蔽
		for _, keyword := range forbiddenKeywords {
			if strings.Contains(path, keyword) {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		// 4. 非法脚本后缀屏蔽 (.cgi, .asp 等)
		for _, suffix := range forbiddenSuffixes {
			if strings.HasSuffix(path, suffix) {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		// 5. 功能级封禁 (保持原样)
		if slices.Contains(forbiddenExact, path) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		// 6. PHP 攻击防护 (白名单模式)
		// 允许访问的 PHP 列表
		allowPHP := []string{"/gallerytorrents.php", "/favorites.php", "/torrents.php", "/gallerypopups.php"}
		allowedPHP := false
		if strings.HasSuffix(path, ".php") {
			for _, a := range allowPHP {
				if path == a {
					allowedPHP = true
					break
				}
			}
			if !allowedPHP {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		// 7. GeoIP 屏蔽 (保持原样)
		country := c.GetHeader("Cf-Ipcountry")
		acceptLang := strings.ToLower(c.GetHeader("accept-language"))
		if !strings.Contains(acceptLang, "zh") && !slices.Contains([]string{"CN", ""}, country) {
			c.String(http.StatusForbidden, "请使用大陆IP.\nPlease ensure you're in China Mainland\n中国・大陸以外のアクセスは制限されています\n Current Region: "+country)
			c.Abort()
			return
		}

		// 不是 allowed php，而且不是 get 的情况
		if !allowedPHP && c.Request.Method != http.MethodGet {
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

	// 静态资源路由映射
	if c.Query("host") != "" || strings.HasPrefix(path, "/static/") || path == "/sw.js" || path == "/manifest.json" {
		resp, err := http.Get("https://" + StaticHost + c.Request.URL.String())
		if tools.AbortWithError(c, http.StatusBadGateway, err) {
			return
		}
		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
			"X-From": "page.moonchan.xyz",
		})
		return
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
	if strings.HasPrefix(c.Request.URL.String(), "/s/") && strings.Contains(h.Get("User-Agent"), "Mobile") {
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
	// 注入外部 Script 标签
	const metaNoReferer = `<meta name="referrer" content="no-referrer">`
	const scriptTag = `<script src="https://config.810114.xyz/exhentai/ex.js" defer></script>`
	const headStartTag = "<head>"

	// 查找 </head> 标签的位置
	if !bytes.Contains(data, []byte(headStartTag)) {
		// 如果没有找到 </head>，则直接返回处理过 URL 的 html
		return data
	}

	// 执行替换：将 </head> 替换为 <script ...></script></head>
	return bytes.Replace(data, []byte(headStartTag), []byte(headStartTag+metaNoReferer+scriptTag), 1)
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
