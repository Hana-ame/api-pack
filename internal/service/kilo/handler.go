package kilo

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// Routes registers the Kilo free-pool reverse proxy under the gin engine.
// Path prefix is /api/v2/kilo: any suffix (e.g. /v1/chat/completions,
// /v1/models) is forwarded to the Kilo gateway root. Clients that carry the
// upstream path verbatim (/api/gateway/...) are mapped idempotently — same
// de-dup trick as opencode's /v1 handling.
//
// An optional ?egress= query parameter overrides the outbound egress:
//
//	?egress=proxy   force the KILO_EGRESS proxy (502 + message if unset/broken)
//	?egress=direct  force a direct connection, ignoring KILO_EGRESS
//	?egress=auto    KILO_EGRESS when set, direct otherwise (default)
//
// Kilo-specific failures are labelled on the X-Kilo-Gate response header
// (EgressBlocked / ProviderRateLimit / DailyLimit / UpstreamOverloaded /
// AuthError) so a client can tell "this deployment's egress is wrong" from
// "that free model's provider is limiting" — both arrive as 5xx/429 otherwise.
func Routes(r *gin.Engine) {
	// The egress summary is printed once at mount time because a CN egress is
	// silently useless against Kilo: every request would die below HTTP. The
	// operator should see which outbound path the binary resolved *before*
	// debugging individual 502s.
	log.Printf("kilo: reverse proxy mounted at /api/v2/kilo -> %s; egress: %s",
		endpointBase(), egressSummary())

	rg := r.Group("/api/v2/kilo")
	rg.Any("/*any", ReverseProxyHandler())
}

// endpointBase returns the upstream Kilo gateway root, overridable via env
// KILO_ENDPOINT (useful for private upstreams or testing).
func endpointBase() string {
	if s := strings.TrimSpace(envGet("KILO_ENDPOINT")); s != "" {
		return strings.TrimRight(s, "/")
	}
	return defaultEndpoint
}

// ReverseProxyHandler builds the Kilo reverse proxy handler. One proxy is
// constructed per request because the egress transport is picked from the
// ?egress= query parameter, and httputil.ReverseProxy uses its own Transport
// field and never consults req.Transport.
func ReverseProxyHandler() gin.HandlerFunc {
	target, err := url.Parse(endpointBase())
	if err != nil {
		log.Printf("kilo: invalid KILO_ENDPOINT %q: %v; falling back to %s",
			envGet("KILO_ENDPOINT"), err, defaultEndpoint)
		target, _ = url.Parse(defaultEndpoint)
	}
	basePath := strings.TrimRight(target.Path, "/")

	build := func(mode egressMode) *httputil.ReverseProxy {
		return &httputil.ReverseProxy{
			FlushInterval: -1, // immediate SSE flushing
			Transport:     transportFor(mode),
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.Host = target.Host

				// Map /api/v2/kilo/<client-path> onto <base><client-path>.
				up := req.URL.Path
				if i := strings.LastIndex(up, "/kilo"); i >= 0 {
					up = up[i+len("/kilo"):]
				}
				if !strings.HasPrefix(up, "/") {
					up = "/" + up
				}
				// Idempotent mapping: a client pointing the official full path
				// (/api/gateway/v1/...) at us must not double the base.
				bp := basePath
				if strings.HasSuffix(bp, "/api/gateway") &&
					(up == "/api/gateway" || strings.HasPrefix(up, "/api/gateway/")) {
					bp = strings.TrimSuffix(bp, "/api/gateway")
				}
				req.URL.Path = bp + up

				// Drop the egress param we consume; keep all others.
				if q := req.URL.Query(); q.Get("egress") != "" {
					q.Del("egress")
					req.URL.RawQuery = q.Encode()
				}

				// Kilo requires no credential and no particular UA (verified
				// 2026-10-07: empty UA and curl's default both return 200 on
				// /v1/models). The default below only keeps api-pack traffic
				// identifiable in gateway logs; a client UA is never touched.
				if req.Header.Get("User-Agent") == "" {
					req.Header.Set("User-Agent", userAgent)
				}
			},
			ModifyResponse: func(resp *http.Response) error {
				switch {
				case resp.StatusCode >= 400:
					if class, hint := classifyErrorResponse(resp); class != "" {
						resp.Header.Set("X-Kilo-Gate", class)
						log.Printf("kilo: upstream %d %s: %s", resp.StatusCode, class, hint)
					}
				case resp.StatusCode == http.StatusOK:
					// Kilo relays provider failures inside HTTP 200 bodies
					// (verified: a 503 "Upstream error from Nvidia" carried by
					// a 200), so 200 alone does not mean the model answered.
					if class, hint := classifyJSONError(resp); class != "" {
						resp.Header.Set("X-Kilo-Gate", class)
						log.Printf("kilo: upstream %d carrying %s: %s", resp.StatusCode, class, hint)
					}
				}
				return nil
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				// No HTTP response ever came back (TLS RST, refused proxy, ...).
				// The default ReverseProxy error is a bare 502 "bad gateway",
				// which cannot be acted on — replace it with the classified
				// verdict so the client/operator sees *why* it failed.
				class, hint := transportErrorHint(err)
				if class != "" {
					w.Header().Set("X-Kilo-Gate", class)
					log.Printf("kilo: upstream request failed (%s): %s", class, hint)
				} else {
					log.Printf("kilo: upstream request failed: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				// Marshal rather than fmt %q: %q falls back to backtick-quoted
				// Go strings when the value contains a newline, which is invalid
				// JSON — and transport errors do contain newlines.
				var env errorEnvelope
				env.Error.Message = err.Error()
				env.Error.Type = class
				body, _ := json.Marshal(env)
				_, _ = w.Write(body)
			},
		}
	}

	return func(c *gin.Context) {
		mode := normalizeEgress(c.Query("egress"))
		build(mode).ServeHTTP(c.Writer, c.Request)
	}
}

// transportFor resolves the request transport, converting a configuration
// error into a RoundTripper that fails every request with that error
// verbatim. Deliberately *not* falling back to direct: a broken KILO_EGRESS is
// a deployment bug, and on a CN host "try direct" produces a second, unrelated
// failure (EgressBlocked) that hides the real cause.
func transportFor(mode egressMode) http.RoundTripper {
	tr, err := transportForEgress(mode)
	if err != nil {
		// transportForEgress fails only for a bad configuration, so label it as
		// such rather than letting it look like a blocked network path.
		return errRoundTripper{&egressConfigError{detail: err.Error()}}
	}
	return tr
}

// errorEnvelope is the OpenAI-shaped body used on the transport-error path
// (a failure that never produced an upstream HTTP response). Kilo's own
// failures already arrive in this shape, so a client that parses OpenAI errors
// needs no special case for api-pack-generated ones.
type errorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type,omitempty"`
	} `json:"error"`
}

type errRoundTripper struct{ err error }

func (t errRoundTripper) RoundTrip(*http.Request) (*http.Response, error) { return nil, t.err }
