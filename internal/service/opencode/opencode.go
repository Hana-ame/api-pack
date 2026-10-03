// Package opencode embeds an opencode zen free-tier reverse proxy into
// api-pack. Any OpenAI-compatible client pointed at /api/v2/opencode/... gets
// transparently fingerprinted and body-gated so it can use the opencode free
// tier, without the client itself being the opencode CLI.
//
// The proxy strips an optional ?net=v4|v6|auto query parameter to choose the
// outbound address family (affects which free-tier quota line is consumed).
package opencode

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	// UA/CLIENT identify us as a genuine opencode desktop client to the
	// gateway; the free tier fingerprint check keys on these headers.
	opencodeUA         = "opencode/1.18.31 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14"
	opencodeDefaultKey = "public"
	opencodeClientTag  = "desktop"
	opencodeProject    = "global"

	firstChunkGap = 30 * time.Second
	idleGap       = 90 * time.Second

	infToolName = "bash"
	infIdleArg  = `{"command":"echo 继续"}`
	infCallID   = "call_idle"

	maxRequestBody = 12 << 20
)

var (
	fingerprintTools = []string{"bash", "glob", "grep", "read"}
	toolDecoyDesc    = "This tool is currently unavailable and must not be used."

	responsesModels = map[string]bool{
		"muse-spark-1.2-contributor-free": true,
		"muse-spark-1.3-contributor-free": true,
	}
	messagesModels = map[string]bool{
		"union-alpha": true,
	}

	opencodeBase62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	idMintMu     sync.Mutex
	idMintLastTS uint64
	idMintSeq    uint64
)

// ---------------------------------------------------------------------------
// id minting — gateway-shaped ses_*/msg_* ids
// ---------------------------------------------------------------------------

func hex48(v uint64) string {
	v &= 0xFFFFFFFFFFFF
	var b [6]byte
	binary.BigEndian.PutUint32(b[2:], uint32(v))
	b[0] = byte(v >> 40)
	b[1] = byte(v >> 32)
	return hex.EncodeToString(b[:])
}

func base62FromBytes(b []byte) string {
	out := make([]byte, len(b))
	for i, x := range b {
		out[i] = opencodeBase62[x%62]
	}
	return string(out)
}

func mintIDPart(invert bool) string {
	idMintMu.Lock()
	now := uint64(time.Now().UnixMilli())
	if now == idMintLastTS {
		idMintSeq++
	} else {
		idMintLastTS = now
		idMintSeq = 0
	}
	v := now*0x1000 + idMintSeq
	idMintMu.Unlock()

	if invert {
		v = ^v
	}
	h := hex48(v)

	raw := make([]byte, 14)
	if _, err := rand.Read(raw); err != nil {
		for i := range raw {
			raw[i] = byte(v>>uint(8*(i%6))) ^ byte(0x5a+i)
		}
	}
	return h + base62FromBytes(raw)
}

func mintSessionID() string { return "ses_" + mintIDPart(true) }
func mintRequestID() string { return "msg_" + mintIDPart(false) }

// ---------------------------------------------------------------------------
// header fingerprint
// ---------------------------------------------------------------------------

func applyFingerprintHeaders(req *http.Request) {
	if ua := req.Header.Get("User-Agent"); !strings.HasPrefix(ua, "opencode") {
		req.Header.Set("User-Agent", opencodeUA)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	setIfEmpty := func(k, v string) {
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}
	setIfEmpty("X-Opencode-Client", opencodeClientTag)
	setIfEmpty("X-Opencode-Project", opencodeProject)
	setIfEmpty("X-Session-Id", "ses_proxy")
	setIfEmpty("X-Session-Affinity", "ses_proxy")
	setIfEmpty("X-Opencode-Session", mintSessionID())
	setIfEmpty("X-Opencode-Request", mintRequestID())
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "text/event-stream")
	}
}

// ---------------------------------------------------------------------------
// request-body gate & hygiene
// ---------------------------------------------------------------------------

