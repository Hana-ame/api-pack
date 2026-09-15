// 26.09.05
// bilibili 封面 API（见 bili.go 的取图流程说明）。
// bilibili 字幕 API（见 bili_subtitle.go 的字幕下载流程说明）。
//
// 不自开端口: main.go 里在根 engine 上 api.Register(r), 作为 /api/v2 系列的一个 handler。
// 必须在 r.Any("/*any", rootProxyHandler) 之前调用, 否则被通配代理吃掉。
//
// 路由:
//
//	GET /api/v2/bili            客户端页面（浏览器直接开这个就能用）
//	GET /api/v2/bili/cover?url=<b23短链|完整地址|BV号|aid>   JSON 元数据（含 pic 封面）
//	GET /api/v2/bili/cover/:id                              同上，id 放在路径里
//	GET /api/v2/bili/info?url=...                           /cover 的别名
//	GET /api/v2/bili/redirect?url=...                       302 跳到封面直链
//	GET /api/v2/bili/raw?url=...                            直接回封面图片字节（可当 <img> src）
//	GET /api/v2/bili/subtitle?url=...                       JSON 字幕数据（含 VTT + 元信息）
//	GET /api/v2/bili/subtitle/:id                           同上，id 放在路径里
//	GET /api/v2/bili/subtitle/vtt?url=...                   直接回 VTT 格式字幕文本
//	GET /api/v2/bili/subtitle/vtt/:id                       同上，id 放在路径里
//	GET /api/v2/bili/subtitle.html                          字幕操作界面（粘贴文本自动提取 BV 号）
//	GET /api/v2/bili/ui/subtitle                            同上
//
// 入参兼容三种写法: ?url= / ?u= / ?id= ，也支持 /cover/BV1xxxx 这种路径写法。
package bilibili

import (
	"bytes"
	"embed"
	"io"
	"net/http"
	"strings"
	"time"

	tools "github.com/Hana-ame/api-pack/pkg/utils"
	"github.com/gin-gonic/gin"
)

//go:embed index.html
var indexHTML embed.FS

//go:embed subtitle.html
var subtitleHTML embed.FS

// routePrefix 所有路由的挂载前缀; 页面用它定位 API, 所以别乱改
const routePrefix = "/api/v2/bili"

// Register 把 /api/v2/bili 系列路由挂到外部 engine 上, 不自开端口。
// 调用方不要用 r.Any("/*any", ...) 做兜底——gin 不允许 root 级 catch-all 和静态路由
// 共存（会 panic）, 得用 r.NoRoute(...)。见 main.go。
func Register(r *gin.Engine) {
	cover := r.Group(routePrefix)
	{
		// 前端页面: 浏览器直接开 /api/v2/bili 就能用
		// serveIndex 把 HTML 里的 __API_BASE__ 替换成真实前缀, 页面才知道去哪调 API
		cover.GET("", serveIndex(routePrefix))
		cover.GET("/index.html", serveIndex(routePrefix))

		cover.GET("/cover", getInfo)
		cover.GET("/cover/:id", getInfo)
		cover.GET("/info", getInfo)

		// 字幕下载: 优先中文，fallback 英文
		cover.GET("/subtitle", getSubtitleJSON)
		cover.GET("/subtitle/:id", getSubtitleJSON)
		cover.GET("/subtitle/vtt", getSubtitleVTT)
		cover.GET("/subtitle/vtt/:id", getSubtitleVTT)

		// 字幕操作页面
		cover.GET("/subtitle.html", serveSubtitleIndex(routePrefix))
		cover.GET("/ui/subtitle", serveSubtitleIndex(routePrefix))

		// 显式注册 HEAD: 这个 gin 版本不会把 HEAD 自动映射到 GET, 不注册的话 curl -I 会 404
		cover.GET("/redirect", redirectCover)
		cover.HEAD("/redirect", redirectCover)
		cover.GET("/raw", rawCover)
		cover.HEAD("/raw", rawCover)
	}
}

// serveIndex 返回嵌进二进制的客户端页面（api/index.html, 用 go:embed 打进包）。
// 页面里的 __API_BASE__ 在这里替换成 base（去掉尾部斜杠）—— 页面靠它知道去哪调 API。
func serveIndex(base string) gin.HandlerFunc {
	apiBase := strings.TrimSuffix(base, "/")
	return func(c *gin.Context) {
		b, err := indexHTML.ReadFile("index.html")
		if err != nil {
			tools.AbortWithError(c, http.StatusInternalServerError, err)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8",
			bytes.ReplaceAll(b, []byte("__API_BASE__"), []byte(apiBase)))
	}
}

