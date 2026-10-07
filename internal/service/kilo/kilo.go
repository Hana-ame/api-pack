// Package kilo embeds the Kilo free-pool gateway (OFM lane ②) into api-pack as
// a CORS-friendly reverse proxy. Any OpenAI-compatible client pointed at
// /api/v2/kilo/... can use Kilo's credential-free free tier without knowing
// anything about the gateway.
//
// Unlike the opencode lane (internal/service/opencode), Kilo needs no client
// fingerprinting: the gateway accepts anonymous requests and does not require a
// particular User-Agent (verified 2026-10-07: an empty UA and curl's default UA
// both return 200 on /v1/models).
//
// What it does need is an **overseas egress**:
//
//	api.kilo.ai is fronted by Vercel, which resets the TLS handshake for
//	mainland-China source IPs — right after ClientHello, with no HTTP response
//	at all. From that same CN egress, vercel.com / api.vercel.com / kilo.ai /
//	docs.kilo.ai all answer normally, so this is an api.kilo.ai-scoped SNI/WAF
//	block: not DNS pollution, not a missing route, and not "Kilo is down".
//
// Because the block happens below HTTP, the failure surfaces as a *transport*
// error and never as a status code — which is exactly why it gets misread. This
// package classifies it onto X-Kilo-Gate: EgressBlocked and states the fix.
//
//	KILO_EGRESS=socks5://172.29.80.1:10808   # overseas SOCKS5 (e.g. cloudcone)
//	KILO_EGRESS=http://127.0.0.1:7890        # or an HTTP CONNECT proxy
//	KILO_EGRESS unset                        # direct — only correct when
//	                                         # api-pack itself runs overseas
//
// Go's transport only knows the "socks5" scheme, and for it Go hands the
// *hostname* to the proxy instead of resolving it locally — i.e. socks5h
// semantics, matching `curl --socks5-hostname` (which is the form verified to
// work against api.kilo.ai). A "socks5h://" value is therefore accepted and
// folded onto "socks5", and a bare "host:port" is read as SOCKS5.
package kilo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	// defaultEndpoint is the Kilo gateway root; client paths are appended to it
	// (/v1/models, /v1/chat/completions, ...). Overridable via KILO_ENDPOINT.
	defaultEndpoint = "https://api.kilo.ai/api/gateway"

	// userAgent identifies this proxy to the gateway when the client has not
	// chosen one. Kilo does not require it, but a stable value keeps api-pack
	// traffic identifiable in upstream logs.
	userAgent = "api-pack-kilo-proxy"

	dialTimeout = 15 * time.Second

	// sniffLimit bounds how much of a response body we buffer to classify an
	// upstream failure. The /v1/models listing is ~495 KB, so it stays well
	// clear of this and is streamed straight through.
	sniffLimit = 256 << 10
)

// envGet is an indirection over os.Getenv so tests can redirect the upstream
// endpoint and egress without touching real env.
var envGet = os.Getenv

// ---------------------------------------------------------------------------
// egress selection
// ---------------------------------------------------------------------------

type egressMode string

const (
	// egressAuto uses KILO_EGRESS when set, otherwise connects directly.
	egressAuto egressMode = "auto"
	// egressDirect forces a direct connection, ignoring KILO_EGRESS.
	egressDirect egressMode = "direct"
	// egressProxy forces the KILO_EGRESS proxy (400 if it is not configured).
	egressProxy egressMode = "proxy"
)

// normalizeEgress maps the ?egress= query value onto a mode. Anything
// unrecognised falls back to auto, so a typo degrades to the configured
// default instead of failing the request.
func normalizeEgress(s string) egressMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "direct", "off", "none":
		return egressDirect
	case "proxy", "on", "egress":
		return egressProxy
	default:
		return egressAuto
	}
}

