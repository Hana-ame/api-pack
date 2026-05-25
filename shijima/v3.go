package shijima

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Hana-ame/api-pack/shijima/bot"
	"github.com/Hana-ame/api-pack/utils/orderedmap"
	tools "github.com/Hana-ame/api-pack/utils/utils"
	"github.com/gin-gonic/gin"
)

// ============================================================
// API v3 — 现代 RESTful 路由
// ============================================================

func registerV3Routes(r *gin.Engine) {
	v3 := r.Group("/api/v3")

	// Boards
	v3.GET("/boards", v3GetBoards)
	v3.GET("/boards/:bid", v3GetBoard)

	// Threads
	v3.GET("/threads/:tid", v3GetThread)
	v3.POST("/threads", checkID, v3PostThread)
	v3.DELETE("/threads/:tid", checkID, v3DeleteThread)

	// Reactions (仅使用 alt 聚合模式)
	v3.GET("/threads/:tid/reactions", v3GetReactions)
	v3.POST("/threads/:tid/reactions", checkID, v3SetReaction)

	// New reactions
	v3.GET("/new-reactions", v3GetNewReactions)
	v3.DELETE("/new-reactions/:tid", v3DeleteNewReaction)

	// Covers
	v3.GET("/covers/random", v3GetCover)
	v3.GET("/covers/bili", v3GetCoverBili)
	v3.POST("/covers", checkID, v3AddCover)

	// Random URL
	v3.GET("/random", v3GetRandom)
	v3.POST("/random", v3AddRandom)

	// Auth
	v3.POST("/auth/cookie", v3Cookie)

	// Bots
	v3.GET("/bots", v3ListBots)
	v3.POST("/bots/:name", checkID, v3InvokeBot)
	v3.GET("/bots/:name/:tid", v3GetBotResponse)
}

// ---- Boards ----

func v3GetBoards(c *gin.Context) {
	rows, err := db.Query(`SELECT DISTINCT bid FROM board ORDER BY bid ASC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	boards := make([]gin.H, 0)
	for rows.Next() {
		var bid int
		if err := rows.Scan(&bid); err != nil {
			continue
		}
		boards = append(boards, gin.H{"bid": bid})
	}
	c.JSON(http.StatusOK, boards)
}

func v3GetBoard(c *gin.Context) {
	bid := tools.Atoi(c.Param("bid"), 0)
	pn := tools.Atoi(c.Query("pn"), 0)
	if bid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bid"})
		return
	}
	threads, err := getBoard(bid, pn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, threads)
}

// ---- Threads ----

func v3GetThread(c *gin.Context) {
	tid := tools.Atoi(c.Param("tid"), 0)
	pn := tools.Atoi(c.Query("pn"), 0)
	if tid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tid"})
		return
	}
	thread, err := getThread(tid, pn)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, thread)
}

func v3PostThread(c *gin.Context) {
	bid := tools.Atoi(c.Query("bid"), 0)
	tid := tools.Atoi(c.Query("tid"), 0)

	var thread Thread
	if err := c.BindJSON(&thread); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	thread.R = tools.Or(thread.R, uint(tid))
	if thread.R == 0 && bid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plz set tid or bid"})
		return
	}

	thread.ID = c.GetString("id")
	thread.C = c.GetHeader("Cf-Ipcountry")
	thread.IP = c.GetHeader("X-Forwarded-For")

	lastInsertID, err := postThread(&thread, bid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	thread.No = uint(lastInsertID)
	c.JSON(http.StatusCreated, thread)
}

func v3DeleteThread(c *gin.Context) {
	tid := tools.Atoi(c.Param("tid"), 0)
	id := c.GetString("id")
	ip := c.GetHeader("X-Forwarded-For")

	if tid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tid"})
		return
	}

	if err := deleteThread(tid, id, ip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": tid})
}

// ---- Reactions ----

func v3GetReactions(c *gin.Context) {
	tid := tools.Atoi(c.Param("tid"), 0)
	if tid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tid"})
		return
	}
	counts, err := getReactionsAlt(tid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reactions": counts})
}

func v3SetReaction(c *gin.Context) {
	tid := tools.Atoi(c.Param("tid"), 0)
	if tid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tid"})
		return
	}

	reaction := string(tools.Match(c.GetRawData()).Result())
	if err := setReactionAlt(tid, reaction); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if reaction == "🎲" {
		postThread(&Thread{
			ID:  c.GetString("id"),
			R:   uint(tid),
			Txt: "自动roll点 [1,100]\n@rd 1d100",
		}, 0)
	}

	c.JSON(http.StatusOK, gin.H{"reaction": reaction})
}

// ---- New Reactions ----

func v3GetNewReactions(c *gin.Context) {
	if id := c.Query("delete"); id != "" {
		new50.Delete(tools.Atoi(id, 0))
		c.JSON(http.StatusOK, gin.H{"deleted": id})
		return
	}
	getNewReactions(c)
}

func v3DeleteNewReaction(c *gin.Context) {
	tid := tools.Atoi(c.Param("tid"), 0)
	new50.Delete(tid)
	c.JSON(http.StatusOK, gin.H{"deleted": tid})
}

// ---- Covers ----

func v3GetCover(c *gin.Context) {
	getRandomRecordHandler(c)
}

func v3GetCoverBili(c *gin.Context) {
	getRandomRecordHandlerBili(c)
}

func v3AddCover(c *gin.Context) {
	addURLHandler(c)
}

// ---- Random ----

func v3GetRandom(c *gin.Context) {
	getRandomHandler(c)
}

func v3AddRandom(c *gin.Context) {
	addRandomHandler(c)
}

// ---- Auth ----

func v3Cookie(c *gin.Context) {
	cookie(c)
}

// ============================================================
// Bot 系统 v3 — 可配置 Bot + 同步回调
// ============================================================

func initBotTable() error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS bot_config (
		name TEXT PRIMARY KEY,
		endpoint TEXT DEFAULT '',
		description TEXT DEFAULT ''
	)`)
	if err != nil {
		return err
	}
	// 种子内置 bot（如果不存在）
	for _, b := range []struct{ name, endpoint, desc string }{
		{"@rd", "", "随机数投掷，格式: 1d100"},
		{"@board", "", "版块跳转，格式: {bid} {name}"},
		{"@reaction", "", "添加反应表情"},
	} {
		_, _ = db.Exec(`INSERT OR IGNORE INTO bot_config (name, endpoint, description) VALUES (?, ?, ?)`,
			b.name, b.endpoint, b.desc)
	}
	return nil
}