func rewriteOpencodeBody(r *http.Request) ([]byte, string, bool, error) {
	body, err := readRequestBody(r)
	if err != nil {
		return nil, "", false, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, "", false, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, "", false, fmt.Errorf("invalid JSON body: %w", err)
	}

	model, _ := payload["model"].(string)
	sanitizeRoles(payload)

	if v, ok := payload["max_tokens"]; ok {
		if n, isInt := toInt(v); isInt {
			if n < 1 {
				n = 1
			}
			if n > 131072 {
				n = 131072
			}
			payload["max_tokens"] = n
		}
	}
	if v, ok := payload["top_p"]; ok {
		if f, isF := toFloat(v); isF {
			if f <= 0 {
				f = 0.1
			}
			if f > 1 {
				f = 1
			}
			payload["top_p"] = f
		}
	}
	if v, ok := payload["temperature"]; ok {
		if f, isF := toFloat(v); isF {
			if f < 0 {
				f = 0
			}
			if f > 2 {
				f = 2
			}
			payload["temperature"] = f
		}
	}

	payload["reasoning_effort"] = "max"
	payload["stream"] = true

	rawTools, _ := payload["tools"].([]any)
	_, _ = ensureToolQuartet(payload, rawTools, false)

	out, err := json.Marshal(payload)
	if err != nil {
		return nil, "", false, fmt.Errorf("re-encode body: %w", err)
	}
	return out, model, len(rawTools) > 0, nil
}

func ensureToolQuartet(payload map[string]any, tools []any, flat bool) ([]any, bool) {
	seen := make(map[string]bool, len(tools))
	hadClientTools := false
	for _, t := range tools {
		m, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if n := toolNameOf(m); n != "" {
			seen[n] = true
			hadClientTools = true
		}
	}
	for _, want := range fingerprintTools {
		if seen[want] {
			continue
		}
		tools = append(tools, decoyTool(want, flat))
	}
	payload["tools"] = tools
	if hadClientTools || flat {
		payload["tool_choice"] = "auto"
	} else {
		payload["tool_choice"] = "none"
	}
	return tools, hadClientTools
}

func decoyTool(name string, flat bool) map[string]any {
	body := map[string]any{
		"name":        name,
		"description": toolDecoyDesc,
		"parameters":  map[string]any{"type": "object", "properties": map[string]any{}},
	}
	if flat {
		body["type"] = "function"
		return body
	}
	return map[string]any{"type": "function", "function": body}
}

func toolNameOf(m map[string]any) string {
	if n, ok := m["name"].(string); ok && n != "" {
		return n
	}
	if fn, ok := m["function"].(map[string]any); ok {
		if n, ok := fn["name"].(string); ok && n != "" {
			return n
		}
	}
	return ""
}

func sanitizeRoles(payload map[string]any) {
	msgs, ok := payload["messages"].([]any)
	if !ok {
		return
	}
	for _, m := range msgs {
		mm, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if mm["role"] == "developer" {
			mm["role"] = "system"
		}
	}
}

func readRequestBody(r *http.Request) ([]byte, error) {
	if r == nil || r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		if f, err := n.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}

// ---------------------------------------------------------------------------
// endpoint routing
// ---------------------------------------------------------------------------

func trimModelID(model string) string {
	if i := strings.Index(model, "/"); i >= 0 {
		model = model[i+1:]
	}
	if i := strings.Index(model, " ("); i >= 0 {
		model = model[:i]
	}
	return model
}

func endpointFor(model string) string {
	model = trimModelID(model)
	if responsesModels[model] {
		return "/responses"
	}
	if messagesModels[model] {
		return "/messages"
	}
	return "/chat/completions"
}

func reRouteModelEndpoint(req *http.Request, model string) string {
	if !strings.HasSuffix(req.URL.Path, "/chat/completions") {
		return ""
	}
	ep := endpointFor(trimModelID(model))
	if ep == "/chat/completions" {
		return ""
	}
	return ep
}

// ---------------------------------------------------------------------------
// SSE stream pump: idle detection + keepalive injection
// ---------------------------------------------------------------------------

type pumpChunk struct {
	data []byte
	eof  bool
	err  error
}

type ssePumpBody struct {
	upstream io.ReadCloser
	out      chan pumpChunk
	done     chan struct{}
	model    string

	pending []byte
}

func newSSEPumpBody(upstream io.ReadCloser, model string) *ssePumpBody {
	b := &ssePumpBody{
		upstream: upstream,
		out:      make(chan pumpChunk, 64),
		done:     make(chan struct{}),
		model:    model,
	}
	go b.pump()
	return b
}

