// shijima — moonchan forum server
// API v2 (compat) + API v3 (modern RESTful)
// SQLite3 backend, aarch64 compatible
package main

import (
	"net"
	"net/http"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"

	shijima "github.com/Hana-ame/api-pack/shijima"
	"github.com/Hana-ame/api-pack/utils/debug"
)

func localTCPAddrFromEnv() *net.TCPAddr {
	if ipStr := os.Getenv("LOCAL_IP"); ipStr != "" {
		if ip := net.ParseIP(ipStr); ip != nil {
			return &net.TCPAddr{IP: ip}
		}
	}
	return nil
}

func main() {
	http.DefaultClient = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				LocalAddr: localTCPAddrFromEnv(),
				Timeout:   15 * time.Second,
				KeepAlive: 90 * time.Second,
			}).DialContext,
			MaxIdleConns:        256,
			IdleConnTimeout:     10 * time.Second,
			TLSHandshakeTimeout: 30 * time.Second,
		},
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	debug.LogLevel = debug.Trace

	addr := os.Getenv("SHIJIMA")
	if addr == "" {
		addr = "127.25.5.19:8080"
	}
	if err := shijima.Run(addr); err != nil {
		panic(err)
	}
}