// serveSubtitleIndex 返回嵌进二进制的字幕操作页面（api/subtitle.html）。
// 页面里的 __API_BASE__ 在这里替换成 base（去掉尾部斜杠）。
func serveSubtitleIndex(base string) gin.HandlerFunc {
	apiBase := strings.TrimSuffix(base, "/")
	return func(c *gin.Context) {
		b, err := subtitleHTML.ReadFile("subtitle.html")
		if err != nil {
			tools.AbortWithError(c, http.StatusInternalServerError, err)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8",
			bytes.ReplaceAll(b, []byte("__API_BASE__"), []byte(apiBase)))
	}
}

// inputFrom 从 query（url/u/id）或路径参数里取出输入
func inputFrom(c *gin.Context) string {
	if id := c.Param("id"); id != "" {
		return id
	}
	for _, k := range []string{"url", "u", "id", "bvid", "link"} {
		if v := c.Query(k); v != "" {
			return v
		}
	}
	return ""
}

// getInfo 返回视频元数据 JSON（pic 字段即封面）
func getInfo(c *gin.Context) {
	info, err := GetInfo(inputFrom(c))
	if err != nil {
		tools.AbortWithError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

// redirectCover 302 跳转到封面直链
func redirectCover(c *gin.Context) {
	info, err := GetInfo(inputFrom(c))
	if err != nil {
		tools.AbortWithError(c, http.StatusBadRequest, err)
		return
	}
	if info.Pic == "" {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "empty pic", "bvid": info.Bvid})
		return
	}
	c.Redirect(http.StatusFound, info.Pic)
}

// rawCover 代理封面图片字节，方便直接作为 <img src> 使用
func rawCover(c *gin.Context) {
	info, err := GetInfo(inputFrom(c))
	if err != nil {
		tools.AbortWithError(c, http.StatusBadRequest, err)
		return
	}
	if info.Pic == "" {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "empty pic"})
		return
	}

	req, err := http.NewRequest(http.MethodGet, info.Pic, nil)
	if err != nil {
		tools.AbortWithError(c, http.StatusInternalServerError, err)
		return
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://www.bilibili.com/video/"+info.Bvid)

	// 继承 DefaultClient.Transport（main.go 里配了 LocalAddr 的自定义 transport）,
	// 但不复用它的 CheckRedirect——生产环境那个返回 ErrUseLastResponse, 遇到 302 会卡住。
	// Transport 为 nil（测试进程）时 http.Client 自动退回 http.DefaultTransport。
	client := &http.Client{Timeout: 30 * time.Second, Transport: http.DefaultClient.Transport}
	resp, err := client.Do(req)
	if err != nil {
		tools.AbortWithError(c, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
			"error": "upstream status " + resp.Status,
			"cover": info.Pic,
			"bvid":  info.Bvid,
			"title": info.Title,
		})
		return
	}

	ctype := resp.Header.Get("Content-Type")
	if ctype == "" {
		ctype = "image/jpeg"
	}
	c.Header("Content-Type", ctype)
	c.Header("Cache-Control", "public, max-age=86400")
	c.Header("X-Bilibili-Bvid", info.Bvid)
	if l := resp.Header.Get("Content-Length"); l != "" {
		c.Header("Content-Length", l)
	}

	_, _ = io.Copy(c.Writer, io.LimitReader(resp.Body, 32<<20))
}

// getSubtitleJSON 返回字幕 JSON（含 VTT 内容 + 元信息 + 可用语言列表）
func getSubtitleJSON(c *gin.Context) {
	result, err := GetSubtitle(inputFrom(c))
	if err != nil {
		tools.AbortWithError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// getSubtitleVTT 直接返回 VTT 格式字幕文本，可用作 <track src="..."> 的 src
func getSubtitleVTT(c *gin.Context) {
	result, err := GetSubtitle(inputFrom(c))
	if err != nil {
		tools.AbortWithError(c, http.StatusBadRequest, err)
		return
	}
	c.Header("Content-Type", "text/vtt; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("X-Bilibili-Bvid", result.Bvid)
	c.Header("X-Bilibili-Lang", result.Lang)
	c.String(http.StatusOK, result.VTT)
}