// parseEgressURL normalizes a KILO_EGRESS value into a proxy URL that net/http
// accepts.
//
// socks5h is folded onto socks5 on purpose: Go's transport only recognises
// "socks5" (anything else is rejected as an unsupported proxy scheme), and its
// socks5 path already passes the hostname through to the proxy rather than
// resolving it locally — so "socks5" in Go *is* socks5h. Writing "socks5h" is
// the habit from curl, so accept it rather than making the operator discover
// this the hard way.
func parseEgressURL(spec string) (*url.URL, error) {
	raw := strings.TrimSpace(spec)
	if raw == "" {
		return nil, errors.New("empty egress spec")
	}
	if !strings.Contains(raw, "://") {
		// "172.29.80.1:10808" is the natural shorthand; assume SOCKS5.
		raw = "socks5://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid KILO_EGRESS %q: %w", spec, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "socks5", "socks5h", "socks":
		u.Scheme = "socks5"
	case "http", "https":
		u.Scheme = strings.ToLower(u.Scheme)
	default:
		return nil, fmt.Errorf("unsupported KILO_EGRESS scheme %q (want socks5, socks5h, http or https)", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("invalid KILO_EGRESS %q: missing host", spec)
	}
	// Always rebuild Host through JoinHostPort. url.Parse leaves an unbracketed
	// IPv6 literal as "Host=::1:1080" and Port() then mis-reads the address;
	// JoinHostPort brackets it correctly whether or not a port was written.
	port := u.Port()
	if port == "" {
		port = defaultPortFor(u.Scheme)
	}
	u.Host = net.JoinHostPort(u.Hostname(), port)
	return u, nil
}

func defaultPortFor(scheme string) string {
	switch scheme {
	case "https":
		return "443"
	case "http":
		return "80"
	default:
		return "1080"
	}
}

// transports are cached per egress spec so connections (and in-flight SSE
// streams) are reused across requests; changing KILO_EGRESS swaps in a new one.
var (
	trMu        sync.Mutex
	trDirect    *http.Transport
	trProxy     *http.Transport
	trProxySpec string
)

// transportForEgress resolves the outbound transport for a request.
func transportForEgress(mode egressMode) (*http.Transport, error) {
	spec := strings.TrimSpace(envGet("KILO_EGRESS"))
	if mode == egressAuto {
		if spec == "" {
			mode = egressDirect
		} else {
			mode = egressProxy
		}
	}

	if mode == egressDirect {
		trMu.Lock()
		defer trMu.Unlock()
		if trDirect == nil {
			trDirect = newTransport(nil)
		}
		return trDirect, nil
	}

	if spec == "" {
		return nil, fmt.Errorf("?egress=proxy requires KILO_EGRESS to be set, " +
			"e.g. KILO_EGRESS=socks5://172.29.80.1:10808 (api.kilo.ai resets TLS for mainland-China IPs)")
	}
	pu, err := parseEgressURL(spec)
	if err != nil {
		return nil, err
	}

	trMu.Lock()
	defer trMu.Unlock()
	if trProxy == nil || trProxySpec != pu.String() {
		old := trProxy
		trProxy, trProxySpec = newTransport(pu), pu.String()
		if old != nil {
			old.CloseIdleConnections()
		}
	}
	return trProxy, nil
}

// newTransport builds the outbound transport. proxyURL == nil means direct.
func newTransport(proxyURL *url.URL) *http.Transport {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   dialTimeout,
		ExpectContinueTimeout: time.Second,
		ForceAttemptHTTP2:     true,
		// Never let the transport negotiate or transparently undo content
		// encoding: Kilo streams SSE and some CDNs mislabel Content-Encoding
		// (see generic_proxy.go), so bytes are passed through untouched.
		DisableCompression: true,
	}
	if proxyURL != nil {
		tr.Proxy = http.ProxyURL(proxyURL)
	}
	return tr
}

// egressSummary describes the active outbound path for the startup log, so an
// operator can tell at a glance whether this deployment can reach Kilo at all.
func egressSummary() string {
	spec := strings.TrimSpace(envGet("KILO_EGRESS"))
	if spec == "" {
		return "direct (KILO_EGRESS unset) — only reaches api.kilo.ai if api-pack itself runs outside mainland China"
	}
	u, err := parseEgressURL(spec)
	if err != nil {
		return "INVALID KILO_EGRESS=" + spec + ": " + err.Error()
	}
	return u.String() + " (proxy-side DNS)"
}

// ---------------------------------------------------------------------------
// upstream failure classification
// ---------------------------------------------------------------------------

// Classes surfaced on the X-Kilo-Gate response header.
//
// Kilo's failure modes are easy to misread, and the difference matters because
// only one of them is api-pack's problem:
//
//   - EgressBlocked / EgressUnreachable are local: no HTTP response came back.
//   - DailyLimit / ProviderRateLimit / UpstreamOverloaded come from the
//     *upstream provider* behind Kilo. Each Kilo free model is served by a
//     different provider (Poolside / Nvidia / novita / Stepfun / Liquid /
//     Cohere / Thinking Machines), so one model being limited says nothing
//     about the gateway or about the other 15 models.
//   - Kilo even relays provider failures inside an HTTP 200 body, so the status
//     code alone cannot decide whether a completion succeeded.
const (
	gateEgressBlocked     = "EgressBlocked"
	gateEgressUnreachable = "EgressUnreachable"
	// EgressMisconfigured means the operator asked for a proxy that was not
	// configured (or configured in a form this proxy cannot speak). Kept
	// separate from EgressBlocked: this is a fixable config typo, not a
	// blocked network path, and conflating the two sends the operator chasing
	// a firewall that is not there.
	gateEgressMisconfigured = "EgressMisconfigured"
	gateProviderLimit       = "ProviderRateLimit"
	gateDailyLimit          = "DailyLimit"
	gateOverloaded          = "UpstreamOverloaded"
	gateAuth                = "AuthError"
)

// egressConfigError marks an egress configuration fault (missing or
// unsupported KILO_EGRESS). transportForEgress only ever fails for a bad
// configuration, so the boundary wraps every such error in this type.
type egressConfigError struct{ detail string }

func (e *egressConfigError) Error() string { return e.detail }

