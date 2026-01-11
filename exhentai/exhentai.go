package exhentai

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch/v2"
	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/antchfx/htmlquery"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var proxyClient = &http.Client{
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			LocalAddr: &net.TCPAddr{IP: net.IPv4(142, 171, 157, 74)},
			Timeout:   3 * time.Second,
			KeepAlive: 3 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     10 * time.Second,
		TLSHandshakeTimeout: 3 * time.Second,
	},
	Timeout: 10 * time.Second,
}

// 配置常量
// var (
// 	OldDomain = []string{"nmbyd1.top"}
// 	NewDomain = []string{"ex.nmbyd2.top"}
// )

const (
	// 	ConfigURL = "https://config.810114.xyz/exhentai/settings"

	TargetHost = "exhentai.org"

	StaticHost = "page.moonchan.xyz"

// MigrationDate = "2025-04-10T00:00:00Z"

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

		special.GET("/fullimg/*any", p.handleOrigin)
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
		"/../",
	}

	// 预定义非法后缀
	forbiddenSuffixes := []string{
		".cgi", ".asp", ".aspx", ".jsp", ".jspx",
		".sh", ".py", ".pl", ".sql", ".bak", ".log", ".swp",
	}

	// 3. 预定义精确匹配的非法路径 (添加了日志中出现的 DoH 相关路径)
	forbiddenExact := []string{
		"/fullimg", "/mytags", "/uconfig.php",
		"/login",
		"/dns-query", "/query", "/resolve", // 屏蔽常见的 DoH 路径
		"/.well-known",
		"/sw.js", "/manifest.json", // 不是在这里用的
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

	file, err := os.Open("./exhentai/settings.html")
	if tools.AbortWithError(c, http.StatusInternalServerError, err) {
		return
	}
	fileInfo, err := file.Stat()
	if tools.AbortWithError(c, http.StatusInternalServerError, err) {
		return
	}

	// Create the map with your custom 1-week cache setting
	headers := map[string]string{
		"Cache-Control": "public, max-age=604800",
		"X-From":        ".",
	}

	// DataFromReader handles the Content-Type and Content-Length via arguments,
	// and the rest via the map.
	c.DataFromReader(
		http.StatusOK,
		fileInfo.Size(),
		"text/html; charset=utf-8",
		file,
		headers,
	)

	c.Abort()
	return
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
	// 1. 定义资源存放的物理根目录
	const assetsRoot = "./exhentai"

	path := c.Request.URL.Path
	host := TargetHost

	// 静态资源路由映射
	if strings.HasPrefix(path, "/exhentai/") {

		// 2. 获取去掉前缀后的子路径
		// 例如 path 是 "/exhentai/images/1.jpg" -> subPath 是 "images/1.jpg"
		subPath := strings.TrimPrefix(c.Request.URL.Path, "/exhentai/")

		// 3. 拼接物理路径并 Clean
		finalPath := filepath.Join(assetsRoot, subPath)

		// 4. 安全检查：确保计算后的路径确实在 assetsRoot 内部
		rel, err := filepath.Rel(assetsRoot, finalPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "非法路径访问"})
			return
		}

		// 5. 检查是否是目录（防止打开目录流）
		fi, err := os.Stat(finalPath)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		if fi.IsDir() {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		// 6. 使用 Gin 内置方法（它会自动处理 Range, ETag, Content-Type 等）
		c.File(finalPath)

		return
	}

	if c.Query("host") != "" || strings.HasPrefix(path, "/static/") || path == "/sw.js" || path == "/manifest.json" || path == "/logo192.png" {
		resp, err := proxyClient.Get("https://" + StaticHost + c.Request.URL.String())
		if tools.AbortWithError(c, http.StatusBadGateway, err) {
			return
		}
		defer resp.Body.Close()
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
		tools.AbortWithError(c, http.StatusServiceUnavailable, err)
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

	// 5. 响应客户端 (不压缩，由 Cloudflare 负责压缩)
	copyHeaders(c, resp.Header)

	// 【重要】确保删除 Content-Encoding
	// 如果 copyHeaders 从上游把 content-encoding: gzip 拷过来了，必须删掉，
	// 否则浏览器会以为数据是压缩的，结果收到明文会报错。
	c.Writer.Header().Del("Content-Encoding")

	// 【建议】设置 Content-Length
	// 之前用 Gzip 是因为不知道压缩后多大，所以置空强制 Chunked。
	// 现在直接发原数据，长度是已知的，设置 Content-Length 对 HTTP 传输效率更高。
	c.Header("Content-Length", strconv.Itoa(len(finalData)))

	// 再次确保 Content-Type 是 html (防止 copyHeaders 没拷过来)
	// 如果 copyHeaders 已经有了可以省略这一行
	if c.Writer.Header().Get("Content-Type") == "" {
		c.Header("Content-Type", "text/html; charset=utf-8")
	}

	c.Status(resp.StatusCode)

	// 【核心修改】直接写入原始数据，不再套用 gzip writer
	c.Writer.Write(finalData)
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
	const scriptTag = `<script src="/exhentai/ex.js" defer></script>`
	// const cssTag = `<link rel="stylesheet" type="text/css" href="/z/0381/x.css">`
	const headStartTag = "<head>"

	// 查找 </head> 标签的位置
	if !bytes.Contains(data, []byte(headStartTag)) {
		// 如果没有找到 </head>，则直接返回处理过 URL 的 html
		return data
	}
	data = bytes.Replace(data, []byte("https://exhentai.org"), []byte{}, -1)
	data = bytes.Replace(data, []byte("https://s.exhentai.org"), []byte("https://ehgt.org"), -1)
	data = bytes.Replace(data, []byte("https://ehgt.org/api.php"), []byte("/api.php"), 1)

	// 执行替换：将 </head> 替换为 <script ...></script></head>
	// return bytes.Replace(data, []byte(headStartTag), []byte(headStartTag+metaNoReferer+scriptTag+cssTag), 1)
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

func (p *ProxyHandler) handleOrigin(c *gin.Context) {
	c.Redirect(302, "https://exhentai.org"+c.Request.URL.String())
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
