package opencode

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

var sessionIDRe = regexp.MustCompile(`^ses_[0-9a-f]{12}[0-9A-Za-z]{14}$`)
var requestIDRe = regexp.MustCompile(`^msg_[0-9a-f]{12}[0-9A-Za-z]{14}$`)

// recorderWithCloseNotify wraps httptest.ResponseRecorder so the reverse proxy
// (which checks http.CloseNotifier on the ResponseWriter) can detect client
// disconnects — gin's responseWriter.CloseNotify would otherwise panic on the
// bare recorder inside tests.
func recorderWithCloseNotify() *closeNotifyingRecorder {
	return &closeNotifyingRecorder{ResponseRecorder: httptest.NewRecorder()}
}

type closeNotifyingRecorder struct {
	*httptest.ResponseRecorder
}

func (r *closeNotifyingRecorder) CloseNotify() <-chan bool { return nil }

func TestMintSessionID(t *testing.T) {
	for i := 0; i < 100; i++ {
		if id := mintSessionID(); !sessionIDRe.MatchString(id) {
			t.Fatalf("session id %q does not match gateway regex", id)
		}
	}
}

func TestMintRequestID(t *testing.T) {
	for i := 0; i < 100; i++ {
		if id := mintRequestID(); !requestIDRe.MatchString(id) {
			t.Fatalf("request id %q does not match gateway regex", id)
		}
	}
}

func TestMintSessionIDMonotonic(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		id := mintSessionID()
		if seen[id] {
			t.Fatalf("duplicate session id %q within same millisecond", id)
		}
		seen[id] = true
	}
}

// --- header fingerprint ---

func TestApplyFingerprintHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v2/opencode/v1/chat/completions", nil)
	applyFingerprintHeaders(req)
	if ua := req.Header.Get("User-Agent"); !strings.HasPrefix(ua, "opencode/") {
		t.Fatalf("User-Agent not set: %q", ua)
	}
	if got := req.Header.Get("X-Opencode-Session"); !sessionIDRe.MatchString(got) {
		t.Fatalf("X-Opencode-Session = %q", got)
	}
	if got := req.Header.Get("X-Opencode-Request"); !requestIDRe.MatchString(got) {
		t.Fatalf("X-Opencode-Request = %q", got)
	}
}

func TestApplyFingerprintHeadersPreservesClient(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v2/opencode/v1/chat/completions", nil)
	req.Header.Set("User-Agent", "opencode/1.18.30 desktop")
	req.Header.Set("X-Opencode-Session", "ses_000000000000A1B2C3D4E5F6")
	applyFingerprintHeaders(req)
	if got := req.Header.Get("User-Agent"); got != "opencode/1.18.30 desktop" {
		t.Fatalf("overwrote genuine UA: %q", got)
	}
	if got := req.Header.Get("X-Opencode-Session"); got != "ses_000000000000A1B2C3D4E5F6" {
		t.Fatalf("overwrote genuine session id: %q", got)
	}
}

// --- body rewrite ---

func TestRewriteOpencodeBodyForcesStreamAndQuartet(t *testing.T) {
	body := []byte(`{"model":"nemotron-3.5-lightning-free","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/opencode/v1/chat/completions", bytes.NewReader(body))
	out, model, hasTools, err := rewriteOpencodeBody(req)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if model != "nemotron-3.5-lightning-free" {
		t.Fatalf("model = %q", model)
	}
	if hasTools {
		t.Fatalf("client declared no tools, hasTools should be false")
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if payload["stream"] != true {
		t.Fatalf("stream not forced true: %v", payload["stream"])
	}
	tools, ok := payload["tools"].([]any)
	if !ok || len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %v", payload["tools"])
	}
	for _, want := range fingerprintTools {
		found := false
		for _, tt := range tools {
			if toolNameOf(tt.(map[string]any)) == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("tool %q missing", want)
		}
	}
	if payload["tool_choice"] != "none" {
		t.Fatalf("tool_choice = %v, want none", payload["tool_choice"])
	}
}

func TestRewriteOpencodeBodyPreservesClientTools(t *testing.T) {
	body := []byte(`{"model":"m","messages":[],"tools":[{"type":"function","function":{"name":"bash","parameters":{"type":"object"}}}]}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	out, _, hasTools, err := rewriteOpencodeBody(req)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !hasTools {
		t.Fatalf("client declared bash, hasTools should be true")
	}
	var payload map[string]any
	_ = json.Unmarshal(out, &payload)
	tools := payload["tools"].([]any)
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}
	if payload["tool_choice"] != "auto" {
		t.Fatalf("tool_choice = %v, want auto", payload["tool_choice"])
	}
}

