package shijima_bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Engine    *Engine
	ForumBase string
	BotCookie string
}

func NewHandler(e *Engine) *Handler {
	return &Handler{
		Engine:    e,
		ForumBase: os.Getenv("FORUM_BASE_URL"),
		BotCookie: os.Getenv("BOT_COOKIE"),
	}
}

// HandleBotRequest receives a bot invocation from the forum.
// POST /
func (h *Handler) HandleBotRequest(c *gin.Context) {
	var ctx Context
	if err := c.BindJSON(&ctx); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[bot] received: %s %q (msg=%d ch=%d)", ctx.Bot, ctx.Query, ctx.MessageID, ctx.ChannelID)

	responseText, err := h.Engine.Dispatch(ctx)
	if err != nil {
		log.Printf("[bot] %s failed: %v", ctx.Bot, err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Post reply to forum if there's content
	if responseText != "" {
		go h.postReply(ctx, responseText)
	}

	c.JSON(202, gin.H{"status": "accepted", "bot": ctx.Bot})
}

// ListBots returns all registered bots.
// GET /bots
func (h *Handler) ListBots(c *gin.Context) {
	c.JSON(200, gin.H{"bots": h.Engine.ListBots()})
}

// Health check.
// GET /health
func (h *Handler) Health(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) postReply(ctx Context, responseText string) {
	body := BuildReplyMessage(ctx.Bot, responseText, ctx)
	if body == nil {
		return
	}

	payload, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/v3/channels/%d/messages", h.ForumBase, ctx.ChannelID)

	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		log.Printf("[bot] callback error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "auth="+h.BotCookie)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[bot] callback error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		log.Printf("[bot] callback failed: %d %s", resp.StatusCode, string(b))
		return
	}
	log.Printf("[bot] callback ok: %s replied to msg %d", ctx.Bot, ctx.MessageID)
}
