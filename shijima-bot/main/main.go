package main

import (
	"log"
	"os"

	shijima_bot "github.com/Hana-ame/api-pack/shijima-bot"

	"github.com/gin-gonic/gin"
	tools "github.com/Hana-ame/api-pack/utils/utils"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	engine := shijima_bot.NewEngine()
	h := shijima_bot.NewHandler(engine)

	addr := tools.Or(os.Getenv("BOT_LISTEN"), "127.26.5.27:8080")
	log.Printf("bot service listening on %s", addr)
	log.Printf("forum base: %s", h.ForumBase)

	router := gin.Default()
	router.POST("/", h.HandleBotRequest)
	router.GET("/bots", h.ListBots)
	router.GET("/health", h.Health)

	if err := router.Run(addr); err != nil {
		log.Fatalf("failed to start: %v", err)
	}
}