func TestRewriteOpencodeBodyDowngradesDeveloper(t *testing.T) {
	body := []byte(`{"model":"m","messages":[{"role":"developer","content":"be brief"},{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	out, _, _, err := rewriteOpencodeBody(req)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	var payload map[string]any
	_ = json.Unmarshal(out, &payload)
	msgs := payload["messages"].([]any)
	if got := msgs[0].(map[string]any)["role"]; got != "system" {
		t.Fatalf("developer not downgraded: %v", got)
	}
}

func TestRewriteOpencodeBodyEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	out, model, hasTools, err := rewriteOpencodeBody(req)
	if err != nil {
		t.Fatalf("empty body should not error: %v", err)
	}
	if model != "" || hasTools || len(out) != 0 {
		t.Fatalf("empty body should pass through: model=%q hasTools=%v out=%q", model, hasTools, out)
	}
}

// --- endpoint routing ---

func TestEndpointFor(t *testing.T) {
	cases := map[string]string{
		"nemotron-3.5-lightning-free":               "/chat/completions",
		"muse-spark-1.3-contributor-free":           "/responses",
		"union-alpha":                               "/messages",
		"anthropic/muse-spark-1.2-contributor-free": "/responses",
	}
	for model, want := range cases {
		if got := endpointFor(model); got != want {
			t.Fatalf("endpointFor(%q) = %q, want %q", model, got, want)
		}
	}
}

func TestTrimModelID(t *testing.T) {
	if got := trimModelID("anthropic/muse-spark-1.2-contributor-free"); got != "muse-spark-1.2-contributor-free" {
		t.Fatalf("vendor prefix not stripped: %q", got)
	}
	if got := trimModelID("model (thinking)"); got != "model" {
		t.Fatalf("suffix not stripped: %q", got)
	}
}

// --- gate classification ---

func TestClassifyGateError(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`{"error":{"type":"FreeTierError"}}`, "FreeTierError"},
		{`{"error":{"type":"FreeUsageLimitError"}}`, "FreeUsageLimitError"},
		{`{"error":{"type":"RegionError"}}`, "RegionError"},
		{`{"error":{"message":"Model is unavailable"}}`, "ModelUnavailable"},
		{`{"error":{"message":"Endpoint is unavailable"}}`, "EndpointUnavailable"},
		{`{"error":{"message":"Missing API key","type":"AuthError"}}`, "AuthError"},
		{`{"error":{"message":"boom"}}`, ""},
	}
	for _, c := range cases {
		resp := &http.Response{StatusCode: http.StatusForbidden, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(c.body))}
		class, hint := classifyGateError(resp)
		if class != c.want {
			t.Fatalf("classifyGateError(%q) = %q, want %q", c.body, class, c.want)
		}
		if c.want != "" && hint == "" {
			t.Fatalf("no hint for class %q", c.want)
		}
		got, _ := io.ReadAll(resp.Body)
		if string(got) != c.body {
			t.Fatalf("body corrupted: got %q", string(got))
		}
	}
}

// --- SSE helpers ---

func TestIsRealSSE(t *testing.T) {
	if !isRealSSE([]byte("data: {}\n\n")) {
		t.Fatalf("data: should be real SSE")
	}
	if isRealSSE([]byte("HTTP/1.1 200 OK\r\n")) {
		t.Fatalf("HTTP headers are not SSE")
	}
}

func TestNextSSEEvent(t *testing.T) {
	buf := []byte("data: {\"a\":1}\n\ndata: {\"b\":2}")
	ev, rest, ok := nextSSEEvent(buf)
	if !ok || !bytes.Contains(ev, []byte("data: {\"a\":1}")) {
		t.Fatalf("first event wrong: %q ok=%v", ev, ok)
	}
	if !strings.HasPrefix(string(rest), "data: {\"b\":2}") {
		t.Fatalf("remainder wrong: %q", rest)
	}
	if _, _, ok := nextSSEEvent([]byte("data: x")); ok {
		t.Fatalf("incomplete event should not parse")
	}
	ev, _, ok = nextSSEEvent([]byte("data: x\r\n\r\n"))
	if !ok || !bytes.Contains(ev, []byte("data: x")) {
		t.Fatalf("CRLF framing not handled")
	}
}

func TestHasToolCall(t *testing.T) {
	if !hasToolCall([]byte(`data: {"choices":[{"delta":{"tool_calls":[]}}]}`)) {
		t.Fatalf("should detect a tool call")
	}
	if hasToolCall([]byte(`data: {"choices":[{"delta":{"content":"hi"}}]}`)) {
		t.Fatalf("content is not a tool call")
	}
}

func TestHasContent(t *testing.T) {
	if hasContent([]byte(`data: {"choices":[{"delta":{}}]}`)) {
		t.Fatalf("empty delta is not content")
	}
	if !hasContent([]byte(`data: [DONE]`)) {
		t.Fatalf("[DONE] should count")
	}
}

func TestHasError(t *testing.T) {
	if !hasError([]byte(`data: {"error":{"type":"FreeTierError"}}`)) {
		t.Fatalf("should detect error")
	}
	if hasError([]byte(`data: {"choices":[{"delta":{"content":"hi"}}]}`)) {
		t.Fatalf("content is not error")
	}
}

// --- transport selection ---

func TestTransportForNetParam(t *testing.T) {
	if transportFor("v4") == nil {
		t.Fatalf("v4 transport nil")
	}
	if transportFor("v6") == nil {
		t.Fatalf("v6 transport nil")
	}
	if transportFor("auto") == nil {
		t.Fatalf("auto transport nil")
	}
	if transportFor("bogus") == nil {
		t.Fatalf("bogus net should fall back to auto")
	}
}

// --- end-to-end handler ---

func TestOpencodeHandlerEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotUA, gotSession string
	var gotBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotSession = r.Header.Get("X-Opencode-Session")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer upstream.Close()

	old := envGet
	envGet = func(k string) string { return upstream.URL + "/zen/v1" }
	defer func() { envGet = old }()

	r := gin.New()
	Routes(r)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/opencode/v1/chat/completions",
		strings.NewReader(`{"model":"nemotron-3.5-lightning-free","messages":[{"role":"user","content":"hi"}],"stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	rec := recorderWithCloseNotify()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(gotUA, "opencode/") {
		t.Fatalf("User-Agent not forwarded: %q", gotUA)
	}
	if !sessionIDRe.MatchString(gotSession) {
		t.Fatalf("X-Opencode-Session = %q", gotSession)
	}
	var sent map[string]any
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("upstream body not JSON: %v", err)
	}
	if sent["stream"] != true {
		t.Fatalf("stream not forced true: %v", sent["stream"])
	}
	if got, _ := sent["tools"].([]any); len(got) != 4 {
		t.Fatalf("expected 4 tools on the wire, got %d", len(got))
	}
	if !strings.Contains(rec.Body.String(), "data: [DONE]") {
		t.Fatalf("SSE body not forwarded: %q", rec.Body.String())
	}
}

