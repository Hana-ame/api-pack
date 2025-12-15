package qwen

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// 定义 API 请求结构
// Go 的 bool 默认值为 false，所以如果不传这两个字段，默认为 false
type ChatRequest struct {
	ID       string `json:"id"`       // 如果为空，则新建 Chat
	Content  string `json:"content"`  // 提问内容
	Search   bool   `json:"search"`   // 是否开启联网搜索
	Thinking bool   `json:"thinking"` // 是否开启深度思考
}

// 定义 WS 消息协议
type WSMessage struct {
	Type     string `json:"type"`     // "execute", "chunk", "done", "error"
	ReqID    string `json:"req_id"`   // 请求ID
	Payload  string `json:"payload"`  // 文本内容
	ChatID   string `json:"chat_id"`  //
	Search   bool   `json:"search"`   // 指令：开启搜索
	Thinking bool   `json:"thinking"` // 指令：开启思考
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Global Session Manager
type BrowserSession struct {
	Conn          *websocket.Conn
	Lock          sync.Mutex
	ActiveStreams map[string]chan string
	StreamLock    sync.RWMutex
}

var session *BrowserSession

func main() {
	http.HandleFunc("/ws", handleWS)
	http.HandleFunc("/chat", handleChatAPI)

	fmt.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// 1. 处理 Tampermonkey 的 WebSocket 连接
func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	log.Println("Browser connected!")

	session = &BrowserSession{
		Conn:          conn,
		ActiveStreams: make(map[string]chan string),
	}

	for {
		var msg WSMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("WS Read error:", err)
			break
		}

		session.StreamLock.RLock()
		ch, exists := session.ActiveStreams[msg.ReqID]
		session.StreamLock.RUnlock()

		if !exists {
			continue
		}

		if msg.Type == "chunk" {
			ch <- msg.Payload
		} else if msg.Type == "done" || msg.Type == "error" {
			close(ch)
		}
	}

	log.Println("Browser disconnected")
	session = nil
}

// 2. 处理外部 API 请求 (HTTP POST)
func handleChatAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" {
		return
	}

	if session == nil || session.Conn == nil {
		http.Error(w, "Browser agent not connected", http.StatusServiceUnavailable)
		return
	}

	// 互斥锁，确保串行处理
	session.Lock.Lock()
	defer session.Lock.Unlock()

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	reqID := fmt.Sprintf("%d", time.Now().UnixNano())
	dataCh := make(chan string, 100)

	session.StreamLock.Lock()
	session.ActiveStreams[reqID] = dataCh
	session.StreamLock.Unlock()

	defer func() {
		session.StreamLock.Lock()
		delete(session.ActiveStreams, reqID)
		session.StreamLock.Unlock()
	}()

	// 发送指令给 Tampermonkey
	cmd := WSMessage{
		Type:     "execute",
		ReqID:    reqID,
		ChatID:   req.ID,
		Payload:  req.Content,
		Search:   req.Search,   // 透传布尔值
		Thinking: req.Thinking, // 透传布尔值
	}

	if err := session.Conn.WriteJSON(cmd); err != nil {
		http.Error(w, "Failed to send command to browser", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	timeout := time.After(120 * time.Second) // 增加超时时间以适应 DeepThinking

loop:
	for {
		select {
		case chunk, open := <-dataCh:
			if !open {
				break loop
			}
			fmt.Fprint(w, chunk)
			flusher.Flush()
			timeout = time.After(120 * time.Second)
		case <-timeout:
			fmt.Fprintf(w, "event: error\ndata: timeout\n\n")
			break loop
		}
	}
}
