package qwen

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ==========================================
// 1. 数据结构
// ==========================================

type ChatRequest struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Search   bool   `json:"search"`
	Thinking bool   `json:"thinking"`
}

type WSMessage struct {
	Type     string `json:"type"` // "execute", "chunk", "done", "error"
	ReqID    string `json:"req_id"`
	Payload  string `json:"payload"`
	ChatID   string `json:"chat_id"`
	Search   bool   `json:"search"`
	Thinking bool   `json:"thinking"`
}

// 辅助结构：用于检测 SSE 结束信号
type QwenResponseCheck struct {
	Choices []struct {
		Delta struct {
			Status string `json:"status"` // finished
		} `json:"delta"`
	} `json:"choices"`
}

type BrowserAgent struct {
	ID            string
	Conn          *websocket.Conn
	ActiveStreams map[string]chan string
	StreamLock    sync.RWMutex

	// 状态管理
	IsBusy     bool
	BusyLock   sync.Mutex
	LastActive time.Time
	// 用于超时强制解锁
	BusyTimer *time.Timer
}

type AgentManager struct {
	Agents    map[string]*BrowserAgent
	ChatRoute map[string]string // ChatID -> AgentID
	Lock      sync.RWMutex
}

var (
	manager  = NewAgentManager()
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

// ==========================================
// 2. Agent 方法
// ==========================================

// 尝试锁定 Agent (原子操作)
func (a *BrowserAgent) TryLock() bool {
	a.BusyLock.Lock()
	defer a.BusyLock.Unlock()

	if a.IsBusy {
		return false
	}
	a.IsBusy = true

	// 设置看门狗：如果 3 分钟后还没解锁，强制解锁
	if a.BusyTimer != nil {
		a.BusyTimer.Stop()
	}
	a.BusyTimer = time.AfterFunc(3*time.Minute, func() {
		log.Printf("[Agent %s] Watchdog: Forced unlock due to timeout", a.ID)
		a.ForceUnlock()
	})

	return true
}

// 解锁 Agent
func (a *BrowserAgent) Unlock() {
	a.BusyLock.Lock()
	defer a.BusyLock.Unlock()

	if a.BusyTimer != nil {
		a.BusyTimer.Stop()
	}
	a.IsBusy = false
	log.Printf("[Agent %s] Released (Ready for next)", a.ID)
}

// 强制解锁 (用于外部调用，避免死锁)
func (a *BrowserAgent) ForceUnlock() {
	a.BusyLock.Lock()
	defer a.BusyLock.Unlock()
	a.IsBusy = false
}

// 检查 chunk 是否包含结束信号
func isFinishedSignal(jsonStr string) bool {
	// 快速过滤：如果字符串里都不包含 "finished"，就不用解析 JSON 了，省 CPU
	if !strings.Contains(jsonStr, "finished") {
		return false
	}

	var check QwenResponseCheck
	if err := json.Unmarshal([]byte(jsonStr), &check); err != nil {
		return false
	}
	if len(check.Choices) > 0 && check.Choices[0].Delta.Status == "finished" {
		return true
	}
	return false
}

// ==========================================
// 3. Manager 实现
// ==========================================

func NewAgentManager() *AgentManager {
	return &AgentManager{
		Agents:    make(map[string]*BrowserAgent),
		ChatRoute: make(map[string]string),
	}
}

func (m *AgentManager) AddAgent(conn *websocket.Conn) *BrowserAgent {
	agent := &BrowserAgent{
		ID:            fmt.Sprintf("agent-%d-%d", time.Now().UnixNano(), rand.Intn(1000)),
		Conn:          conn,
		ActiveStreams: make(map[string]chan string),
		LastActive:    time.Now(),
		IsBusy:        false,
	}

	m.Lock.Lock()
	defer m.Lock.Unlock()
	m.Agents[agent.ID] = agent
	log.Printf("[Manager] Agent added: %s (Total: %d)", agent.ID, len(m.Agents))
	return agent
}

func (m *AgentManager) RemoveAgent(agentID string) {
	m.Lock.Lock()
	defer m.Lock.Unlock()

	delete(m.Agents, agentID)

	// 清理路由表
	for chatID, assignedAgentID := range m.ChatRoute {
		if assignedAgentID == agentID {
			delete(m.ChatRoute, chatID)
		}
	}
	log.Printf("[Manager] Agent removed: %s", agentID)
}

// 核心路由逻辑：选择并锁定 Agent
func (m *AgentManager) PickAndLockAgent(chatID string) (*BrowserAgent, error) {
	m.Lock.RLock()
	defer m.Lock.RUnlock() // 注意：这里我们只读 Agents Map，Agent 内部状态由 Agent 自己的锁管理

	if len(m.Agents) == 0 {
		return nil, fmt.Errorf("no agents connected")
	}

	// 1. Sticky 路由策略
	if chatID != "" {
		if agentID, ok := m.ChatRoute[chatID]; ok {
			if agent, exists := m.Agents[agentID]; exists {
				// 尝试锁定
				if agent.TryLock() {
					log.Printf("[Router] Sticky hit & Locked: Agent %s", agent.ID)
					return agent, nil
				} else {
					return nil, fmt.Errorf("sticky agent %s is busy, please try again later", agent.ID)
				}
			}
			// 如果 Agent 不在了，fallthrough 到随机策略
			log.Printf("[Router] Sticky agent missing for chat %s, re-routing...", chatID)
		}
	}

	// 2. 随机/空闲 策略 (New Chat or Lost Sticky)
	// Go Map 遍历是随机的，所以我们只要找到第一个能 Lock 的就行
	for _, agent := range m.Agents {
		if agent.TryLock() {
			log.Printf("[Router] Selected & Locked idle agent: %s", agent.ID)
			return agent, nil
		}
	}

	return nil, fmt.Errorf("all agents are busy")
}

func (m *AgentManager) BindChat(chatID string, agentID string) {
	if chatID == "" {
		return
	}
	m.Lock.Lock()
	defer m.Lock.Unlock()
	m.ChatRoute[chatID] = agentID
}

// ==========================================
// 4. Handlers
// ==========================================

func Run(addr string) {
	if addr == "" {
		return
	}
	rand.Seed(time.Now().UnixNano())
	http.HandleFunc("/ws", handleWS)
	http.HandleFunc("/chat", handleChatAPI)
	fmt.Println("Server started on ", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	agent := manager.AddAgent(conn)
	defer manager.RemoveAgent(agent.ID)

	for {
		var msg WSMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}

		agent.LastActive = time.Now()

		// 检查是否需要解锁
		// 1. 显式 Done/Error 消息
		if msg.Type == "done" || msg.Type == "error" {
			agent.Unlock()
		}
		// 2. 数据流中的 finished 信号 (处理 data: {...})
		if msg.Type == "chunk" {
			// Chunk 的 payload 通常是 'data: {...}'
			// 我们需要去掉 'data: ' 前缀来解析 JSON
			cleanPayload := strings.TrimPrefix(msg.Payload, "data: ")
			if isFinishedSignal(cleanPayload) {
				log.Printf("[Agent %s] Detected 'finished' signal in stream", agent.ID)
				agent.Unlock()
			}
		}

		// 转发数据
		agent.StreamLock.RLock()
		ch, exists := agent.ActiveStreams[msg.ReqID]
		agent.StreamLock.RUnlock()

		if exists {
			switch msg.Type {
			case "chunk":
				ch <- msg.Payload
			case "done", "error":
				close(ch)
			}
		}
	}
}

func handleChatAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" {
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 1. 选择并锁定 Agent (如果所有忙，直接返回 503)
	agent, err := manager.PickAndLockAgent(req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	// 注意：agent 现在处于 Locked (Busy) 状态
	// 无论发生什么，Request 结束时必须确保没锁死 (虽然 WS loop 也会处理，但这里兜底)
	// 不过正常的解锁应该由 WS 收到 finished 信号触发
	// 这里只处理 API 发送失败的情况
	sendSuccess := false
	defer func() {
		if !sendSuccess {
			agent.Unlock()
		}
	}()

	if req.ID != "" {
		manager.BindChat(req.ID, agent.ID)
	}

	reqID := fmt.Sprintf("%d", time.Now().UnixNano())
	dataCh := make(chan string, 100)

	agent.StreamLock.Lock()
	agent.ActiveStreams[reqID] = dataCh
	agent.StreamLock.Unlock()

	defer func() {
		agent.StreamLock.Lock()
		delete(agent.ActiveStreams, reqID)
		agent.StreamLock.Unlock()
	}()

	cmd := WSMessage{
		Type:     "execute",
		ReqID:    reqID,
		ChatID:   req.ID,
		Payload:  req.Content,
		Search:   req.Search,
		Thinking: req.Thinking,
	}

	if err := agent.Conn.WriteJSON(cmd); err != nil {
		http.Error(w, "Failed to send to agent", http.StatusInternalServerError)
		return
	}

	// 发送成功，接下来的解锁任务交给 WS 监听循环
	sendSuccess = true

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	// 稍微延长的超时，防止深度思考时等待过久
	timeout := time.After(300 * time.Second)

loop:
	for {
		select {
		case chunk, open := <-dataCh:
			if !open {
				break loop
			}
			fmt.Fprint(w, chunk)
			flusher.Flush()
			timeout = time.After(300 * time.Second)
		case <-timeout:
			fmt.Fprintf(w, "event: error\ndata: timeout\n\n")
			// 超时也意味着出问题了，强制解锁
			agent.Unlock()
			break loop
		}
	}
}