type BotConfig struct {
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	Description string `json:"description"`
}

func v3ListBots(c *gin.Context) {
	rows, err := db.Query(`SELECT name, endpoint, description FROM bot_config ORDER BY name`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	bots := make([]BotConfig, 0)
	for rows.Next() {
		var b BotConfig
		if err := rows.Scan(&b.Name, &b.Endpoint, &b.Description); err != nil {
			continue
		}
		bots = append(bots, b)
	}
	c.JSON(http.StatusOK, bots)
}

// v3InvokeBot 异步调用 bot，结果作为帖子回复写入
// POST /api/v3/bots/:name?tid=123
// Body: {"query": "1d100"} 或纯文本
func v3InvokeBot(c *gin.Context) {
	botName := c.Param("name")
	if !strings.HasPrefix(botName, "@") {
		botName = "@" + botName
	}

	tid := tools.Atoi(c.Query("tid"), 0)
	id := c.GetString("id")
	ip := c.GetHeader("X-Forwarded-For")

	if tid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tid is required"})
		return
	}

	// 获取 query
	query := c.Query("q")
	if query == "" {
		body, _ := c.GetRawData()
		var m map[string]interface{}
		if json.Unmarshal(body, &m) == nil {
			if q, ok := m["query"].(string); ok {
				query = q
			}
		}
	}

	// 查找 bot 配置
	var cfg BotConfig
	err := db.QueryRow(`SELECT name, endpoint, description FROM bot_config WHERE name = ?`, botName).
		Scan(&cfg.Name, &cfg.Endpoint, &cfg.Description)

	if err != nil {
		cfg = BotConfig{Name: botName, Endpoint: "", Description: "external bot"}
	}

	// 标记 pending 状态
	bot.InsertOrUpdate(int64(tid), botName, query, "pending", "")

	// 异步执行 bot
	go func() {
		responseText, err := dispatchBot(cfg, tid, query, id, ip)
		if err != nil {
			bot.InsertOrUpdate(int64(tid), botName, query, "failed", err.Error())
			return
		}

		// 写入 bot 表
		bot.InsertOrUpdate(int64(tid), botName, query, "done", responseText)

		// 将 bot 回复作为帖子写入
		replyThread := &Thread{
			T:   botName,
			N:   botName,
			ID:  "bot",
			R:   uint(tid),
			Txt: responseText,
		}
		postThread(replyThread, 0)
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"status": "pending",
		"bot":    botName,
		"query":  query,
	})
}

// v3GetBotResponse 获取特定 bot 在特定帖子中的响应记录
func v3GetBotResponse(c *gin.Context) {
	botName := c.Param("name")
	if !strings.HasPrefix(botName, "@") {
		botName = "@" + botName
	}
	tid := tools.Atoi(c.Param("tid"), 0)
	if tid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tid"})
		return
	}

	rows, err := db.Query(
		`SELECT no, t, n, ts, id, txt FROM thread WHERE r = ? AND n = ? AND del >= 0 ORDER BY no DESC`,
		tid, botName,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	replies := make([]*Thread, 0)
	for rows.Next() {
		var t Thread
		if err := rows.Scan(&t.T, &t.N, &t.Ts, &t.ID, &t.No, &t.Txt); err != nil {
			continue
		}
		replies = append(replies, &t)
	}
	c.JSON(http.StatusOK, replies)
}