// transportErrorHint classifies an error that never produced an HTTP response.
func transportErrorHint(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	// The client went away, or we cancelled: not an upstream fault. Labelling
	// this as an egress problem would send the operator chasing a ghost.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "", ""
	}
	var cfg *egressConfigError
	if errors.As(err, &cfg) {
		return gateEgressMisconfigured, "egress misconfigured: " + cfg.Error()
	}
	msg := strings.ToLower(err.Error())
	switch {
	case isTLSEgressReset(msg):
		return gateEgressBlocked, "egress blocked: api.kilo.ai resets the TLS handshake for this source IP " +
			"(mainland-China egress is cut right after ClientHello, so there is no HTTP status to read). " +
			"Point KILO_EGRESS at an overseas proxy — e.g. KILO_EGRESS=socks5://172.29.80.1:10808 — " +
			"or retry with ?egress=proxy. vercel.com and kilo.ai answer fine from the same egress, " +
			"so this is the api.kilo.ai SNI, not the CDN and not api-pack."
	case isDialFailure(msg):
		return gateEgressUnreachable, "egress unreachable: could not establish a connection to the gateway " +
			"or to the KILO_EGRESS proxy (dial failure / timeout / DNS). Check KILO_EGRESS points at a live proxy."
	}
	return "", ""
}

// isTLSEgressReset matches the ClientHello-then-close signature: an RST, a bare
// EOF, or a handshake that never completes. All three mean "no HTTP response".
func isTLSEgressReset(msg string) bool {
	for _, sig := range []string{
		"connection reset by peer",
		"connection reset",
		"broken pipe",
		"unexpected eof",
		"remote error: tls",
		"tls: handshake failure",
		"tls handshake timeout",
		"eof",
	} {
		if strings.Contains(msg, sig) {
			return true
		}
	}
	return false
}

// isDialFailure matches failures that happen before/at connect time.
func isDialFailure(msg string) bool {
	for _, sig := range []string{
		"connection refused",
		"no route to host",
		"network is unreachable",
		"i/o timeout",
		"no such host",
		"dial tcp",
		"proxyconnect",
	} {
		if strings.Contains(msg, sig) {
			return true
		}
	}
	return false
}

// classifyErrorResponse classifies a non-2xx response. The body is fully
// restored, so the client still sees exactly what the gateway sent.
func classifyErrorResponse(resp *http.Response) (string, string) {
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil && len(body) == 0 {
		return "", ""
	}
	return classifyErrorText(resp.StatusCode, string(body), resp.Header.Get("X-RateLimit-Reset"))
}

// classifyJSONError sniffs a bounded, non-SSE JSON body for a top-level "error"
// object. Kilo relays provider failures inside an HTTP 200 (verified: a nemotron
// free model answered 200 carrying a 503 "Upstream error from Nvidia"), so the
// status code is not enough to tell success from failure.
func classifyJSONError(resp *http.Response) (string, string) {
	// -1 means chunked/streamed (SSE lands here); anything large is a real
	// payload such as the /v1/models listing. Neither is worth buffering.
	if resp.ContentLength < 0 || resp.ContentLength > sniffLimit {
		return "", ""
	}
	// A compressed body cannot be sniffed without decompressing it; leave it be.
	if resp.Header.Get("Content-Encoding") != "" {
		return "", ""
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil || len(body) == 0 {
		return "", ""
	}

	var obj struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(body, &obj) != nil || len(obj.Error) == 0 {
		return "", ""
	}
	if trimmed := bytes.TrimSpace(obj.Error); len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", ""
	}

	// The provider's own status lives inside the error object; prefer it over
	// the outer 200 when deciding how to classify.
	status := resp.StatusCode
	var inner struct {
		Code json.RawMessage `json:"code"`
	}
	if json.Unmarshal(obj.Error, &inner) == nil && len(inner.Code) > 0 {
		var n int
		if json.Unmarshal(inner.Code, &n) == nil && n >= 400 {
			status = n
		}
	}
	return classifyErrorText(status, string(obj.Error), resp.Header.Get("X-RateLimit-Reset"))
}

func classifyErrorText(status int, text, rateLimitReset string) (string, string) {
	low := strings.ToLower(text)
	switch {
	case strings.Contains(low, "daily limit"):
		hint := "gate: this free model's daily quota is exhausted; it resets on the provider's schedule"
		if rateLimitReset != "" {
			hint += " (X-RateLimit-Reset " + rateLimitReset + ")"
		}
		return gateDailyLimit, hint
	case status == http.StatusTooManyRequests ||
		strings.Contains(low, "rate limit") || strings.Contains(low, "too many requests"):
		return gateProviderLimit, "gate: the upstream provider behind this free model is rate-limiting — " +
			"not the Kilo gateway and not api-pack. Each free model has a different provider, " +
			"so pick another model or retry later."
	case status == http.StatusServiceUnavailable ||
		strings.Contains(low, "overloaded") || strings.Contains(low, "temporarily unavailable"):
		return gateOverloaded, "gate: the upstream provider is overloaded right now. " +
			"Kilo relays this inside the body (sometimes even with HTTP 200). Retry later or pick another free model."
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return gateAuth, "gate: the gateway rejected the request as unauthenticated/forbidden. " +
			"The Kilo free pool needs no credential, so check whether a client Authorization header is being forwarded."
	}
	return "", ""
}
