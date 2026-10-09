package ehviewer

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed static/*
var staticFS embed.FS

func Run(addr string) {
	if addr == "" {
		return
	}

	r := gin.New()
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false
	r.Use(gin.Logger(), gin.Recovery())

	// 嵌入的静态文件系统
	staticSubFS, _ := fs.Sub(staticFS, "static")

	// 预加载所有静态文件到内存，绕过 gin 的文件路由重定向问题
	type staticFile struct {
		path string
		data []byte
		mime string
	}
	var files []staticFile
	entries, _ := fs.ReadDir(staticSubFS, ".")
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		data, err := fs.ReadFile(staticSubFS, name)
		if err != nil {
			continue
		}
		mime := "text/plain; charset=utf-8"
		switch {
		case strings.HasSuffix(name, ".html"):
			mime = "text/html; charset=utf-8"
		case strings.HasSuffix(name, ".json"):
			mime = "application/json"
		case strings.HasSuffix(name, ".js"):
			mime = "text/javascript; charset=utf-8"
		case strings.HasSuffix(name, ".png"):
			mime = "image/png"
		}
		files = append(files, staticFile{name, data, mime})
	}

	// 注册所有静态文件路由
	for _, f := range files {
		fpath, fdata, fmime := f.path, f.data, f.mime
		r.GET("/"+fpath, func(c *gin.Context) {
			c.Data(http.StatusOK, fmime, fdata)
		})
	}

	// 根路径 → index.html
	indexData, _ := fs.ReadFile(staticSubFS, "index.html")
	r.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
	})

	// SPA fallback: 所有未匹配路由返回 index.html（前端路由接管）
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
	})

	r.Run(addr)
}