// ---- Bot dispatch ----

func dispatchBot(cfg BotConfig, tid int, query string, id, ip string) (string, error) {
	// 内置 bot
	if cfg.Endpoint == "" {
		return dispatchBuiltinBot(cfg.Name, int64(tid), query)
	}
	// 外部 webhook bot
	return dispatchExternalBot(cfg, tid, query, id, ip)
}

func dispatchBuiltinBot(name string, tid int64, query string) (string, error) {
	body, status, err := bot.Response(tid, name, query)
	if err != nil {
		return "", err
	}
	if status == "done" || status == "" {
		var wrapper orderedmap.OrderedMap
		if json.Unmarshal(body, &wrapper) == nil {
			if t, ok := wrapper.Get("@type"); ok {
				switch t {
				case "text":
					if txt, ok := wrapper.Get("text"); ok {
						return fmt.Sprintf("%v", txt), nil
					}
				case "board":
					bid, _ := wrapper.Get("bid")
					name, _ := wrapper.Get("name")
					return fmt.Sprintf(`{"@type":"board","bid":%v,"name":"%v"}`, bid, name), nil
				}
			}
		}
		return string(body), nil
	}
	return "", fmt.Errorf("bot status: %s", status)
}

func dispatchExternalBot(cfg BotConfig, tid int, query string, id, ip string) (string, error) {
	// 获取 thread 上下文
	thread, err := getThreadByNo(tid)
	if err != nil {
		return "", fmt.Errorf("failed to get thread: %w", err)
	}

	payload := map[string]interface{}{
		"tid":    tid,
		"query":  query,
		"thread": thread,
		"user":   id,
	}

	body, _ := json.Marshal(payload)

	resp, err := http.Post(cfg.Endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("bot request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read bot response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		// HTML 响应包装为 @type:html
		escaped, _ := json.Marshal(string(respBody))
		return fmt.Sprintf(`{"@type":"html","html":%s}`, string(escaped)), nil
	}

	// 尝试解析为 JSON
	var result map[string]interface{}
	if json.Unmarshal(respBody, &result) == nil {
		if _, hasType := result["@type"]; hasType {
			return string(respBody), nil
		}
	}

	// 纯文本响应
	escaped, _ := json.Marshal(string(respBody))
	return fmt.Sprintf(`{"@type":"text","text":%s}`, string(escaped)), nil
}

// triggerBots 从发帖文本中扫描 @botname 并异步分派
// 由 post handler 在发帖后调用
func triggerBots(triggerNo int64, parentNo int64, txt, userID, userIP string) {
	for _, line := range strings.Split(txt, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "@") {
			continue
		}
		botName, query, err := tools.SeprateString(" ", trimmed)
		if err != nil || botName == "" {
			continue
		}

		// 异步执行每个 bot
		go func(botName, query string) {
				defer func() { if r := recover(); r != nil { fmt.Printf("bot panic %s: %v", botName, r) } }()
			// 查找配置
			var cfg BotConfig
			err := db.QueryRow(`SELECT name, endpoint, description FROM bot_config WHERE name = ?`, botName).
				Scan(&cfg.Name, &cfg.Endpoint, &cfg.Description)
			if err != nil {
				cfg = BotConfig{Name: botName, Endpoint: "", Description: "unregistered"}
			}

			bot.InsertOrUpdate(triggerNo, botName, query, "pending", "")

			responseText, err := dispatchBot(cfg, int(triggerNo), query, userID, userIP)
			if err != nil {
				bot.InsertOrUpdate(triggerNo, botName, query, "failed", err.Error())
				return
			}

			bot.InsertOrUpdate(triggerNo, botName, query, "done", responseText)

			// 将 bot 回复作为帖子写入
			postThread(&Thread{
				T:   botName,
				N:   botName,
				ID:  "bot",
				R:   uint(parentNo),
				Txt: responseText,
			}, 0)
		}(botName, query)
	}
}

func parseBotResponse(raw string) (htmlContent string, isWidget bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err == nil && u.Host != "" {
			return fmt.Sprintf(`<iframe src="%s" width="100%%" height="400" frameborder="0" sandbox="allow-scripts allow-same-origin"></iframe>`, raw), true
		}
	}
	var m map[string]interface{}
	if json.Unmarshal([]byte(raw), &m) == nil {
		switch m["@type"] {
		case "html":
			if h, ok := m["html"].(string); ok {
				return h, true
			}
		case "iframe":
			if u, ok := m["url"].(string); ok {
				h := "400"
				if v, ok := m["h"]; ok {
					h = fmt.Sprintf("%v", v)
				}
				return fmt.Sprintf(`<iframe src="%s" width="100%%" height="%s" frameborder="0" sandbox="allow-scripts allow-same-origin"></iframe>`, u, h), true
			}
		case "text":
			if t, ok := m["text"].(string); ok {
				return t, false
			}
		}
		return raw, false
	}
	return raw, false
}
