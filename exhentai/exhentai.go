package exhentai

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
			LocalAddr: &net.TCPAddr{IP: func() net.IP {
				if ipStr := os.Getenv("LOCAL_IP"); ipStr != "" {
					if ip := net.ParseIP(ipStr); ip != nil {
						return ip
					}
				}
				return nil
			}()},
			Timeout:   3 * time.Second,
			KeepAlive: 3 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     10 * time.Second,
		TLSHandshakeTimeout: 3 * time.Second,
	},
	Timeout: 5 * time.Second,
}

// 预编译正则，提高性能
var (
	reUploader       = regexp.MustCompile(`^/uploader/.*`)
	reTag            = regexp.MustCompile(`^/tag/.*`)
	reZ              = regexp.MustCompile(`^/z/.*`)
	reImg            = regexp.MustCompile(`^/img/.*`)
	reTorrent        = regexp.MustCompile(`^/torrent/.*`)
	reImage          = regexp.MustCompile(`^/s/[0-9a-f]+/\d+-\d+`)
	reGallery        = regexp.MustCompile(`^/g/\d+/[0-9a-f]+(/.*)?`)
	reCover          = regexp.MustCompile(`https://s\.exhentai\.org/([^"'\s>]+)`)
	coverReplacement = []byte("https://proxy.moonchan.xyz/$1?proxy_host=ehgt.org")
	reTorrents       = regexp.MustCompile(`u\.value=\d+`)
)

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
			if c.GetHeader("X-Cookie") != "" {
				p.mainProxyHandler(c)
			} else {
				c.Redirect(301, "https://exhentai.org"+c.Request.URL.String())
			}
			return
		})
		special.GET("/uconfig.php", p.handleUConfig)
		special.POST("/api.php", func(c *gin.Context) { c.AbortWithStatus(http.StatusForbidden) })
		special.GET("/image/*any", p.handleImageLegacy)

		special.GET("/fullimg/*any", func(c *gin.Context) {
			if c.GetHeader("X-Cookie") != "" {
				p.mainProxyHandler(c)
			} else {
				c.Redirect(302, "https://exhentai.org"+c.Request.URL.String())
			}
			return
		})
	}

	r.GET("/sw.js", func(c *gin.Context) {
		// 1. Force the correct MIME type (Required by some browsers)
		c.Header("Content-Type", "application/javascript")

		// 2. Prevent the script itself from being cached by the browser
		// This ensures that when you change sw.js, the browser sees the change immediately
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")

		c.File("./exhentai/sw.js")
	})
	r.GET("/failed.html", func(c *gin.Context) {
		// 1. Force the correct MIME type (Required by some browsers)
		c.Header("Content-Type", "text/html")

		// 2. Prevent the script itself from being cached by the browser
		// This ensures that when you change sw.js, the browser sees the change immediately
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")

		c.File("./exhentai/failed.html")
	})

	// D. 核心代理逻辑 (包含屏蔽逻辑和内容注入)
	r.NoRoute(p.accessControlMiddleware(), p.mainProxyHandler)

	r.Run(addr)
}

// --- 中间件部分 ---

