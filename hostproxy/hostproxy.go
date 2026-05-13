// Package hostproxy provides a configurable reverse proxy that forwards all requests
// to a specified target host, with optional Referer/Origin header control.
//
// Usage:
//
//	// Simple: pass all headers through
//	err := hostproxy.Run(hostproxy.Config{
//	    ListenAddr:   "127.26.5.13:8080",
//	    TargetHost:   "ehgt.org",
//	    TargetScheme: "https",
//	})
//
//	// With header overrides
//	referer := "https://e-hentai.org/"
//	origin := "" // remove Origin
//	err := hostproxy.Run(hostproxy.Config{
//	    ListenAddr:   "127.26.5.13:8080",
//	    TargetHost:   "ehgt.org",
//	    TargetScheme: "https",
//	    Referer:      &referer,
//	    Origin:       &origin,
//	})
package hostproxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Config holds the configuration for a host-based reverse proxy.
type Config struct {
	// ListenAddr is the address the proxy server listens on, e.g. "127.26.5.13:8080".
	ListenAddr string

	// TargetHost is the upstream host to forward requests to, e.g. "ehgt.org".
	TargetHost string

	// TargetScheme is the scheme used for upstream requests. Defaults to "https".
	TargetScheme string

	// Referer controls the Referer header sent to the upstream.
	//   nil       = pass through the original Referer from the client
	//   &""       = strip the Referer header entirely
	//   &"<val>"  = override Referer with the given value
	Referer *string

	// Origin controls the Origin header sent to the upstream.
	//   nil       = pass through the original Origin from the client
	//   &""       = strip the Origin header entirely
	//   &"<val>"  = override Origin with the given value
	Origin *string
}

// Proxy is a reverse proxy instance that forwards all requests to a configured target host.
type Proxy struct {
	config Config
}

// New creates a new Proxy with the given configuration.
func New(config Config) *Proxy {
	if config.TargetScheme == "" {
		config.TargetScheme = "https"
	}
	return &Proxy{config: config}
}

// proxyClient forces IPv4, blocks redirects, and has sensible timeouts.
var proxyClient = &http.Client{
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 90 * time.Second,
			}).DialContext(ctx, "tcp4", addr)
		},
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		TLSHandshakeTimeout: 15 * time.Second,
		MaxIdleConns:        64,
		IdleConnTimeout:     10 * time.Second,
	},
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// Run starts the proxy server. It blocks until the server stops.
func (p *Proxy) Run() error {
	if p.config.ListenAddr == "" {
		return fmt.Errorf("hostproxy: listen address is empty")
	}
	if p.config.TargetHost == "" {
		return fmt.Errorf("hostproxy: target host is empty")
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Any("/*any", p.handler())

	return r.Run(p.config.ListenAddr)
}

func (p *Proxy) handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		targetURL := fmt.Sprintf("%s://%s%s",
			p.config.TargetScheme,
			p.config.TargetHost,
			c.Request.URL.String(),
		)

		req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, targetURL, c.Request.Body)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// Pass through all client headers except hop-by-hop
		for k, vv := range c.Request.Header {
			if isHopByHop(k) {
				continue
			}
			for _, v := range vv {
				req.Header.Add(k, v)
			}
		}

		req.Host = p.config.TargetHost

		// Apply Referer override
		if p.config.Referer != nil {
			if *p.config.Referer == "" {
				req.Header.Del("Referer")
			} else {
				req.Header.Set("Referer", *p.config.Referer)
			}
		}

		// Apply Origin override
		if p.config.Origin != nil {
			if *p.config.Origin == "" {
				req.Header.Del("Origin")
			} else {
				req.Header.Set("Origin", *p.config.Origin)
			}
		}

		resp, err := proxyClient.Do(req)
		if err != nil {
			c.String(http.StatusBadGateway, "Proxy Error: %v", err)
			return
		}
		defer resp.Body.Close()

		// Pass through all upstream response headers except hop-by-hop
		for k, vv := range resp.Header {
			if isHopByHop(k) {
				continue
			}
			for _, v := range vv {
				c.Writer.Header().Add(k, v)
			}
		}

		c.DataFromReader(resp.StatusCode, resp.ContentLength,
			resp.Header.Get("Content-Type"), resp.Body, nil)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH, HEAD")
		c.Header("Access-Control-Allow-Headers", c.GetHeader("Access-Control-Request-Headers"))
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Expose-Headers", "*")
		c.Header("Cross-Origin-Resource-Policy", "cross-origin")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func isHopByHop(header string) bool {
	switch strings.ToLower(header) {
	case "connection", "proxy-connection", "keep-alive",
		"proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade":
		return true
	}
	return false
}

// Run is a convenience function that creates a new Proxy and starts it.
func Run(config Config) error {
	return New(config).Run()
}
