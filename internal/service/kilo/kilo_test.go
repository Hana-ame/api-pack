package kilo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

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

// withEnv swaps envGet for the duration of a test. envGet is package-level
// state, so restoring it in Cleanup keeps later tests independent of the order
// the runner happens to execute them in.
func withEnv(t *testing.T, env map[string]string) {
	t.Helper()
	old := envGet
	envGet = func(k string) string { return env[k] }
	t.Cleanup(func() { envGet = old })
}

// resetTransports drops the cached transports. They are package-level state too,
// and a leftover *http.Transport whose Proxy func points at a dead test proxy
// would otherwise leak into the next test that happens to request a proxy.
func resetTransports() {
	trMu.Lock()
	defer trMu.Unlock()
	if trProxy != nil {
		trProxy.CloseIdleConnections()
	}
	trDirect = nil
	trProxy = nil
	trProxySpec = ""
}

// freshEngine builds a gin engine with only the kilo routes mounted, so tests
// never trip gin's duplicate-route panic.
func freshEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Routes(r)
	return r
}

func newResp(status int, body string, header http.Header) *http.Response {
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		StatusCode:    status,
		Header:        header,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Proto:         "HTTP/2.0",
		ProtoMajor:    2,
		ProtoMinor:    0,
	}
}

// --- egress spec parsing ---

func TestParseEgressURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"socks5://172.29.80.1:10808", "socks5://172.29.80.1:10808"},
		// curl's habit, folded onto socks5: Go's socks5 path already hands the
		// hostname to the proxy, so socks5h adds nothing here.
		{"socks5h://172.29.80.1:10808", "socks5://172.29.80.1:10808"},
		{"socks://172.29.80.1:10808", "socks5://172.29.80.1:10808"},
		{"socks5://172.29.80.1", "socks5://172.29.80.1:1080"},
		// bare host:port is the natural shorthand for a local SOCKS listener.
		{"172.29.80.1:10808", "socks5://172.29.80.1:10808"},
		{"http://127.0.0.1:7890", "http://127.0.0.1:7890"},
		{"https://127.0.0.1:8443", "https://127.0.0.1:8443"},
		{"HTTPS://example.com", "https://example.com:443"},
		{"http://example.com", "http://example.com:80"},
		{" socks5://172.29.80.1:10808 ", "socks5://172.29.80.1:10808"},
		{"socks5://[::1]", "socks5://[::1]:1080"},
		{"socks5://[::1]:1080", "socks5://[::1]:1080"},
		{"socks5://::1:1080", "socks5://[::1]:1080"},
	}
	for _, c := range cases {
		u, err := parseEgressURL(c.in)
		if err != nil {
			t.Fatalf("parseEgressURL(%q) error: %v", c.in, err)
		}
		if got := u.String(); got != c.want {
			t.Errorf("parseEgressURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseEgressURLError(t *testing.T) {
	for _, spec := range []string{"", "   ", "vless://1.2.3.4:443", "ssh://1.2.3.4:22", "ftp://example.com", "http://"} {
		if _, err := parseEgressURL(spec); err == nil {
			t.Fatalf("parseEgressURL(%q) should fail", spec)
		}
	}
}

func TestNormalizeEgress(t *testing.T) {
	cases := map[string]egressMode{
		"":          egressAuto,
		"auto":      egressAuto,
		"gibberish": egressAuto,
		"direct":    egressDirect,
		"off":       egressDirect,
		"none":      egressDirect,
		"DIRECT":    egressDirect,
		"proxy":     egressProxy,
		"on":        egressProxy,
		"egress":    egressProxy,
		" PROXY ":   egressProxy,
	}
	for in, want := range cases {
		if got := normalizeEgress(in); got != want {
			t.Errorf("normalizeEgress(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- transport selection ---

func TestTransportForEgressModes(t *testing.T) {
	resetTransports()
	t.Cleanup(resetTransports)

	withEnv(t, nil)
	d, err := transportForEgress(egressAuto)
	if err != nil {
		t.Fatalf("auto with KILO_EGRESS unset should be direct, got %v", err)
	}
	if d.Proxy != nil {
		t.Fatalf("direct transport must not proxy")
	}
	// The direct transport is cached: the same instance is reused so keepalives
	// (and in-flight SSE) survive across requests.
	if d2, _ := transportForEgress(egressAuto); d2 != d {
		t.Fatalf("direct transport not cached")
	}

	withEnv(t, map[string]string{"KILO_EGRESS": "socks5://172.29.80.1:10808"})
	p, err := transportForEgress(egressAuto)
	if err != nil {
		t.Fatalf("auto with KILO_EGRESS set should proxy: %v", err)
	}
	if p == d {
		t.Fatalf("proxy and direct must be distinct transports")
	}
	pf := p.Proxy
	if pf == nil {
		t.Fatalf("proxy transport has no Proxy func")
	}
	reqURL, err := url.Parse("https://api.kilo.ai/api/gateway/v1/chat/completions")
	if err != nil {
		t.Fatalf("parse test URL: %v", err)
	}
	pu, err := pf(&http.Request{URL: reqURL, Host: "api.kilo.ai"})
	if err != nil {
		t.Fatalf("proxy func failed: %v", err)
	}
	if pu == nil || pu.String() != "socks5://172.29.80.1:10808" {
		t.Fatalf("proxy func returned %v, want socks5://172.29.80.1:10808", pu)
	}
	if _, err := transportForEgress(egressDirect); err != nil {
		t.Fatalf("explicit direct should never fail: %v", err)
	}
}

func TestTransportForEgressProxyErrors(t *testing.T) {
	resetTransports()
	t.Cleanup(resetTransports)

	withEnv(t, nil)
	if _, err := transportForEgress(egressProxy); err == nil {
		t.Fatalf("?egress=proxy with KILO_EGRESS unset must fail")
	}

	withEnv(t, map[string]string{"KILO_EGRESS": "vless://1.2.3.4:443"})
	if _, err := transportForEgress(egressAuto); err == nil {
		t.Fatalf("auto with an unsupported KILO_EGRESS scheme must fail")
	}
}

func TestTransportForEgressCacheSwap(t *testing.T) {
	resetTransports()
	t.Cleanup(resetTransports)

	withEnv(t, map[string]string{"KILO_EGRESS": "socks5://172.29.80.1:10808"})
	a, err := transportForEgress(egressAuto)
	if err != nil {
		t.Fatalf("first proxy: %v", err)
	}
	// Same spec: reused, so idle keepalives are preserved.
	if a2, _ := transportForEgress(egressAuto); a2 != a {
		t.Fatalf("same egress spec should reuse the cached transport")
	}
	// New spec: swapped in, and the old one's idle conns drained.
	withEnv(t, map[string]string{"KILO_EGRESS": "socks5://10.0.0.9:7890"})
	b, err := transportForEgress(egressAuto)
	if err != nil {
		t.Fatalf("second proxy: %v", err)
	}
	if b == a {
		t.Fatalf("changing KILO_EGRESS must swap in a new transport")
	}
}

func TestTransportForWrapsConfigErrors(t *testing.T) {
	resetTransports()
	t.Cleanup(resetTransports)

	withEnv(t, nil)
	rt := transportFor(egressProxy)
	_, err := rt.RoundTrip(&http.Request{Host: "api.kilo.ai"})
	if err == nil {
		t.Fatalf("errRoundTripper should fail every request")
	}
	class, hint := transportErrorHint(err)
	if class != gateEgressMisconfigured {
		t.Fatalf("class = %q, want %q", class, gateEgressMisconfigured)
	}
	if !strings.Contains(hint, "egress misconfigured") {
		t.Fatalf("hint = %q", hint)
	}
}

// --- failure classification ---

func TestTransportErrorHint(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		// The verified CN signature: RST right after ClientHello.
		{errors.New("read tcp 10.0.0.5:52344->104.18.39.49:443: read: connection reset by peer"), gateEgressBlocked},
		{errors.New("read tcp 10.0.0.5:52344->104.18.39.49:443: EOF"), gateEgressBlocked},
		{errors.New("EOF"), gateEgressBlocked},
		{errors.New("http2: transport: TLS handshake timeout"), gateEgressBlocked},
		{errors.New("remote error: tls: internal error"), gateEgressBlocked},
		// Dial failures: the proxy or the gateway is simply not reachable.
		{errors.New("dial tcp 172.29.80.1:10808: connect: connection refused"), gateEgressUnreachable},
		{errors.New("proxyconnect tcp: dial tcp 172.29.80.1:10808: connect: no route to host"), gateEgressUnreachable},
		{errors.New("Get \"https://api.kilo.ai\": dial tcp: lookup api.kilo.ai: no such host"), gateEgressUnreachable},
		{errors.New("i/o timeout"), gateEgressUnreachable},
		// A vanished client is nobody's upstream fault.
		{errors.New("context canceled"), ""},
		{context.Canceled, ""},
		{errors.New("net/http: request canceled (Client.Timeout exceeded while awaiting headers)"), ""},
		// Nothing we recognise: stay quiet rather than guess.
		{errors.New("the moon is far away"), ""},
		{nil, ""},
	}
	for _, c := range cases {
		class, hint := transportErrorHint(c.err)
		if class != c.want {
			t.Errorf("transportErrorHint(%v) = %q, want %q", c.err, class, c.want)
		}
		if class != "" && hint == "" {
			t.Errorf("transportErrorHint(%v) returned class without a hint", c.err)
		}
		if class == "" && hint != "" {
			t.Errorf("transportErrorHint(%v) returned hint without a class", c.err)
		}
	}
}

func TestTransportErrorHintWrapped(t *testing.T) {
	wrapped := fmt.Errorf("Post \"https://api.kilo.ai/api/gateway/v1/chat/completions\": %w",
		errors.New("read tcp 10.0.0.5:52344->104.18.39.49:443: read: connection reset by peer"))
	if class, _ := transportErrorHint(wrapped); class != gateEgressBlocked {
		t.Fatalf("wrapped error not classified: %q", class)
	}
	cfg := &egressConfigError{detail: "KILO_EGRESS unset"}
	if class, _ := transportErrorHint(fmt.Errorf("outer: %w", cfg)); class != gateEgressMisconfigured {
		t.Fatalf("wrapped config error not classified: %q", class)
	}
	if class, _ := transportErrorHint(fmt.Errorf("outer: %w", context.Canceled)); class != "" {
		t.Fatalf("wrapped cancellation must stay unclassified, got %q", class)
	}
}

func TestClassifyErrorText(t *testing.T) {
	cases := []struct {
		status      int
		text        string
		reset       string
		want        string
		wantResetIn bool
	}{
		{429, `{"error":{"message":"Rate limit exceeded"}}`, "", gateProviderLimit, false},
		{429, `{"error":{"message":"too many requests"}}`, "", gateProviderLimit, false},
		{400, `{"error":{"message":"Daily limit reached"}}`, "2026-10-08T00:00:00Z", gateDailyLimit, true},
		{503, `{"error":{"message":"Service temporarily overloaded"}}`, "", gateOverloaded, false},
		{200, `{"error":{"message":"Upstream error from Nvidia: overloaded"}}`, "", gateOverloaded, false},
		{401, `{"error":{"message":"Unauthorized"}}`, "", gateAuth, false},
		{403, `{"error":{"message":"Forbidden"}}`, "", gateAuth, false},
		{200, `{"error":{"message":"fine"}}`, "", "", false},
		{200, `{"choices":[]}`, "", "", false},
	}
	for _, c := range cases {
		class, hint := classifyErrorText(c.status, c.text, c.reset)
		if class != c.want {
			t.Errorf("classifyErrorText(%d, %q) = %q, want %q", c.status, c.text, class, c.want)
		}
		if class != "" && hint == "" {
			t.Errorf("classifyErrorText(%d, %q) returned class without hint", c.status, c.text)
		}
		if c.wantResetIn && !strings.Contains(hint, c.reset) {
			t.Errorf("classifyErrorText(%d, %q) hint missing reset time: %q", c.status, c.text, hint)
		}
	}
}

func TestClassifyErrorResponseRestoresBody(t *testing.T) {
	body := `{"error":{"message":"too many requests"}}`
	resp := newResp(429, body, http.Header{})
	class, _ := classifyErrorResponse(resp)
	if class != gateProviderLimit {
		t.Fatalf("class = %q", class)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("re-reading body: %v", err)
	}
	if string(got) != body {
		t.Fatalf("body corrupted: got %q, want %q", got, body)
	}
}

func TestClassifyJSONErrorSkipsStreamedAndLarge(t *testing.T) {
	skipped := []*http.Response{
		{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":{"code":503}}`)), ContentLength: -1},
		{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":{"code":503}}`)), ContentLength: sniffLimit + 1},
		{StatusCode: 200, Header: http.Header{"Content-Encoding": []string{"gzip"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"code":503}}`)), ContentLength: 30},
		{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":null}`)), ContentLength: 16},
		{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"OK"}}]}`)), ContentLength: 45},
		{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(``)), ContentLength: 0},
		{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`[1,2]`)), ContentLength: 5},
	}
	for i, resp := range skipped {
		if class, _ := classifyJSONError(resp); class != "" {
			t.Errorf("case %d should be skipped, got %q", i, class)
		}
	}

	got, _ := io.ReadAll(skipped[0].Body)
	if string(got) != `{"error":{"code":503}}` {
		t.Fatalf("skipped body must be untouched, got %q", got)
	}
}

func TestClassifyJSONErrorBodyPassesThrough(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`{"error":{"code":503,"message":"Upstream error from Nvidia"}}`, gateOverloaded},
		{`{"error":{"code":429,"message":"Daily limit reached"}}`, gateDailyLimit},
		{`{"error":{"code":401,"message":"Unauthorized"}}`, gateAuth},
		// Inner code below 400 does not override the outer 200.
		{`{"error":{"code":200,"message":"all good"}}`, ""},
		// Inner code is a string, not a number: outer status wins (200 -> none).
		{`{"error":{"code":"429","message":"ignored"}}`, ""},
	}
	for _, c := range cases {
		resp := newResp(200, c.body, http.Header{})
		class, _ := classifyJSONError(resp)
		if class != c.want {
			t.Errorf("classifyJSONError(%q) = %q, want %q", c.body, class, c.want)
		}
		got, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("re-reading %q: %v", c.body, err)
		}
		if string(got) != c.body {
			t.Fatalf("body corrupted: got %q, want %q", got, c.body)
		}
	}
}

// --- end-to-end through the gin handler ---

// proxyUpstream spins up a stand-in for api.kilo.ai and points the reverse proxy
// at it, so a test can never dial the live gateway. It returns what the upstream
// actually saw, so the path rewrite and the query/header hygiene are asserted
// from the same side as the rewrite itself.
//
// cfg.Env holds variables the test needs beyond KILO_ENDPOINT (typically
// KILO_EGRESS), so a test that overrides egress never reverts to the real one.
func proxyUpstream(t *testing.T, cfg proxyCfg) *upstreamRecording {
	t.Helper()
	rec := &upstreamRecording{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.rawQuery = r.URL.RawQuery
		rec.egress = r.URL.Query().Get("egress")
		rec.ua = r.Header.Get("User-Agent")
		rec.body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", cfg.ContentType)
		for k, v := range cfg.Headers {
			w.Header()[k] = append(w.Header()[k], v...)
		}
		w.WriteHeader(cfg.Status)
		_, _ = w.Write([]byte(cfg.Body))
	}))
	t.Cleanup(srv.Close)
	// Carry the real gateway's /api/gateway path so basePath is non-empty and
	// the assertions match production behaviour rather than a pathless stub.
	env := map[string]string{"KILO_ENDPOINT": srv.URL + "/api/gateway"}
	for _, kv := range cfg.Env {
		env[kv[0]] = kv[1]
	}
	withEnv(t, env)
	resetTransports()
	t.Cleanup(resetTransports)
	return rec
}

type proxyCfg struct {
	Status      int
	ContentType string
	Body        string
	Headers     http.Header
	Env         [][2]string
}

type upstreamRecording struct {
	method, path, rawQuery, egress, ua string
	body                               []byte
}

func TestHandlerPathRemapAndEgressParam(t *testing.T) {
	rec := proxyUpstream(t, proxyCfg{Status: 200, ContentType: "application/json", Body: `{"data":[{"id":"m"}]}`})
	r := freshEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/kilo/v1/chat/completions?foo=bar&egress=auto", nil)
	req.Header.Set("User-Agent", "")
	resp := recorderWithCloseNotify()
	r.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Fatalf("status = %d, body = %q", resp.Code, resp.Body.String())
	}
	if rec.path != "/api/gateway/v1/chat/completions" {
		t.Fatalf("upstream path = %q, want /api/gateway/v1/chat/completions", rec.path)
	}
	if rec.egress != "" {
		t.Fatalf("?egress was forwarded upstream, want it stripped (rawQuery=%q)", rec.rawQuery)
	}
	if rec.rawQuery != "foo=bar" {
		t.Fatalf("upstream rawQuery = %q, want foo=bar", rec.rawQuery)
	}
	if rec.ua != userAgent {
		t.Fatalf("upstream User-Agent = %q, want the default %q", rec.ua, userAgent)
	}
	if got := resp.Body.String(); got != `{"data":[{"id":"m"}]}` {
		t.Fatalf("body not passed through: %q", got)
	}
}

func TestHandlerDedupsGatewayPath(t *testing.T) {
	rec := proxyUpstream(t, proxyCfg{Status: 200, ContentType: "application/json", Body: `{}`})
	r := freshEngine(t)

	// A client that already carries the full upstream path must not double it.
	req := httptest.NewRequest(http.MethodGet, "/api/v2/kilo/api/gateway/v1/models", nil)
	resp := recorderWithCloseNotify()
	r.ServeHTTP(resp, req)
	if rec.path != "/api/gateway/v1/models" {
		t.Fatalf("upstream path = %q, want /api/gateway/v1/models", rec.path)
	}
}

func TestHandlerPreservesClientUA(t *testing.T) {
	rec := proxyUpstream(t, proxyCfg{Status: 200, ContentType: "application/json", Body: `{}`})
	r := freshEngine(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v2/kilo/v1/chat/completions", strings.NewReader(`{}`))
	req.Header.Set("User-Agent", "openclaw/1.0")
	resp := recorderWithCloseNotify()
	r.ServeHTTP(resp, req)
	if rec.ua != "openclaw/1.0" {
		t.Fatalf("client User-Agent not preserved: %q", rec.ua)
	}
	if rec.body == nil || string(rec.body) != `{}` {
		t.Fatalf("request body not forwarded: %q", rec.body)
	}
}

func TestHandlerKeepsOtherQueryParams(t *testing.T) {
	rec := proxyUpstream(t, proxyCfg{Status: 200, ContentType: "application/json", Body: `{}`})
	r := freshEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/kilo/v1/models?include=free&egress=auto", nil)
	resp := recorderWithCloseNotify()
	r.ServeHTTP(resp, req)
	if rec.rawQuery != "include=free" {
		t.Fatalf("upstream rawQuery = %q, want include=free", rec.rawQuery)
	}
}

func TestHandlerGateHeaders(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
		reset      string
		want       string
	}{
		{"rate limit", `{"error":{"message":"Rate limit exceeded"}}`, 429, "", gateProviderLimit},
		{"daily limit", `{"error":{"message":"Daily limit reached"}}`, 429, "2026-10-08T00:00:00Z", gateDailyLimit},
		{"overloaded", `{"error":{"message":"Service temporarily overloaded"}}`, 503, "", gateOverloaded},
		{"forbidden", `{"error":{"message":"Forbidden"}}`, 403, "", gateAuth},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			header := http.Header{}
			if c.reset != "" {
				header.Set("X-RateLimit-Reset", c.reset)
			}
			proxyUpstream(t, proxyCfg{Status: c.status, ContentType: "application/json", Body: c.body, Headers: header})
			r := freshEngine(t)

			req := httptest.NewRequest(http.MethodPost, "/api/v2/kilo/v1/chat/completions", strings.NewReader(`{"model":"x"}`))
			resp := recorderWithCloseNotify()
			r.ServeHTTP(resp, req)

			if resp.Code != c.status {
				t.Fatalf("status = %d, want %d", resp.Code, c.status)
			}
			if got := resp.Header().Get("X-Kilo-Gate"); got != c.want {
				t.Fatalf("X-Kilo-Gate = %q, want %q", got, c.want)
			}
			if resp.Body.String() != c.body {
				t.Fatalf("error body corrupted: %q", resp.Body.String())
			}
		})
	}
}

// The Kilo quirk that most misleads operators: a provider failure relayed
// inside an HTTP 200. The status alone says success, so the gate header is the
// only signal.
func TestHandlerGateOn200WithEmbeddedError(t *testing.T) {
	body := `{"id":"gen-1","object":"chat.completion","error":{"code":503,"message":"Upstream error from Nvidia"}}`
	proxyUpstream(t, proxyCfg{Status: 200, ContentType: "application/json", Body: body})
	r := freshEngine(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v2/kilo/v1/chat/completions", strings.NewReader(`{"model":"x"}`))
	resp := recorderWithCloseNotify()
	r.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Fatalf("status = %d, want 200 (Kilo relays the provider error inside a 200)", resp.Code)
	}
	if got := resp.Header().Get("X-Kilo-Gate"); got != gateOverloaded {
		t.Fatalf("X-Kilo-Gate = %q, want %q", got, gateOverloaded)
	}
	if resp.Body.String() != body {
		t.Fatalf("body must be untouched: %q", resp.Body.String())
	}
}

func TestHandlerNoGateOnCleanCompletion(t *testing.T) {
	body := `{"id":"gen-1","object":"chat.completion","choices":[{"message":{"role":"assistant","content":"OK"}}]}`
	proxyUpstream(t, proxyCfg{Status: 200, ContentType: "application/json", Body: body})
	r := freshEngine(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v2/kilo/v1/chat/completions", strings.NewReader(`{"model":"x"}`))
	resp := recorderWithCloseNotify()
	r.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Fatalf("status = %d", resp.Code)
	}
	if got := resp.Header().Get("X-Kilo-Gate"); got != "" {
		t.Fatalf("clean completion must carry no gate header, got %q", got)
	}
	if resp.Body.String() != body {
		t.Fatalf("body corrupted: %q", resp.Body.String())
	}
}

// An egress block never produces an HTTP response, so it lands in the proxy's
// error path. A bare "bad gateway" cannot be acted on, so the verdict goes
// onto a header and the body is valid OpenAI-shaped JSON.
func TestHandlerTransportFailureGate(t *testing.T) {
	resetTransports()
	t.Cleanup(resetTransports)
	withEnv(t, map[string]string{"KILO_ENDPOINT": "http://127.0.0.1:1"})
	r := freshEngine(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v2/kilo/v1/chat/completions", strings.NewReader(`{"model":"x"}`))
	resp := recorderWithCloseNotify()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.Code)
	}
	if got := resp.Header().Get("X-Kilo-Gate"); got != gateEgressUnreachable {
		t.Fatalf("X-Kilo-Gate = %q, want %q", got, gateEgressUnreachable)
	}
	if got := resp.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	env := assertOpenAIErrorEnvelope(t, resp.Body.Bytes())
	if env.Error.Message == "" {
		t.Fatalf("error message empty: %q", resp.Body.String())
	}
}

func TestHandlerProxyForcedButUnset(t *testing.T) {
	proxyUpstream(t, proxyCfg{Status: 200, ContentType: "application/json", Body: `{}`})
	r := freshEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/kilo/v1/models?egress=proxy", nil)
	resp := recorderWithCloseNotify()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.Code)
	}
	if got := resp.Header().Get("X-Kilo-Gate"); got != gateEgressMisconfigured {
		t.Fatalf("X-Kilo-Gate = %q, want %q", got, gateEgressMisconfigured)
	}
}

// An egress typo must read as a config fault, not as a blocked network. Both
// previously produced the same un-actionable "bad gateway".
func TestHandlerTransportFailureIsJSONAndValid(t *testing.T) {
	proxyUpstream(t, proxyCfg{Status: 200, ContentType: "application/json", Body: `{}`})
	withEnv(t, map[string]string{"KILO_EGRESS": "vless://1.2.3.4:443"})
	resetTransports()
	t.Cleanup(resetTransports)
	r := freshEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/kilo/v1/models?egress=proxy", nil)
	resp := recorderWithCloseNotify()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.Code)
	}
	if got := resp.Header().Get("X-Kilo-Gate"); got != gateEgressMisconfigured {
		t.Fatalf("X-Kilo-Gate = %q, want %q", got, gateEgressMisconfigured)
	}
	env := assertOpenAIErrorEnvelope(t, resp.Body.Bytes())
	if env.Error.Message == "" {
		t.Fatalf("error message empty: %q", resp.Body.String())
	}
	if env.Error.Type != gateEgressMisconfigured {
		t.Fatalf("error type = %q, want %q", env.Error.Type, gateEgressMisconfigured)
	}
}

// assertOpenAIErrorEnvelope parses a proxy-generated error body and fails the
// test if it is not JSON or not in the OpenAI error shape.
func assertOpenAIErrorEnvelope(t *testing.T, raw []byte) errorEnvelope {
	t.Helper()
	var env errorEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("error body is not JSON: %v (%q)", err, raw)
	}
	return env
}

// The whole point of the gate: when the transport cannot reach the upstream,
// the operator gets a verdict instead of a mystery 502.
func TestEgressSummary(t *testing.T) {
	withEnv(t, nil)
	if s := egressSummary(); !strings.Contains(s, "direct") {
		t.Fatalf("unset summary should say direct: %q", s)
	}
	withEnv(t, map[string]string{"KILO_EGRESS": "socks5://172.29.80.1:10808"})
	if s := egressSummary(); !strings.Contains(s, "socks5://172.29.80.1:10808") {
		t.Fatalf("summary = %q", s)
	}
	withEnv(t, map[string]string{"KILO_EGRESS": "vless://1.2.3.4:443"})
	if s := egressSummary(); !strings.Contains(s, "INVALID") {
		t.Fatalf("summary = %q", s)
	}
}

func TestEndpointBaseOverride(t *testing.T) {
	withEnv(t, nil)
	if endpointBase() != defaultEndpoint {
		t.Fatalf("endpointBase() = %q, want %q", endpointBase(), defaultEndpoint)
	}
	withEnv(t, map[string]string{"KILO_ENDPOINT": "https://mirror.example.com/api/gateway/"})
	if got := endpointBase(); got != "https://mirror.example.com/api/gateway" {
		t.Fatalf("endpointBase() = %q, trailing slash not trimmed", got)
	}
}
