package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// jsonBody 构造 {"model":"..."} 请求体, 避免测试源码里手写转义 JSON。
func jsonBody(model string) string {
	b, _ := json.Marshal(map[string]string{"model": model})
	return string(b)
}

// newGateRouter 复刻 RunProxyRouter 的中间件装法(入站检查 -> 代理),
// RunProxyRouter 本身阻塞在 r.Run, 测试里不能直接跑。
func newGateRouter(cfg ProxyConfig) *gin.Engine {
	r := gin.New()
	if cfg.InboundToken != "" {
		r.Use(InboundAuthMiddleware(cfg.InboundToken))
	}
	r.Any("/*any", GenericProxyHandler(cfg))
	return r
}

// TestInboundAuthMiddleware —— 前置 auth 检查(单人自用):
// Bearer 命中放通; 裸 token 容错放通; 错误/缺失 401; OPTIONS 放行; 空配置永不放通。
func TestInboundAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	inner := gin.New()
	inner.Use(InboundAuthMiddleware("gate-secret"))
	inner.Any("/ping", func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			c.String(200, "opt")
			return
		}
		c.String(200, "pong")
	})

	hit := func(method, auth string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/ping", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		inner.ServeHTTP(w, req)
		return w
	}

	if w := hit("GET", "Bearer gate-secret"); w.Code != 200 {
		t.Errorf("valid bearer: %d", w.Code)
	}
	if w := hit("GET", "gate-secret"); w.Code != 200 {
		t.Errorf("bare token tolerance: %d", w.Code)
	}
	if w := hit("GET", "bearer gate-secret"); w.Code != 200 { // scheme 大小写不敏感
		t.Errorf("lowercase scheme: %d", w.Code)
	}
	if w := hit("GET", "Bearer wrong"); w.Code != 401 {
		t.Errorf("wrong token: %d", w.Code)
	}
	if w := hit("GET", ""); w.Code != 401 {
		t.Errorf("missing token: %d", w.Code)
	}
	if w := hit("OPTIONS", ""); w.Code != 200 {
		t.Errorf("OPTIONS must bypass gate, got %d", w.Code)
	}

	// 空 expected: 恒 401, 绝不因为配置丢失变成全放通
	empty := gin.New()
	empty.Use(InboundAuthMiddleware(""))
	empty.GET("/ping", func(c *gin.Context) { c.String(200, "pong") })
	req := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	empty.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("empty expected must reject, got %d", w.Code)
	}
}

// TestXingyueModelTokenOverride —— 端到端: 入站检查 + 按模型注入不同出站 token,
// 且门禁 token 绝不透传给上游。
func TestXingyueModelTokenOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	doc := map[string]any{"tokens": map[string]any{
		"m1":     "tok-m1",
		"mgroup": []any{"g1", "g2"},
		"*":      "tok-fb",
	}}
	b, _ := json.Marshal(doc)
	fn := filepath.Join(dir, "tok.json")
	if err := os.WriteFile(fn, b, 0o600); err != nil {
		t.Fatal(err)
	}
	tab, err := NewModelTokenTable(fn + "#$.tokens")
	if err != nil {
		t.Fatalf("table: %v", err)
	}

	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		ok, _ := json.Marshal(map[string]bool{"ok": true})
		w.Write(ok)
	}))
	defer upstream.Close()

	r := newGateRouter(ProxyConfig{
		Name:         "xingyue-test",
		Endpoint:     upstream.URL,
		InboundToken: "gate-secret",
		ModelTokens:  tab,
	})

	post := func(body, auth string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 1. 门禁 token 进门, 上游看到的是模型专属 token(入站 token 被剥离+按模型覆盖)
	w := post(jsonBody("m1"), "Bearer gate-secret")
	if w.Code != 200 || gotAuth != "Bearer tok-m1" {
		t.Fatalf("case1 code=%d upstreamAuth=%q", w.Code, gotAuth)
	}
	if strings.Contains(w.Body.String(), "gate-secret") || strings.Contains(w.Body.String(), "tok-m1") {
		t.Fatalf("response leaked secrets: %s", w.Body.String())
	}

	// 2. token 组轮转: 同一模型连续两次拿到组内两个不同 token
	post(jsonBody("mgroup"), "Bearer gate-secret")
	first := gotAuth
	post(jsonBody("mgroup"), "Bearer gate-secret")
	second := gotAuth
	if first == second {
		t.Fatalf("rotation broken: both %q", first)
	}
	for _, v := range []string{first, second} {
		if v != "Bearer g1" && v != "Bearer g2" {
			t.Fatalf("rotation value unexpected: %q", v)
		}
	}

	// 3. 未命中模型走 "*" 兜底
	post(jsonBody("other"), "Bearer gate-secret")
	if gotAuth != "Bearer tok-fb" {
		t.Fatalf("case3 fallback: %q", gotAuth)
	}

	// 4. GET 无 JSON 体: 不报 400; 门禁 token 被剥离, 上游看不到任何 Authorization
	req := httptest.NewRequest("GET", "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer gate-secret")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("case4 status=%d body=%s", w.Code, w.Body.String())
	}
	if gotAuth != "" {
		t.Fatalf("gate token leaked upstream: %q", gotAuth)
	}

	// 5. 错误/缺失门禁 token -> 401, 不碰上游
	gotAuth = "SENTINEL"
	w = post(jsonBody("m1"), "Bearer nope")
	if w.Code != 401 || gotAuth != "SENTINEL" {
		t.Fatalf("case5 code=%d gotAuth=%q", w.Code, gotAuth)
	}
	w = post(jsonBody("m1"), "")
	if w.Code != 401 {
		t.Fatalf("case5b code=%d", w.Code)
	}
}

// TestModelTokensPassthroughWithoutGate —— 不开入站检查时: 模型未命中表则原样透传
// 客户端自带 key; 命中表才覆盖。
func TestModelTokensPassthroughWithoutGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	doc := map[string]any{"tokens": map[string]any{"m1": "tok-m1"}}
	b, _ := json.Marshal(doc)
	fn := filepath.Join(dir, "t.json")
	if err := os.WriteFile(fn, b, 0o600); err != nil {
		t.Fatal(err)
	}
	tab, err := NewModelTokenTable(fn + "#$.tokens")
	if err != nil {
		t.Fatal(err)
	}
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	r := newGateRouter(ProxyConfig{Name: "t", Endpoint: upstream.URL, ModelTokens: tab})

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(jsonBody("other")))
	req.Header.Set("Authorization", "Bearer client-own-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || gotAuth != "Bearer client-own-key" {
		t.Fatalf("passthrough broken: code=%d auth=%q", w.Code, gotAuth)
	}

	req = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(jsonBody("m1")))
	req.Header.Set("Authorization", "Bearer client-own-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || gotAuth != "Bearer tok-m1" {
		t.Fatalf("override missing: code=%d auth=%q", w.Code, gotAuth)
	}
}
