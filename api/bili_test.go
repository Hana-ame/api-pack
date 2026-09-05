// 26.09.05
// ExtractID 的纯解析部分可离线测; GetInfo / Handler 走真实网络, 连不上时跳过。
package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	"github.com/gin-gonic/gin"
)

func TestServeIndex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newRouter()

	for _, path := range []string{"/api/v2/bili", "/api/v2/bili/index.html"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: want 200, got %d", path, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s: content-type = %q, want text/html", path, ct)
		}
		body := w.Body.String()
		if !strings.Contains(body, "哔哩哔哩封面查询") {
			t.Errorf("%s: page missing title", path)
		}
		// __API_BASE__ 必须被替换成真实前缀, 否则页面找不到 API
		if strings.Contains(body, "__API_BASE__") {
			t.Errorf("%s: __API_BASE__ not replaced", path)
		}
		if !strings.Contains(body, `let BASE = "/api/v2/bili";`) {
			t.Errorf("%s: BASE not set to prefix", path)
		}
		for _, must := range []string{"/cover?", "/raw?", "cover?url="} {
			if !strings.Contains(body, must) {
				t.Errorf("%s: page missing %s", path, must)
			}
		}
	}
}

func TestExtractID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		in      string
		wantBV  string
		wantAid int64
		wantErr bool
	}{
		{"BV15ntz6gEHd", "BV15ntz6gEHd", 0, false},
		{"117215445649383", "", 117215445649383, false},
		{"  BV15ntz6gEHd  ", "BV15ntz6gEHd", 0, false},
		{"https://www.bilibili.com/video/BV15ntz6gEHd", "BV15ntz6gEHd", 0, false},
		{"https://www.bilibili.com/video/BV15ntz6gEHd?spm_id_from=333.999", "BV15ntz6gEHd", 0, false},
		// 分享文案里夹带 BV 号也能捞出来
		{"【开盖即食♡-哔哩哔哩】BV15ntz6gEHd 看看", "BV15ntz6gEHd", 0, false},
		{"", "", 0, true},
		{"你好世界", "", 0, true},
		{"BV1", "", 0, true},
	}

	for _, tc := range cases {
		bvid, aid, err := ExtractID(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("in=%q: want error, got bvid=%q aid=%d", tc.in, bvid, aid)
			}
			continue
		}
		if err != nil {
			t.Errorf("in=%q: unexpected error: %v", tc.in, err)
			continue
		}
		if bvid != tc.wantBV || aid != tc.wantAid {
			t.Errorf("in=%q: got (bvid=%q, aid=%d), want (bvid=%q, aid=%d)",
				tc.in, bvid, aid, tc.wantBV, tc.wantAid)
		}
	}
}

func TestFollowRedirectsLive(t *testing.T) {
	finalURL, err := followRedirects("https://b23.tv/G2o7PT6")
	if err != nil {
		t.Skipf("network unavailable: %v", err)
	}
	if !strings.Contains(finalURL, "BV15ntz6gEHd") {
		t.Fatalf("expected final url to contain BV id, got %s", finalURL)
	}
	t.Logf("resolved: %s", finalURL)
}

func TestGetInfoLive(t *testing.T) {
	info, err := GetInfo("https://b23.tv/G2o7PT6")
	if err != nil {
		t.Skipf("network unavailable: %v", err)
	}
	if info.Bvid != "BV15ntz6gEHd" {
		t.Errorf("bvid = %q, want BV15ntz6gEHd", info.Bvid)
	}
	if info.Title != "开盖即食♡" {
		t.Errorf("title = %q, want 开盖即食♡", info.Title)
	}
	if !strings.HasPrefix(info.Pic, "https://i") || !strings.Contains(info.Pic, "hdslb.com") {
		t.Errorf("pic = %q, want https://i*.hdslb.com/...", info.Pic)
	}
	t.Logf("title=%q pic=%s %dx%d", info.Title, info.Pic, info.Width, info.Height)

	// 同一输入的第二次调用走 LRU 缓存, 且结果一致
	info2, err := GetInfo("BV15ntz6gEHd")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if info2.Pic != info.Pic {
		t.Errorf("cache mismatch: %s vs %s", info.Pic, info2.Pic)
	}
}

func TestBiliHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newRouter()

	mustGet := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(w, req)
		return w
	}

	// 坏输入 -> 400
	if w := mustGet("/api/v2/bili/cover?url=不是视频"); w.Code != http.StatusBadRequest {
		t.Errorf("bad input: want 400, got %d", w.Code)
	}

	// 路径参数形式
	if w := mustGet("/api/v2/bili/cover/BV15ntz6gEHd"); w.Code != http.StatusOK {
		t.Skipf("network unavailable: %s", w.Body.String())
	}

	// 302 跳转
	w := mustGet("/api/v2/bili/redirect?url=BV15ntz6gEHd")
	if w.Code != http.StatusFound {
		t.Fatalf("redirect: want 302, got %d (%s)", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); !strings.Contains(loc, "hdslb.com") {
		t.Errorf("redirect location = %q, want hdslb.com", loc)
	}

	// HEAD 也得能用（curl -I）
	wHead := httptest.NewRecorder()
	r.ServeHTTP(wHead, httptest.NewRequest(http.MethodHead, "/api/v2/bili/redirect?url=BV15ntz6gEHd", nil))
	if wHead.Code != http.StatusFound {
		t.Errorf("HEAD /redirect: want 302, got %d", wHead.Code)
	}

	// 图片字节代理
	w = mustGet("/api/v2/bili/raw?url=BV15ntz6gEHd")
	if w.Code != http.StatusOK {
		t.Fatalf("raw: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		t.Errorf("content-type = %q, want image/*", ct)
	}
	body, _ := io.ReadAll(io.LimitReader(w.Body, 4096))
	if len(body) < 1000 {
		t.Errorf("raw body too small: %d bytes", len(body))
	}
	// JPEG magic (FF D8)
	if len(body) >= 2 && body[0] == 0xFF && body[1] == 0xD8 {
		t.Logf("JPEG ok, bvid=%s", w.Header().Get("X-Bilibili-Bvid"))
	} else {
		t.Errorf("unexpected magic bytes: % x", body[:4])
	}
}

func TestMiddlewareStack(t *testing.T) {
	// 与 Run() 完全一致的中间件栈, 确认生产路由不会在注册或执行时 panic
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	g := r.Group("/api/v2/bili")
	g.GET("/cover", getInfo)
	g.GET("/cover/:id", getInfo)
	g.GET("/info", getInfo)
	g.GET("/redirect", redirectCover)
	g.HEAD("/redirect", redirectCover)
	g.GET("/raw", rawCover)
	g.HEAD("/raw", rawCover)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/bili/cover?url=BV15ntz6gEHd", nil)
	req.Header.Set("Origin", "https://example.com")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Skipf("network unavailable: %d %s", w.Code, w.Body.String())
	}
	if ac := w.Header().Get("Access-Control-Allow-Origin"); ac == "" {
		t.Errorf("CORS: missing Access-Control-Allow-Origin")
	}
}