func (b *ssePumpBody) Read(p []byte) (int, error) {
	for {
		if len(b.pending) > 0 {
			n := copy(p, b.pending)
			b.pending = b.pending[n:]
			return n, nil
		}
		select {
		case c, ok := <-b.out:
			if !ok {
				return 0, io.EOF
			}
			if c.err != nil {
				return 0, c.err
			}
			if c.eof {
				return 0, io.EOF
			}
			if len(c.data) <= len(p) {
				return copy(p, c.data), nil
			}
			n := copy(p, c.data)
			b.pending = c.data[n:]
			return n, nil
		case <-b.done:
			return 0, io.EOF
		}
	}
}

func (b *ssePumpBody) Close() error {
	select {
	case <-b.done:
	default:
		close(b.done)
	}
	return b.upstream.Close()
}

type rawChunk struct {
	data []byte
	eof  bool
	err  error
}

func (b *ssePumpBody) pump() {
	raw := make(chan rawChunk, 32)
	go func() {
		buf := make([]byte, 16384)
		for {
			n, err := b.upstream.Read(buf)
			if n > 0 {
				cp := make([]byte, n)
				copy(cp, buf[:n])
				select {
				case raw <- rawChunk{data: cp}:
				case <-b.done:
					return
				}
			}
			if err != nil {
				m := rawChunk{}
				if errors.Is(err, io.EOF) {
					m.eof = true
				} else {
					m.err = err
				}
				select {
				case raw <- m:
				case <-b.done:
					return
				}
				close(raw)
				return
			}
		}
	}()

	var acc []byte
	finished := false
	doneSent := false
	injected := false
	sawTool := false
	lastReal := time.Now()
	started := time.Now()

	send := func(data []byte) bool {
		if len(data) == 0 {
			return true
		}
		select {
		case b.out <- pumpChunk{data: data}:
			return true
		case <-b.done:
			return false
		}
	}
	sendEOF := func() {
		select {
		case b.out <- pumpChunk{eof: true}:
		case <-b.done:
		}
	}

	preEv, preRest, err := b.preRead(raw)
	if err != nil {
		log.Printf("opencode: pre-read failed: %v", err)
		send([]byte("data: [DONE]\n\n"))
		sendEOF()
		return
	}
	acc = preRest
	if !send(append(append([]byte{}, preEv...), '\n', '\n')) {
		return
	}
	if hasContent(preEv) {
		lastReal = time.Now()
	}
	if finishReason(preEv) != nil {
		finished = true
	}
	if bytes.Contains(preEv, []byte("data: [DONE]")) {
		finished = true
		doneSent = true
	}

	timer := time.NewTimer(idleGap)
	defer timer.Stop()

loop:
	for {
		for {
			ev, rest, ok := nextSSEEvent(acc)
			if !ok {
				acc = rest
				break
			}
			acc = rest
			if !isRealSSE(ev) {
				continue
			}
			if hasToolCall(ev) {
				sawTool = true
			}
			if hasError(ev) {
				if !finished && !injected {
					if !send(idleInject(b.model)) {
						break loop
					}
					injected = true
					doneSent = true
					break loop
				}
				continue
			}
			if finishReason(ev) != nil {
				finished = true
			}
			if bytes.Contains(ev, []byte("data: [DONE]")) {
				finished = true
				doneSent = true
			}
			if hasContent(ev) {
				lastReal = time.Now()
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(idleGap)
			}
			if !send(append(append([]byte{}, ev...), '\n', '\n')) {
				break loop
			}
		}

		select {
		case m, ok := <-raw:
			if !ok || m.eof {
				if m.err != nil && !finished {
					log.Printf("opencode: upstream error mid-stream: %v", m.err)
				}
				if !finished && !doneSent && !injected {
					if !send(idleInject(b.model)) {
						break loop
					}
					injected = true
					doneSent = true
				}
				if !doneSent {
					send([]byte("data: [DONE]\n\n"))
					doneSent = true
				}
				sendEOF()
				break loop
			}
			if m.err != nil {
				if !finished && !injected {
					if !send(idleInject(b.model)) {
						break loop
					}
					injected = true
					doneSent = true
				}
				if !doneSent {
					send([]byte("data: [DONE]\n\n"))
					doneSent = true
				}
				sendEOF()
				break loop
			}
			acc = append(acc, m.data...)
			if time.Since(lastReal) > idleGap {
				if !finished && !injected {
					if !send(idleInject(b.model)) {
						break loop
					}
					injected = true
					doneSent = true
				}
				if !doneSent {
					send([]byte("data: [DONE]\n\n"))
				}
				sendEOF()
				break loop
			}
			timer.Reset(idleGap - time.Since(lastReal))
		case <-timer.C:
			if !finished && !injected {
				if !send(idleInject(b.model)) {
					break loop
				}
				injected = true
				doneSent = true
			}
			if !doneSent {
				send([]byte("data: [DONE]\n\n"))
			}
			sendEOF()
			break loop
		case <-b.done:
			break loop
		}
	}

	if injected {
		log.Printf("opencode: injected keepalive tool call (model=%s)", b.model)
	}
	log.Printf("opencode: stream done in %.1fs saw_tool=%v injected=%v",
		time.Since(started).Seconds(), sawTool, injected)
}