// Upstream EOFs mid-stream -> keepalive injection.
func TestOpencodeHandlerKeepaliveInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"))
	}))
	defer upstream.Close()

	old := envGet
	envGet = func(k string) string { return upstream.URL }
	defer func() { envGet = old }()

	r := gin.New()
	Routes(r)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/opencode/v1/chat/completions",
		strings.NewReader(`{"model":"m","stream":true,"tools":[{"type":"function","function":{"name":"bash","parameters":{"type":"object"}}}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := recorderWithCloseNotify()
	r.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "partial") {
		t.Fatalf("real content lost: %q", body)
	}
	if !strings.Contains(body, `"name":"bash"`) || !strings.Contains(body, "echo 继续") {
		t.Fatalf("keepalive tool call not injected: %q", body)
	}
	if !strings.Contains(body, `"finish_reason":"tool_calls"`) || !strings.Contains(body, "[DONE]") {
		t.Fatalf("keepalive termination missing: %q", body)
	}
}

// Non-SSE path (e.g. /v1/models) passes through untouched.
func TestOpencodeHandlerNonSSEPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"model-a"}]}`))
	}))
	defer upstream.Close()

	old := envGet
	envGet = func(k string) string { return upstream.URL }
	defer func() { envGet = old }()

	r := gin.New()
	Routes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/v2/opencode/v1/models", nil)
	rec := recorderWithCloseNotify()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Body.String(); got != `{"data":[{"id":"model-a"}]}` {
		t.Fatalf("non-SSE body mangled: %q", got)
	}
}

// Gate error from upstream is classified onto a response header.
func TestOpencodeHandlerGateHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"type":"FreeTierError","message":"shape"}`))
	}))
	defer upstream.Close()

	old := envGet
	envGet = func(k string) string { return upstream.URL }
	defer func() { envGet = old }()

	r := gin.New()
	Routes(r)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/opencode/v1/chat/completions",
		strings.NewReader(`{"model":"m"}`))
	rec := recorderWithCloseNotify()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("X-Opencode-Gate"); got != "FreeTierError" {
		t.Fatalf("X-Opencode-Gate = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "FreeTierError") {
		t.Fatalf("error body corrupted: %q", rec.Body.String())
	}
}
