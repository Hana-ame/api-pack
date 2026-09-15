package proxy

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestGenericProxy400BodyReturned —— 上游返回 400 + JSON body (无 Content-Encoding).
// 代理必须回显 [Body] 内容.
func TestGenericProxy400BodyReturned(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid parameters","code":15}`))
	}))
	defer upstream.Close()

	cfg := ProxyConfig{Name: "test-400", Endpoint: upstream.URL}
	r := gin.New()
	r.Any("/*any", GenericProxyHandler(cfg))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"model":"sensenova-u1.5-lite","prompt":"hi"}`)))

	body := w.Body.String()
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", w.Code)
	}
	if !strings.Contains(body, "invalid parameters") {
		t.Fatalf("upstream body not included in response:\n%s", body)
	}
}

// TestGenericProxy400GzipEncodingPlainBody —— 上游声明 Content-Encoding: gzip
// 但 body 是明文 (CDN 常见). 修复前 gzip.NewReader 吞掉整个流, 回显为空.
// 修复后先读 buffer 再解压, 解压失败回退原始 bytes.
func TestGenericProxy400GzipEncodingPlainBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"plain not gzip"}`))
	}))
	defer upstream.Close()

	cfg := ProxyConfig{Name: "test-400-gzip", Endpoint: upstream.URL}
	r := gin.New()
	r.Any("/*any", GenericProxyHandler(cfg))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"model":"sensenova-u1.5-lite","prompt":"hi"}`)))

	body := w.Body.String()
	if len(body) == 0 {
		t.Fatal("BUG: 400 response body is empty when Content-Encoding: gzip but body is plain")
	}
	if !strings.Contains(body, "[Body]") {
		t.Fatalf("debug format missing [Body] section:\n%s", body)
	}
	// 解压失败 → 回退原始 bytes (明文)
	if !strings.Contains(body, "plain not gzip") {
		t.Fatalf("raw body not included in debug response:\n%s", body)
	}
}

// TestGenericProxy400GzipEncodingRealBody —— 上游返回真正的 gzip body.
// 代理应解压并回显明文.
func TestGenericProxy400GzipEncodingRealBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write([]byte(`{"error":"real gzip body"}`))
	gw.Close()
	gzipBody := buf.Bytes()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(gzipBody)
	}))
	defer upstream.Close()

	cfg := ProxyConfig{Name: "test-400-gzip-real", Endpoint: upstream.URL}
	r := gin.New()
	r.Any("/*any", GenericProxyHandler(cfg))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"model":"sensenova-u1.5-lite","prompt":"hi"}`)))

	body := w.Body.String()
	if !strings.Contains(body, "real gzip body") {
		t.Fatalf("decompressed body not included in debug response:\n%s", body)
	}
}

// TestGenericProxy400EmptyBody —— 上游返回 400 + 空 body.
// 调试回显仍有 [Body] 段, 后面为空 (这是正确的, 上游本来就没返回 body).
func TestGenericProxy400EmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer upstream.Close()

	cfg := ProxyConfig{Name: "test-400-empty", Endpoint: upstream.URL}
	r := gin.New()
	r.Any("/*any", GenericProxyHandler(cfg))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"model":"sensenova-u1.5-lite","prompt":"hi"}`)))

	body := w.Body.String()
	if !strings.Contains(body, "[Body]") {
		t.Fatalf("debug format missing [Body] section:\n%s", body)
	}
}
