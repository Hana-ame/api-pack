package proxy

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	middleware "github.com/Hana-ame/api-pack/pkg/ginutil/middleware"
	"github.com/gin-gonic/gin"
)

// TwimgProxyV2 与辅助（isAllowedReferer / isVideoThumbPath）保留；
// 旧版 TwimgProxy / PximgProxy / StreamProxy / 分流机制已移除。

// allowedRefererHosts 允许访问的 Referer 域名白名单（防盗链）。
// 非白名单来源（含空 Referer / 解析失败）的请求一律 302 到 video.twimg.com 同 path。
var allowedRefererHosts = map[string]struct{}{
	"x.moonchan.xyz": {},
	"x.810114.xyz":   {},
	"xx.810114.xyz":  {},
	"x.4545810.xyz":  {},
	"xx.4545810.xyz": {},
}

// isAllowedReferer 判断请求 Referer 是否来自白名单域名（精确 hostname 匹配，自动剥端口）。
// 空 Referer / 解析失败 / 不在白名单 → false（将被 302 重定向到 video.twimg.com）。
func isAllowedReferer(r *http.Request) bool {
	referer := r.Header.Get("Referer")
	if referer == "" {
		return false
	}
	u, err := url.Parse(referer)
	if err != nil {
		return false
	}
	_, ok := allowedRefererHosts[u.Hostname()]
	return ok
}

// videoThumbPrefixes video.twimg.com 上的视频缩略图路径前缀（图片，非视频文件）。
// 这类 thumb 与 pbs.twimg.com 无关，必须回源 video.twimg.com；
// 且用户钦定：不做 Referer 防盗链 302，直接本体反代（同其他图片一样代理）。
var videoThumbPrefixes = []string{
	"/amplify_video_thumb/",
	"/ext_tw_video_thumb/",
	"/tweet_video_thumb/",
}

// isVideoThumbPath 判断请求路径是否为 video.twimg.com 的视频缩略图
// （形如 https://video.twimg.com/amplify_video_thumb/...）。
func isVideoThumbPath(path string) bool {
	for _, p := range videoThumbPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// TwimgProxyV2 twimg 反代（部署于 pbs.moonchan.xyz）：
//   - 图片（非 .mp4，含视频缩略图）：用户钦定绝不允许 302，无论 Referer / Cf-Ipcountry
//     一律本体反代 pbs.twimg.com（视频缩略图回源 video.twimg.com），直接返回 200 内容
//   - .mp4 视频：保留 Referer 白名单防盗链——非白名单来源（含空 Referer）302 到 video.twimg.com 同 path+query
//   - StreamVideoProxy 自带 Range 透传 + 200→206 改写 + 中途截断 + 背压 + 客户端断开级联取消
func TwimgProxyV2(addr string) error {
	if addr == "" {
		addr = "127.26.8.10:8080"
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	headerProcesser := func(h http.Header) http.Header {
		h.Set("Referer", "https://x.com")
		return h
	}
	// 图片走本体反代（StreamVideoProxy 自带 Range 透传 + 200→206 改写 + 中途截断 + 背压）
	// 用 StreamVideoProxy 而非 StreamProxy：当 nginx slice 1m 发 Range: bytes=0-1048575 时，
	// 上游若无视 Range 回全量 200，本层主动改写为 206 并只读请求范围内的字节——
	// 杜绝「只要 1MB 却被灌 618MB」的浪费
	imageProxy := StreamVideoProxy("https://pbs.twimg.com", headerProcesser)
	// 视频也走本体反代：直接自己代理访问资源，任何情况下都访问；不 302 跳转，不使用 CF Country
	// 用 StreamVideoProxy（video_stream.go）：视频专属流式反代，支持 Range/断点续传，首帧快
	videoProxy := StreamVideoProxy("https://video.twimg.com", headerProcesser)

	v2Handler := func(c *gin.Context) {
		// 视频缩略图（形如 video.twimg.com/amplify_video_thumb/...）：
		// 用户钦定不做 Referer 防盗链 302，直接本体反代 video.twimg.com，
		// 与其他图片一样由代理直接回源取字节（不把客户端引到原始 twimg）。
		if isVideoThumbPath(c.Request.URL.Path) {
			videoProxy(c)
			return
		}
		// 图片（非 .mp4）：用户钦定（2026-10-29）绝不允许 302——
		// 无论 Referer（含空/非白名单）、无论 Cf-Ipcountry（含 CN 与非 CN），
		// 一律由本体反代 pbs.twimg.com 直接回源，返回 200 内容，绝不把客户端引到原始 twimg。
		// URL.Path 不含 query，所以 /abc.mp4?tag=8 这类带参视频也能命中；非 .mp4 全按图片处理。
		if !strings.HasSuffix(strings.ToLower(c.Request.URL.Path), ".mp4") {
			imageProxy(c)
			return
		}
		// .mp4 视频：保留 Referer 白名单防盗链——非白名单来源（含空 Referer）302 到 video.twimg.com 同 path+query
		if !isAllowedReferer(c.Request) {
			c.Header("Cache-Control", "no-cache, no-store, private")
			c.Redirect(http.StatusFound, "https://video.twimg.com"+c.Request.URL.String())
			return
		}
		videoProxy(c)
	}
	r.GET("/*any", v2Handler)
	r.HEAD("/*any", v2Handler)

	// 不用 r.Run：自定义 listener 开启 TCP keepalive。
	// 惩罚池里挂起排队（不发任何 HTTP 响应）的连接长时间无数据，
	// 会被中间设备/NAT 认为死连接掐掉；OS 定期发 keepalive 探测保活
	lc := net.ListenConfig{KeepAlive: 30 * time.Second}
	ln, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return err
	}
	return (&http.Server{
		Handler:     r,
		IdleTimeout: 120 * time.Second,
		// 不设 ReadTimeout：池内排队请求不发字节，超过 ReadTimeout 会被 RST
		// 客户端主动断开由 TCP keepalive 探测 + request context 取消兜底
		WriteTimeout: 30 * time.Minute,
	}).Serve(ln)
}
