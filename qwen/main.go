package main

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
type ChatRequest struct {
	ID      string `json:"id"`      // 如果为空，则新建 Chat
	Content string `json:"content"` // 提问内容
}

// 定义 WS 消息协议
type WSMessage struct {
	Type    string `json:"type"`    // "command" (S->C), "chunk" (C->S), "done" (C->S), "error" (C->S)
	ReqID   string `json:"req_id"`  // 请求ID，用于关联 HTTP 请求
	Payload string `json:"payload"` // 文本内容或 JSON 字符串
	ChatID  string `json:"chat_id"` // 用于指令携带 chat_id
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Global Session Manager
type BrowserSession struct {
	Conn *websocket.Conn
	Lock sync.Mutex
	// 用于将 WS 收到的流数据传回 HTTP Handler
	// Key: RequestID, Value: Channel
	ActiveStreams map[string]chan string
	StreamLock    sync.RWMutex
}

var session *BrowserSession

func main() {
	http.HandleFunc("/ws", handleWS)
	http.HandleFunc("/chat", handleChatAPI)

	fmt.Println("Server started on :8765")
	log.Fatal(http.ListenAndServe(":8765", nil))
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

	// 初始化 session (简化版：只支持单浏览器单标签)
	session = &BrowserSession{
		Conn:          conn,
		ActiveStreams: make(map[string]chan string),
	}

	for {
		// 读取浏览器发回的消息
		var msg WSMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("WS Read error:", err)
			break
		}

		// 根据消息类型分发
		session.StreamLock.RLock()
		ch, exists := session.ActiveStreams[msg.ReqID]
		session.StreamLock.RUnlock()

		if !exists {
			continue // 可能请求已超时关闭
		}

		switch msg.Type {
		case "chunk":
			ch <- msg.Payload
		case "done", "error":
			close(ch)
			// 清理 Map 在 HTTP handler 侧做，或者这里做也可以
		}
	}

	log.Println("Browser disconnected")
	session = nil
}

// 2. 处理外部 API 请求 (HTTP POST)
func handleChatAPI(w http.ResponseWriter, r *http.Request) {
	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" {
		return
	}

	if session == nil || session.Conn == nil {
		http.Error(w, "Browser agent not connected", http.StatusServiceUnavailable)
		return
	}

	// 1. 获取锁，确保串行处理 (Qwen 网页不能并发)
	session.Lock.Lock()
	defer session.Lock.Unlock()

	// 2. 解析请求
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 3. 准备通道
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

	// 4. 发送指令给 Tampermonkey
	cmd := WSMessage{
		Type:    "execute",
		ReqID:   reqID,
		ChatID:  req.ID,      // 如果为空，JS 端处理为 New Chat
		Payload: req.Content, // Prompt
	}

	if err := session.Conn.WriteJSON(cmd); err != nil {
		http.Error(w, "Failed to send command to browser", http.StatusInternalServerError)
		return
	}

	// 5. 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// 6. 监听通道并回写流
	// 这是一个简单的超时控制
	timeout := time.After(60 * time.Second)

loop:
	for {
		select {
		case chunk, open := <-dataCh:
			if !open {
				break loop // 通道关闭，传输结束
			}
			// 透传 chunk (注意：fetch interceptor 拿到的往往是原始的 data: ... 字符串)
			// 如果已经是 SSE 格式，直接写；如果不是，可能需要 fmt.Fprintf(w, "data: %s\n\n", chunk)
			// 根据前面的 Log，浏览器拿到的已经是 data: {...} 格式
			fmt.Fprint(w, chunk)
			flusher.Flush()

			// 重置超时 (可选逻辑)
			timeout = time.After(60 * time.Second)

		case <-timeout:
			fmt.Fprintf(w, "event: error\ndata: timeout\n\n")
			break loop
		}
	}
}