// 访问控制中间件
// 访问控制中间件
func (p *ProxyHandler) accessControlMiddleware() gin.HandlerFunc {
	// 1. 定义分类白名单
	categories := []string{
		"/doujinshi", "/manga", "/artistcg", "/gamecg", "/non-h",
		"/imageset", "/cosplay", "/asianporn", "/misc", "/western",
	}

	// 2. 定义系统必要路径白名单 (js, css, logo 等)
	systemPaths := []string{
		"/", "/sw.js", "/favicon.ico", "/manifest.json", "/logo192.png", "/failed.html",
		"/popular", "/watched",
	}

	return func(c *gin.Context) {
		// 获取不带参数的路径并转小写
		path := strings.ToLower(c.Request.URL.Path)

		if c.Request.Method != http.MethodGet {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		// --- A. 白名单放行逻辑 ---
		isWhite := false

		// 1. 首页及系统文件
		if slices.Contains(systemPaths, path) {
			isWhite = true
		}

		// 2. 十大分类
		if !isWhite && slices.Contains(categories, path) {
			isWhite = true
		}

		// 3. PHP 文件 (通配 *.php)
		if !isWhite && strings.HasSuffix(path, ".php") {
			isWhite = true
		}

		// 4. 正则匹配: uploader, tag, gallery, image
		if !isWhite {
			if reUploader.MatchString(path) ||
				reTag.MatchString(path) ||
				reZ.MatchString(path) ||
				reImg.MatchString(path) ||
				reTorrent.MatchString(path) ||
				reImage.MatchString(path) ||
				reGallery.MatchString(path) {
				isWhite = true
			}
		}

		// 5. 内部代理资源路径 (exhentai 资源和 static 资源)
		if !isWhite && (strings.HasPrefix(path, "/exhentai/") || strings.HasPrefix(path, "/static/")) {
			isWhite = true
		}

		// --- B. 拦截逻辑 ---
		// 如果不在白名单内，直接拦截
		if !isWhite {
			// 如果有 redirect_to 参数，说明是点击了某些跳转，可以考虑放行
			if c.Query("redirect_to") == "" {
				c.Redirect(302, "https://810114.xyz")
				// c.AbortWithStatus(http.StatusForbidden) // 没在白名单里的一律 404
				return
			}
		}

		// --- C. 基础防护 (即使在白名单，也要防止路径遍历) ---
		if strings.Contains(path, "/../") {
			c.Redirect(302, "https://810114.xyz")
			// c.AbortWithStatus(http.StatusForbidden)
			return
		}

		// --- D. PHP 细化过滤 (可选：如果你只想放行特定的 PHP) ---
		if strings.HasSuffix(path, ".php") {
			allowPHP := []string{
				"/gallerytorrents.php", "/favorites.php", "/torrents.php",
				"/gallerypopups.php", "/api.php", "/uconfig.php", "/archiver.php",
			}
			if !slices.Contains(allowPHP, path) {
				c.Redirect(302, "https://810114.xyz")
				// c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		// --- E. GeoIP 屏蔽 (保留你的原逻辑) ---
		country := c.GetHeader("Cf-Ipcountry")
		acceptLang := strings.ToLower(c.GetHeader("accept-language"))
		if c.Query("redirect_to") != "image" && // 允许重定向啊，傻逼ai。26.02.15
			!strings.Contains(acceptLang, "zh") && !slices.Contains([]string{"CN"}, country) {
			c.Redirect(302, "https://810114.xyz")
			// c.String(http.StatusForbidden, "Restricted Region: "+country)
			c.Abort()
			return
		}

		// --- F. 方法限制 ---
		// 只有 .php 允许 POST，其他一律 GET
		if !strings.HasSuffix(path, ".php") && c.Request.Method != http.MethodGet {
			c.Redirect(302, "https://810114.xyz")
			// c.AbortWithStatus(http.StatusForbidden)
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

		// Set Cache-Control for 1 hour (3600 seconds)
		c.Header("Cache-Control", "public, max-age=86400")

		// 6. 使用 Gin 内置方法（它会自动处理 Range, ETag, Content-Type 等）
		c.File(finalPath)

		return
	}

	if c.Query("host") != "" || strings.HasPrefix(path, "/static/") || path == "/manifest.json" || path == "/logo192.png" {
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

	p.doProxy(c, "https://"+host+c.Request.URL.String())
}

// 核心转发逻辑
func (p *ProxyHandler) doProxy(c *gin.Context, targetURL string) {
	reqHeader := p.prepareHeaders(c)

	resp, err := p.FetchWithRetry(c.Request.Method, targetURL, reqHeader, c.Request.Body)
	if err != nil {
		tools.AbortWithError(c, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()

	u, _ := url.Parse(targetURL)
	targetPath := targetURL
	if u != nil {
		targetPath = u.Path
	}

	// 2. 特殊文件直接流式返回 (Torrent/JS)
	if strings.HasPrefix(targetPath, "/torrent/") || strings.HasPrefix(targetPath, "/z/") || strings.HasPrefix(targetPath, "/api.php") {
		copyHeaders(c, resp.Header)
		if enc := resp.Header.Get("Content-Encoding"); enc != "" {
			c.Header("Content-Encoding", enc)
		}
		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
			"X-Proxy": "proxy",
		})
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
	c.Writer.Header().Del("Content-Length")

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

func (p *ProxyHandler) prepareHeaders(c *gin.Context) http.Header {
	h := c.Request.Header.Clone()

	// 设置 Cookie
	cookie := tools.NewSlice(c.GetHeader("X-Cookie"), os.Getenv("EXHENTAI_PROXY_COOKIE")).FirstUnequal("")
	h.Set("Cookie", cookie)

	// User-Agent 修正
	if strings.HasPrefix(c.Request.URL.String(), "/s/") {
		h.Set("User-Agent", strings.Replace(h.Get("User-Agent"), "Mobile", "Myfetch", 1))
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

	h.Set("Origin", "https://exhentai.org")

	return h
}

// Compile once globally for performance
var beaconRegex = regexp.MustCompile(`(?i)<script[^>]*?static\.cloudflareinsights\.com/beacon\.min\.js[^>]*?>.*?</script>`)

func stripCloudflareBeacon(input []byte) []byte {
	// ReplaceAll operates directly on byte slices
	return beaconRegex.ReplaceAll(input, []byte(""))
}

func (p *ProxyHandler) transformContent(c *gin.Context, data []byte, targetURL string) []byte {
	// 注入外部 Script 标签
	const metaNoReferer = `<meta name="referrer" content="no-referrer">`
	const sw = `<script> window.onbeforeinstallprompt = (e) => e.preventDefault(); if ('serviceWorker' in navigator) { window.addEventListener('load', () => { navigator.serviceWorker.register('/sw.js'); }); } </script>`
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
	// data = reCover.ReplaceAll(data, coverReplacement) // 26.02.14
	// data = ReplaceManual(data) // 26.02.14
	data = bytes.Replace(data, []byte("https://ehgt.org/api.php"), []byte("/api.php"), 1)
	data = reTorrents.ReplaceAll(data, []byte("u.value=114514")) // 26.07.20
	data = stripCloudflareBeacon(data)

	// 执行替换：将 </head> 替换为 <script ...></script></head>
	// return bytes.Replace(data, []byte(headStartTag), []byte(headStartTag+metaNoReferer+scriptTag+cssTag), 1)
	return bytes.Replace(data, []byte(headStartTag), []byte(headStartTag+sw+metaNoReferer+scriptTag), 1)
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
	if c.GetHeader("X-Cookie") != "" {
		p.mainProxyHandler(c)
	} else {
		c.Redirect(302, "https://exhentai.org"+c.Request.URL.String())
	}
	return
}

func (p *ProxyHandler) handleAPI(c *gin.Context) {
	p.doProxy(c, "https://s.exhentai.org/api.php")
	return
}

func copyHeaders(c *gin.Context, h http.Header) {
	for k, vs := range h {
		if k == "Content-Length" || k == "Content-Encoding" {
			continue
		}

		// 特殊处理Content-Disposition头部，修复编码问题
		if k == "Content-Disposition" {
			for _, v := range vs {
				// 尝试修复编码问题
				fixedValue := fixContentDisposition(v)
				c.Writer.Header().Add(k, fixedValue)
			}
			continue
		}

		for _, v := range vs {
			c.Writer.Header().Add(k, v)
		}
	}
}

// 修复Content-Disposition头部的编码问题
func fixContentDisposition(header string) string {
	// 检查是否包含乱码字符
	// if strings.Contains(header, "ã") || strings.Contains(header, "ã") {
	// 提取文件名部分
	if idx := strings.Index(header, "filename="); idx != -1 {
		filenamePart := header[idx+9:] // "filename=" 长度是9

		// 如果文件名被引号包围
		if strings.HasPrefix(filenamePart, "\"") {
			filenamePart = filenamePart[1:]
			if endIdx := strings.Index(filenamePart, "\""); endIdx != -1 {
				filenamePart = filenamePart[:endIdx]
			}
		}

		// 解码乱码（假设是UTF-8被错误解析为ISO-8859-1）
		decodedFilename := fixUTF8Mojibake(filenamePart)

		// 使用RFC 5987编码规范
		return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
			url.PathEscape(decodedFilename),
			url.PathEscape(decodedFilename))
	}
	// }
	return header
}

// 修复UTF-8乱码（双重编码问题）
func fixUTF8Mojibake(s string) string {
	// 假设字符串是UTF-8被错误地解释为ISO-8859-1
	// 首先将字符串转换为字节（作为ISO-8859-1）
	isoBytes := make([]byte, len(s))
	for i, r := range s {
		isoBytes[i] = byte(r)
	}

	// 然后将这些字节解码为UTF-8
	return string(isoBytes)
}