func (b *ssePumpBody) preRead(raw <-chan rawChunk) ([]byte, []byte, error) {
	deadline := time.Now().Add(firstChunkGap)
	var buf []byte
	for {
		remain := time.Until(deadline)
		if remain <= 0 {
			return nil, nil, fmt.Errorf("upstream sent no real SSE event within %s", firstChunkGap)
		}
		timer := time.NewTimer(remain)
		select {
		case m, ok := <-raw:
			timer.Stop()
			if !ok || m.eof {
				return nil, nil, fmt.Errorf("upstream closed before first SSE event")
			}
			if m.err != nil {
				return nil, nil, m.err
			}
			buf = append(buf, m.data...)
			if ev, rest, complete := nextSSEEvent(buf); complete && isRealSSE(ev) {
				return ev, rest, nil
			}
		case <-timer.C:
			return nil, nil, fmt.Errorf("upstream stalled before first SSE event")
		case <-b.done:
			return nil, nil, fmt.Errorf("client disconnected")
		}
	}
}

func idleInject(model string) []byte {
	toolEvt := fmt.Sprintf(
		`{"id":"idle","object":"chat.completion.chunk","created":0,"model":%s,"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":%q,"type":"function","function":{"name":%q,"arguments":%s}}]},"finish_reason":null}]}`,
		jsonString(model), infCallID, infToolName, jsonString(infIdleArg))
	finishEvt := fmt.Sprintf(
		`{"id":"idle","object":"chat.completion.chunk","created":0,"model":%s,"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		jsonString(model))
	return []byte("data: " + toolEvt + "\n\ndata: " + finishEvt + "\n\ndata: [DONE]\n\n")
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// ---------------------------------------------------------------------------
// SSE parsing helpers
// ---------------------------------------------------------------------------

func isRealSSE(buf []byte) bool {
	for _, line := range bytes.Split(buf, []byte("\n")) {
		t := bytes.TrimSpace(line)
		if len(t) == 0 || bytes.HasPrefix(t, []byte(":")) {
			continue
		}
		if bytes.HasPrefix(t, []byte("data:")) || bytes.HasPrefix(t, []byte("event:")) {
			return true
		}
	}
	return false
}

func nextSSEEvent(buf []byte) ([]byte, []byte, bool) {
	for _, sep := range [][]byte{[]byte("\r\n\r\n"), []byte("\n\n")} {
		if idx := bytes.Index(buf, sep); idx >= 0 {
			return buf[:idx], buf[idx+len(sep):], true
		}
	}
	return nil, buf, false
}

func hasToolCall(ev []byte) bool {
	return bytes.Contains(ev, []byte(`"tool_calls"`))
}

func hasContent(ev []byte) bool {
	if bytes.Contains(ev, []byte("data: [DONE]")) {
		return true
	}
	for _, needle := range []string{`"content"`, `"tool_calls"`, `"reasoning"`, `"finish_reason"`} {
		if bytes.Contains(ev, []byte(needle)) {
			return true
		}
	}
	return false
}

func hasError(ev []byte) bool {
	line := sseDataLine(ev)
	if len(line) == 0 || line[0] != '{' {
		return false
	}
	var obj map[string]any
	if err := json.Unmarshal(line, &obj); err != nil {
		return false
	}
	_, ok := obj["error"]
	return ok
}

func finishReason(ev []byte) *string {
	var p struct {
		Choices []struct {
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
	}
	line := sseDataLine(ev)
	if len(line) == 0 || line[0] != '{' {
		return nil
	}
	if err := json.Unmarshal(line, &p); err != nil {
		return nil
	}
	for _, c := range p.Choices {
		if c.FinishReason != nil && *c.FinishReason != "" {
			return c.FinishReason
		}
	}
	return nil
}

func sseDataLine(ev []byte) []byte {
	idx := bytes.Index(ev, []byte("data:"))
	if idx < 0 {
		return nil
	}
	return bytes.TrimSpace(ev[idx+len("data:"):])
}

// ---------------------------------------------------------------------------
// gate error classification
// ---------------------------------------------------------------------------

func gateClassOf(body []byte) string {
	s := string(body)
	switch {
	case strings.Contains(s, "FreeTierError"):
		return "FreeTierError"
	case strings.Contains(s, "FreeUsageLimitError"):
		return "FreeUsageLimitError"
	case strings.Contains(s, "RegionError"):
		return "RegionError"
	case strings.Contains(s, "Model is unavailable"):
		return "ModelUnavailable"
	case strings.Contains(s, "Endpoint is unavailable"):
		return "EndpointUnavailable"
	case strings.Contains(s, "AuthError") || strings.Contains(s, "Missing API key"):
		return "AuthError"
	}
	return ""
}

func classifyGateError(resp *http.Response) (string, string) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", ""
	}
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	class := gateClassOf(body)
	if class == "" {
		return "", ""
	}
	return class, gateHintClassifies([]byte(class))
}

func gateHintClassifies(class []byte) string {
	switch string(class) {
	case "FreeTierError":
		return "gate: request was rejected by the opencode free-tier shape check. The proxy forces stream:true and the bash/glob/grep/read quartet, so a FreeTierError means this egress IP is blocked or the gateway rules changed."
	case "FreeUsageLimitError":
		return "gate: free-tier daily quota for this session/egress IP is exhausted. Try a different egress, or wait for the UTC quota reset."
	case "RegionError":
		return "gate: model region-locked for this egress IP. Try a different egress/net, or another model."
	case "ModelUnavailable":
		return "gate: the upstream model is unavailable right now. Try again later or pick another free model."
	case "EndpointUnavailable":
		return "gate: the upstream endpoint is unavailable right now. Try again later."
	case "AuthError":
		return "gate: the upstream rejected authentication. Check the proxy Authkey config."
	}
	return ""
}

// ---------------------------------------------------------------------------
// transport selection by ?net= param
// ---------------------------------------------------------------------------

// transports caches one http.Transport per address family. LocalAddr binds
// the outbound source address, which is what splits the free-tier quota lines.
type transportSet struct {
	v4, v6, auto *http.Transport
}

var (
	tsMu   sync.Mutex
	tsSets = map[string]*transportSet{}
)

func transportFor(netMode string) *http.Transport {
	tsMu.Lock()
	defer tsMu.Unlock()
	ts := tsSets[netMode]
	if ts == nil {
		ts = buildTransportSet()
		tsSets[netMode] = ts
	}

	switch netMode {
	case "v4":
		if ts.v4 != nil {
			return ts.v4
		}
	case "v6":
		if ts.v6 != nil {
			return ts.v6
		}
	}
	return ts.auto
}

// buildTransportSet constructs per-family transports from env:
//
//	OPENCODE_NET_V4  (optional) source IPv4 for the v4 line
//	OPENCODE_NET_V6  (optional) source IPv6 for the v6 line
func buildTransportSet() *transportSet {
	base := func(local net.IP, network string) *http.Transport {
		dialer := &net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		if local != nil {
			dialer.LocalAddr = &net.TCPAddr{IP: local}
		}
		return &http.Transport{
			DialContext: func(ctx context.Context, netw, addr string) (net.Conn, error) {
				if network != "" && netw == "tcp" {
					netw = network
				}
				return dialer.DialContext(ctx, netw, addr)
			},
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 15 * time.Second,
			ForceAttemptHTTP2:   true,
			DisableCompression:  true,
		}
	}

	ts := &transportSet{
		v4:   base(parseIPEnv("OPENCODE_NET_V4"), "tcp4"),
		v6:   base(parseIPEnv("OPENCODE_NET_V6"), "tcp6"),
		auto: base(nil, ""),
	}
	return ts
}

func parseIPEnv(key string) net.IP {
	if s := strings.TrimSpace(envGet(key)); s != "" {
		if ip := net.ParseIP(s); ip != nil {
			return ip
		}
	}
	return nil
}

// envGet is an indirection over os.Getenv so tests can redirect the upstream
// endpoint per-case without touching real env.
var envGet = os.Getenv
