package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"

	shijima "github.com/Hana-ame/api-pack/shijima"
)

func main() {
	http.DefaultClient = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
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

	addr := os.Getenv("SHIJIMA")
	if addr == "" {
		addr = "127.25.5.19:8080"
	}

	log.Printf("shijima forum starting on %s", addr)
	if err := shijima.Run(addr); err != nil {
		log.Fatalf("shijima: %v", err)
	}
}
