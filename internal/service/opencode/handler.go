package opencode

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Routes registers the opencode proxy under the gin engine. Path prefix is
// /api/v2/opencode: any suffix (e.g. /v1/chat/completions, /v1/models,
// /responses) is forwarded to the opencode gateway. An optional ?net= query
// parameter picks the outbound address family: v4 | v6 | auto (default).
func Routes(r *gin.Engine) {
	rg := r.Group("/api/v2/opencode")
	rg.Any("/*any", ReverseProxyHandler())
}

// endpointBase returns the upstream opencode gateway, overridable via env
// OPENCODE_ENDPOINT (useful for private upstreams / testing).
func endpointBase() string {
	if s := strings.TrimSpace(envGet("OPENCODE_ENDPOINT")); s != "" {
		return strings.TrimRight(s, "/")
	}
	return "https://opencode.ai/zen/v1"
}

// ReverseProxyHandler builds the opencode reverse proxy handler. One proxy is
// constructed per request with the address-family transport chosen from the
// ?net= query param, because httputil.ReverseProxy uses its own Transport
// field and never consults req.Transport.
func ReverseProxyHandler() gin.HandlerFunc {
	target, err := url.Parse(endpointBase())
	if err != nil {
		log.Printf("opencode: invalid OPENCODE_ENDPOINT: %v", err)
		target, _ = url.Parse("https://opencode.ai/zen/v1")
	}
	basePath := strings.TrimRight(target.Path, "/")

	build := func(netMode string) *httputil.ReverseProxy {
		proxy := &httputil.ReverseProxy{
			Transport:     transportFor(netMode),
			FlushInterval: -1, // immediate SSE flushing
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.Host = target.Host

				// Map /api/v2/opencode/<client-path> onto <base><client-path>.
				up := req.URL.Path
				if i := strings.LastIndex(up, "/opencode"); i >= 0 {
					up = up[i+len("/opencode"):]
				}
				if !strings.HasPrefix(up, "/") {
					up = "/" + up
				}
				// Avoid doubling /v1 when the base already ends with it.
				bp := basePath
				if strings.HasSuffix(bp, "/v1") && strings.HasPrefix(up, "/v1") {
					bp = strings.TrimSuffix(bp, "/v1")
				}
				req.URL.Path = bp + up

				// Drop the net param we consume; keep all others.
				if q := req.URL.Query(); q.Get("net") != "" {
					q.Del("net")
					req.URL.RawQuery = q.Encode()
				}

				// opencode lane: fingerprint + request-body gate.
				applyFingerprintHeaders(req)
				newBody, model, _, err := rewriteOpencodeBody(req)
				if err != nil {
					log.Printf("opencode: body rewrite failed, forwarding as-is: %v", err)
					return
				}
				if newBody != nil {
					req.Body = io.NopCloser(strings.NewReader(string(newBody)))
					req.ContentLength = int64(len(newBody))
					req.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
					req.GetBody = func() (io.ReadCloser, error) {
						return io.NopCloser(strings.NewReader(string(newBody))), nil
					}
					// Free models that only live on /responses or /messages.
					if np := reRouteModelEndpoint(req, model); np != "" {
						req.URL.Path = strings.TrimSuffix(req.URL.Path, "/chat/completions") + np
					}
				}
			},
			ModifyResponse: func(resp *http.Response) error {
				switch {
				case resp.StatusCode == http.StatusOK:
					if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "text/event-stream") {
						if resp.Body != nil {
							model := ""
							if gb := resp.Request.GetBody; gb != nil {
								if bc, err := gb(); err == nil {
									body, _ := io.ReadAll(bc)
									var p map[string]any
									if json.Unmarshal(body, &p) == nil {
										model, _ = p["model"].(string)
									}
								}
							}
							resp.Body = newSSEPumpBody(resp.Body, model)
						}
					}
				case resp.StatusCode >= 400:
					if class, hint := classifyGateError(resp); class != "" {
						resp.Header.Set("X-Opencode-Gate", class)
						log.Printf("opencode: upstream %d %s: %s", resp.StatusCode, class, hint)
					}
				}
				return nil
			},
		}
		return proxy
	}

	return func(c *gin.Context) {
		netMode := strings.ToLower(strings.TrimSpace(c.Query("net")))
		if netMode != "v4" && netMode != "v6" {
			netMode = "auto"
		}
		build(netMode).ServeHTTP(c.Writer, c.Request)
	}
}
