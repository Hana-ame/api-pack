package proxy

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

// TestStripBodyKeys —— DropBodyKeys 顶层剥键: 命中删除、非命中保留、值字节保真。
func TestStripBodyKeys(t *testing.T) {
	body := `{"model":"gemma-4-26b-a4b-it","store":false,"stream_options":{"include_usage":true},"messages":[],"max_tokens":100,"n":1.5}`
	got, err := stripBodyKeys([]byte(body), []string{"store", "stream_options"})
	if err != nil {
		t.Fatalf("stripBodyKeys err: %v", err)
	}
	s := string(got)
	for _, banned := range []string{"store", "stream_options", "include_usage"} {
		if strings.Contains(s, banned) {
			t.Errorf("被删字段仍存在 %q: %s", banned, s)
		}
	}
	for _, kept := range []string{`"model":"gemma-4-26b-a4b-it"`, `"max_tokens":100`, `"n":1.5`} {
		if !strings.Contains(s, kept) {
			t.Errorf("应保留字段丢失 %q: %s", kept, s)
		}
	}
}

// TestStripBodyKeysNonJSON —— 非 JSON 体(如 GET 无体)原样返回 + err, 不透传乱改。
func TestStripBodyKeysNonJSON(t *testing.T) {
	if out, err := stripBodyKeys([]byte(""), []string{"store"}); err == nil {
		t.Errorf("空体应报 err, 得到 %q", out)
	}
	if out, err := stripBodyKeys([]byte("plain text"), []string{"store"}); err == nil {
		t.Errorf("非 JSON 应报 err, 得到 %q", out)
	}
	if out, err := stripBodyKeys([]byte(`[1,2,3]`), []string{"store"}); err == nil {
		t.Errorf("数组应报 err, 得到 %q", out)
	}
}

// TestStripBodyKeysHandler —— 端到端: DropBodyKeys 配置下, 上游收到的 body 不含被删键。
func TestStripBodyKeysHandler(t *testing.T) {
	var upstreamBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		r.Body.Read(b)
		upstreamBody = string(b)
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	gin.SetMode(gin.TestMode)
	h := GenericProxyHandler(ProxyConfig{
		Name:         "t",
		Endpoint:     upstream.URL,
		DropBodyKeys: []string{"store", "stream_options"},
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions",
		strings.NewReader(`{"model":"m","store":false,"stream_options":{"include_usage":true},"messages":[]}`))
	w := httptest.NewRecorder()
	h(ginCtx(req, w))

	if strings.Contains(upstreamBody, "store") || strings.Contains(upstreamBody, "include_usage") {
		t.Errorf("上游仍收到被删字段: %s", upstreamBody)
	}
	if !strings.Contains(upstreamBody, `"model":"m"`) {
		t.Errorf("上游 body 丢失 model: %s", upstreamBody)
	}
}

func ginCtx(req *http.Request, w *httptest.ResponseRecorder) *gin.Context {
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c
}
