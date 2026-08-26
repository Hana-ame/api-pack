package proxies

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestGenericProxyFreeModelKeyInjection —— 回归测试: FreeModels 免费模型的 APIKey 注入。
//
// 发现背景: 2026-08-25 sensenova.moonchan.xyz 生图页 E2E 调试时暴露。
// generic_proxy.go 里 `model, ok := reqMap.GetOrDefault(...)` 的 := 在 if 块内
// 新建了内层 model, 遮蔽外层 var model string, 导致外层 model 恒为 "",
// config.FreeModels[model] 恒 false, 服务端 APIKey 从未注入,
// 上游 token.sensenova.cn 返回 401 "Authorization Not Found" (code 16)。
// 修复方式: 改为 var ok bool; model, ok = ... 赋值给外层变量。
// 本测试保护: 客户端未带 Authorization + 请求体 model 命中 FreeModels 时,
// 转发上游的请求必须带上 "Bearer <server key>"; 反之自带 key 透传、非免费模型不注入。
func TestGenericProxyFreeModelKeyInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := ProxyConfig{
		Name:     "test-free",
		Endpoint: upstream.URL,
		APIKey:   "sk-server-test-key",
		FreeModels: map[string]bool{
			"test-model-free": true,
		},
	}

	r := gin.New()
	r.Any("/*any", GenericProxyHandler(cfg))

	post := func(body string, auth string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader(body))
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 用例1: 无 Authorization + FreeModels 内模型 → 必须注入服务端 key
	w := post(`{"model":"test-model-free","prompt":"hi"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("case1 status=%d body=%s", w.Code, w.Body.String())
	}
	if gotAuth != "Bearer sk-server-test-key" {
		t.Fatalf("case1: server key not injected, upstream Authorization=%q", gotAuth)
	}

	// 用例2: 客户端自带 Bearer key → 透传客户端的, 不覆盖
	gotAuth = ""
	w = post(`{"model":"test-model-free","prompt":"hi"}`, "Bearer sk-client-key")
	if w.Code != http.StatusOK {
		t.Fatalf("case2 status=%d body=%s", w.Code, w.Body.String())
	}
	if gotAuth != "Bearer sk-client-key" {
		t.Fatalf("case2: client auth should pass through, got %q", gotAuth)
	}

	// 用例3: 无 Authorization + 非 Free 模型 → 不注入 (保持无 auth)
	gotAuth = ""
	w = post(`{"model":"paid-model","prompt":"hi"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("case3 status=%d body=%s", w.Code, w.Body.String())
	}
	if gotAuth != "" {
		t.Fatalf("case3: no injection expected for non-free model, got %q", gotAuth)
	}

	// 用例4: 缺 model 字段 → 400 "model field is required"
	w = post(`{"prompt":"hi"}`, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("case4 status=%d body=%s", w.Code, w.Body.String())
	}
}

// TestGenericProxyErrorMasksSensitiveHeaders —— 回归测试: 上游返回错误(非 2xx)时,
// 回显的 Request/Response Headers 里 Authorization 等敏感头必须用 *** 打码,
// 不能把服务端注入或客户端传来的 APIKey 泄露给前端。
func TestGenericProxyErrorMasksSensitiveHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer upstream.Close()

	cfg := ProxyConfig{
		Name:     "test-mask",
		Endpoint: upstream.URL,
		APIKey:   "sk-super-secret-server-key",
		FreeModels: map[string]bool{
			"test-model-free": true,
		},
		MaskedHeaders: []string{"Authorization", "X-Api-Key", "Cookie"},
	}

	r := gin.New()
	r.Any("/*any", GenericProxyHandler(cfg))

	// 客户端不带 key → 代理注入服务端 key → 错误回显里绝不能出现该 key
	req := httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader(`{"model":"test-model-free","prompt":"hi"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "sk-super-secret-server-key") {
		t.Fatalf("server APIKey leaked in error body:\n%s", body)
	}
	if !strings.Contains(body, "Authorization: ***") {
		t.Fatalf("Authorization header not masked, body:\n%s", body)
	}
}
