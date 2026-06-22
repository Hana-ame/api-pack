package exhentai_modify

import (
	"net/http"
	"time"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch/v2"
	"github.com/joho/godotenv"
)

var defaultClient = &myfetch.Client{
	Client: &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100, // + 关键：补上这行！否则默认只有 2，并发一高立刻导致大量 Wait
			IdleConnTimeout:     10 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	},
}

func Run(addr string) {
	if addr == "" {
		return
	}
	godotenv.Load(".env")

	ExhProxy(defaultClient, addr)
}
