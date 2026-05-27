// shijima — forum + bot service
// API v2 (compat) + API v3 (modern RESTful)
// SQLite3 backend, aarch64 compatible
package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"

	shijima "github.com/Hana-ame/api-pack/shijima"
	shijima_bot "github.com/Hana-ame/api-pack/shijima-bot"
	"github.com/Hana-ame/api-pack/utils/debug"
	"github.com/gin-gonic/gin"
	tools "github.com/Hana-ame/api-pack/utils/utils"
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

	debug.LogLevel = debug.Trace

	// Forum
	go func() {
		addr := os.Getenv("SHIJIMA")
		if addr == "" {
			addr = "127.25.5.19:8080"
		}
		log.Printf("forum starting on %s", addr)
		if err := shijima.Run(addr); err != nil {
			log.Fatalf("forum: %v", err)
		}
	}()

	// Bot service
	gin.SetMode(gin.ReleaseMode)
	engine := shijima_bot.NewEngine()
	h := shijima_bot.NewHandler(engine)

	botAddr := tools.Or(os.Getenv("BOT_LISTEN"), "127.26.5.27:8080")
	log.Printf("bot service starting on %s (forum=%s)", botAddr, h.ForumBase)

	router := gin.Default()
	router.POST("/", h.HandleBotRequest)
	router.GET("/bots", h.ListBots)
	router.GET("/health", h.Health)

	if err := router.Run(botAddr); err != nil {
		log.Fatalf("bot service: %v", err)
	}
}
