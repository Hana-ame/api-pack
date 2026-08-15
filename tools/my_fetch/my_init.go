// gin-pack @ 2024-04-06

package myfetch

import (
	"context"
	"net"
	"net/http"
	"time"
)

func init() {
	if tr, ok := http.DefaultTransport.(*http.Transport); ok {
		tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext(ctx, "tcp4", addr)
		}
	}
	DefaultClientPool = NewClientPool([]*http.Client{NewV4Client(nil)})
	DefaultFetcher = NewFetcher(nil, DefaultClientPool)
}
