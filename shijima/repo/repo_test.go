package repo

import (
	"database/sql"
	"testing"

	"github.com/Hana-ame/api-pack/shijima/model"
	_ "modernc.org/sqlite"
)

func setupRepo(t *testing.T) *Repo {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	r := New(db)
	if err := r.InitDB(); err != nil {
		t.Fatalf("initDB: %v", err)
	}
	if err := r.InitBotTable(); err != nil {
		t.Fatalf("initBotTable: %v", err)
	}
	return r
}

func TestChannelCreateAndList(t *testing.T) {
	r := setupRepo(t)

	id, err := r.ChannelCreate("general", "chat")
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero channel id")
	}

	chs, err := r.ChannelList()
	if err != nil {
		t.Fatalf("list channels: %v", err)
	}
	if len(chs) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(chs))
	}
	if chs[0].Mode != "chat" {
		t.Errorf("mode = %q, want \"chat\"", chs[0].Mode)
	}
}

func TestMessageCreateAndGet(t *testing.T) {
	r := setupRepo(t)
	cid, _ := r.ChannelCreate("test", "chat")

	m := &model.Message{
		ChannelID: cid,
		Author:    model.Author{ID: "user1", Name: "tester"},
		Content:   "hello world",
	}
	id, err := r.MessageCreate(m)
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero message id")
	}

	got, err := r.MessageGet(int(id))
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if got.Content != "hello world" {
		t.Errorf("content = %q, want %q", got.Content, "hello world")
	}
	if got.Author.Name != "tester" {
		t.Errorf("author = %q, want %q", got.Author.Name, "tester")
	}
}

func TestMessageList(t *testing.T) {
	r := setupRepo(t)
	cid, _ := r.ChannelCreate("test", "chat")

	for i := 0; i < 5; i++ {
		r.MessageCreate(&model.Message{
			ChannelID: cid,
			Author:    model.Author{ID: "u1"},
			Content:   "msg",
		})
	}

	msgs, err := r.MessageList(cid, 0, 10)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 5 {
		t.Errorf("expected 5 messages, got %d", len(msgs))
	}
}

func TestMessageReplies(t *testing.T) {
	r := setupRepo(t)
	cid, _ := r.ChannelCreate("test", "chat")

	parent := &model.Message{ChannelID: cid, Author: model.Author{ID: "u1"}, Content: "OP"}
	pid, _ := r.MessageCreate(parent)

	reply := &model.Message{ChannelID: cid, Author: model.Author{ID: "u2"}, Content: "reply", ParentID: int(pid)}
	r.MessageCreate(reply)

	replies, err := r.MessageReplies(int(pid))
	if err != nil {
		t.Fatalf("get replies: %v", err)
	}
	if len(replies) != 1 {
		t.Errorf("expected 1 reply, got %d", len(replies))
	}
}

func TestReaction(t *testing.T) {
	r := setupRepo(t)
	cid, _ := r.ChannelCreate("test", "chat")

	m := &model.Message{ChannelID: cid, Author: model.Author{ID: "u1"}, Content: "react"}
	mid, _ := r.MessageCreate(m)

	r.ReactionSet(int(mid), "👍")
	r.ReactionSet(int(mid), "👍")

	om, err := r.ReactionGet(int(mid))
	if err != nil {
		t.Fatalf("get reactions: %v", err)
	}
	if c := om.GetOrDefault("👍", 0).(int); c != 2 {
		t.Errorf("count = %d, want 2", c)
	}
}

func TestV2CompatPostAndGet(t *testing.T) {
	r := setupRepo(t)

	thread := &model.Thread{ID: "abc", Name: "anon", Content: "hello", Image: "a.jpg"}
	id, err := r.V2PostThread(thread, 1)
	if err != nil {
		t.Fatalf("v2 post: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	got, err := r.V2GetThread(int(id))
	if err != nil {
		t.Fatalf("v2 get: %v", err)
	}
	if got.Content != "hello" {
		t.Errorf("content = %q", got.Content)
	}

	// board listing
	threads, err := r.V2GetBoard(1, 0)
	if err != nil {
		t.Fatalf("v2 board: %v", err)
	}
	if len(threads) != 1 {
		t.Errorf("expected 1 thread in board, got %d", len(threads))
	}
}

func TestV2CompatReply(t *testing.T) {
	r := setupRepo(t)

	thread := &model.Thread{ID: "a", Content: "OP"}
	pid, _ := r.V2PostThread(thread, 1)

	reply := &model.Thread{ID: "b", Content: "reply", ReplyTo: uint(pid)}
	r.V2PostThread(reply, 1)

	replies, err := r.V2GetReplies(int(pid), 0)
	if err != nil {
		t.Fatalf("v2 replies: %v", err)
	}
	if len(replies) != 1 {
		t.Errorf("expected 1 reply, got %d", len(replies))
	}
}

func TestSchemaVersion(t *testing.T) {
	r := setupRepo(t)

	var version int
	r.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version)
	if version != 2 {
		t.Errorf("schema version = %d, want 2", version)
	}
}
