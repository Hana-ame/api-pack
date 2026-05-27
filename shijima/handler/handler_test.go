package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hana-ame/api-pack/shijima/bot"
	"github.com/Hana-ame/api-pack/shijima/model"
	"github.com/Hana-ame/api-pack/shijima/repo"
	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

func middlewareForTest(c *gin.Context) {
	c.Set("id", "test-user")
	c.Next()
}

func setup(t *testing.T) (*Handler, *gin.Engine) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	r := repo.New(db)
	r.InitDB()
	r.InitBotTable()

	bengine := bot.NewEngine(db)
	h := New(r, bengine)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	h.RegisterV3Routes(engine, middlewareForTest)
	return h, engine
}

func TestV3ChannelCreate(t *testing.T) {
	_, engine := setup(t)

	body := strings.NewReader(`{"name":"general","mode":"chat"}`)
	req := httptest.NewRequest("POST", "/api/v3/channels", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create channel: %d %s", w.Code, w.Body.String())
	}
	var ch model.Channel
	json.Unmarshal(w.Body.Bytes(), &ch)
	if ch.Mode != "chat" {
		t.Errorf("mode = %q", ch.Mode)
	}
}

func TestV3ChannelList(t *testing.T) {
	_, engine := setup(t)

	// seed a channel
	engine.ServeHTTP(httptest.NewRecorder(),
		newJSONReq("POST", "/api/v3/channels", `{"name":"test"}`))

	req := httptest.NewRequest("GET", "/api/v3/channels", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("list channels: %d", w.Code)
	}
}

func TestV3MessageCreateAndList(t *testing.T) {
	_, engine := setup(t)

	engine.ServeHTTP(httptest.NewRecorder(),
		newJSONReq("POST", "/api/v3/channels", `{"name":"test"}`))

	msgReq := newJSONReq("POST", "/api/v3/channels/1/messages", `{"content":"hello"}`)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, msgReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("create message: %d %s", w.Code, w.Body.String())
	}

	listReq := httptest.NewRequest("GET", "/api/v3/channels/1/messages", nil)
	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, listReq)
	if w2.Code != http.StatusOK {
		t.Errorf("list messages: %d", w2.Code)
	}

	getReq := httptest.NewRequest("GET", "/api/v3/messages/1", nil)
	w3 := httptest.NewRecorder()
	engine.ServeHTTP(w3, getReq)
	if w3.Code != http.StatusOK {
		t.Errorf("get message: %d", w3.Code)
	}
}

func TestV3Reaction(t *testing.T) {
	_, engine := setup(t)

	engine.ServeHTTP(httptest.NewRecorder(),
		newJSONReq("POST", "/api/v3/channels", `{"name":"test"}`))
	engine.ServeHTTP(httptest.NewRecorder(),
		newJSONReq("POST", "/api/v3/channels/1/messages", `{"content":"test"}`))

	setReq := newJSONReq("POST", "/api/v3/messages/1/reactions", `"👍"`)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, setReq)
	if w.Code != http.StatusOK {
		t.Errorf("set reaction: %d", w.Code)
	}

	getReq := httptest.NewRequest("GET", "/api/v3/messages/1/reactions", nil)
	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, getReq)
	if w2.Code != http.StatusOK {
		t.Errorf("get reactions: %d", w2.Code)
	}
}

func TestV3BotList(t *testing.T) {
	_, engine := setup(t)

	req := httptest.NewRequest("GET", "/api/v3/bots", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("bot list: %d", w.Code)
	}
}

func newJSONReq(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}
