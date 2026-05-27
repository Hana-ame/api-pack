package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/Hana-ame/api-pack/shijima/model"
	tools "github.com/Hana-ame/api-pack/utils/utils"
	"github.com/gin-gonic/gin"
)

var botServiceURL = "http://127.26.5.27:8080"

func init() {
	if u := os.Getenv("BOT_SERVICE_URL"); u != "" {
		botServiceURL = u
	}
}

func (h *Handler) RegisterV3Routes(r *gin.Engine, auth gin.HandlerFunc) {
	v3 := r.Group("/api/v3")

	v3.GET("/channels", h.V3ChannelList)
	v3.POST("/channels", auth, h.V3ChannelCreate)
	v3.GET("/channels/:cid", h.V3ChannelGet)

	v3.GET("/channels/:cid/messages", h.V3MessageList)
	v3.POST("/channels/:cid/messages", auth, h.V3MessageCreate)
	v3.GET("/messages/:mid", h.V3MessageGet)
	v3.PATCH("/messages/:mid", auth, h.V3MessageEdit)
	v3.DELETE("/messages/:mid", auth, h.V3MessageDelete)

	v3.GET("/messages/:mid/replies", h.V3MessageReplies)
	v3.GET("/messages/:mid/reactions", h.V3ReactionGet)
	v3.POST("/messages/:mid/reactions", auth, h.V3ReactionSet)

	v3.POST("/auth/cookie", CookieHandler)
}

// ---- Channels ----

func (h *Handler) V3ChannelList(c *gin.Context) {
	chs, err := h.Repo.ChannelList()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, chs)
}

func (h *Handler) V3ChannelCreate(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
		Mode string `json:"mode"`
	}
	if err := c.BindJSON(&body); err != nil || body.Name == "" {
		c.JSON(400, gin.H{"error": "name required"})
		return
	}
	id, err := h.Repo.ChannelCreate(body.Name, body.Mode)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	ch, _ := h.Repo.ChannelGet(id)
	c.JSON(201, ch)
}

func (h *Handler) V3ChannelGet(c *gin.Context) {
	cid := tools.Atoi(c.Param("cid"), 0)
	ch, err := h.Repo.ChannelGet(cid)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, ch)
}

// ---- Messages ----

func (h *Handler) V3MessageList(c *gin.Context) {
	cid := tools.Atoi(c.Param("cid"), 0)
	before := tools.Atoi(c.Query("before"), 0)
	limit := tools.Atoi(c.Query("limit"), 30)

	msgs, err := h.Repo.MessageList(cid, before, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.enrichMessages(msgs)
	c.JSON(200, msgs)
}

func (h *Handler) V3MessageCreate(c *gin.Context) {
	cid := tools.Atoi(c.Param("cid"), 0)
	if cid == 0 {
		c.JSON(400, gin.H{"error": "invalid channel"})
		return
	}

	var body struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		Image    string `json:"image"`
		ParentID int    `json:"parent_id"`
		Name     string `json:"name"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	m := &model.Message{
		ChannelID: cid,
		ParentID:  body.ParentID,
		Title:     body.Title,
		Author: model.Author{
			ID:   c.GetString("id"),
			Name: tools.Or(body.Name, "无名氏"),
		},
		Content: body.Content,
		Image:   body.Image,
		Country: c.GetHeader("Cf-Ipcountry"),
		IP:      c.GetHeader("X-Forwarded-For"),
	}

	id, err := h.Repo.MessageCreate(m)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	m.ID = int(id)

	go h.triggerBots(m)

	c.JSON(201, m)
}

func (h *Handler) V3MessageGet(c *gin.Context) {
	mid := tools.Atoi(c.Param("mid"), 0)
	m, err := h.Repo.MessageGet(mid)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	h.enrichMessages([]*model.Message{m})
	var rc int
	h.Repo.DB().QueryRow(`SELECT COUNT(*) FROM message WHERE parent_id = ? AND deleted = 0`, mid).Scan(&rc)
	m.ReplyCount = rc
	c.JSON(200, m)
}

func (h *Handler) V3MessageEdit(c *gin.Context) {
	mid := tools.Atoi(c.Param("mid"), 0)
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.Repo.MessageEdit(mid, body.Title, body.Content); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"id": mid})
}

func (h *Handler) V3MessageDelete(c *gin.Context) {
	mid := tools.Atoi(c.Param("mid"), 0)
	if err := h.Repo.MessageDelete(mid, c.GetString("id"), c.GetHeader("X-Forwarded-For")); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"deleted": mid})
}

// ---- Replies ----

func (h *Handler) V3MessageReplies(c *gin.Context) {
	mid := tools.Atoi(c.Param("mid"), 0)
	msgs, err := h.Repo.MessageReplies(mid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.enrichMessages(msgs)
	c.JSON(200, msgs)
}

// ---- Reactions ----

func (h *Handler) V3ReactionGet(c *gin.Context) {
	mid := tools.Atoi(c.Param("mid"), 0)
	counts, _ := h.Repo.ReactionGet(mid)
	c.JSON(200, gin.H{"reactions": counts})
}

func (h *Handler) V3ReactionSet(c *gin.Context) {
	mid := tools.Atoi(c.Param("mid"), 0)
	emoji := string(tools.Match(c.GetRawData()).Result())
	if err := h.Repo.ReactionSet(mid, emoji); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if emoji == "🎲" {
		msg, _ := h.Repo.MessageGet(mid)
		if msg != nil {
			h.Repo.MessageCreate(&model.Message{
				ChannelID: msg.ChannelID,
				ParentID:  mid,
				Content:   "auto roll [1,100]\n@rd 1d100",
			})
		}
	}
	c.JSON(200, gin.H{"emoji": emoji})
}

// ---- helpers ----

func (h *Handler) enrichMessages(msgs []*model.Message) {
	for _, m := range msgs {
		if om, err := h.Repo.ReactionGet(m.ID); err == nil && len(om.Keys()) > 0 {
			rc := make(map[string]int)
			for _, k := range om.Keys() {
				rc[k] = om.GetOrDefault(k, 0).(int)
			}
			m.Reactions = rc
		}
		var rc int
		h.Repo.DB().QueryRow(`SELECT COUNT(*) FROM message WHERE parent_id = ? AND deleted = 0`, m.ID).Scan(&rc)
		m.ReplyCount = rc
	}
}

func (h *Handler) triggerBots(m *model.Message) {
	for _, line := range strings.Split(m.Content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "@") {
			continue
		}
		botName, query, err := tools.SeprateString(" ", trimmed)
		if err != nil || botName == "" {
			continue
		}

		go func(botName, query string) {
			defer func() { recover() }()
			payload, _ := json.Marshal(map[string]interface{}{
				"bot":        botName,
				"query":      query,
				"message_id": m.ID,
				"channel_id": m.ChannelID,
				"user_id":    m.Author.ID,
			})
			http.Post(botServiceURL, "application/json", bytes.NewReader(payload))
		}(botName, query)
	}
}
