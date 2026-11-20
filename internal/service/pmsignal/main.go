// pm-signal — PeerJS 兼容的极简信令服务器（自托管，替代 0.peerjs.com）
//
// 行为按用户裁定：
//  1. **同 ID 冲突时新的直接顶掉旧的** —— 不比对 token，不返回 ID-TAKEN。
//     旧 socket 收到 LEAVE 后被关闭；新 socket 收到 OPEN 接管。
//     这消除了「自己断网留下的旧登记把自己挡在门外 35 分钟」的整类问题。
//  2. **快速过期** —— 心跳 PONG 超时默认 15s 即释放 ID；socket 一断立即释放。
//     不再有云端半开连接残留。
//
// 与 peerjs 1.5.4 客户端线协议完全兼容（已从 node_modules/peerjs/dist/bundler.mjs 取证）：
//
//	连接 URL：  ws://host:port{path}peerjs?key=<key>&id=<id>&token=<token>&version=<v>
//	客户端→服务器：{"type":"HEARTBEAT" | "OFFER" | "ANSWER" | "CANDIDATE" | "LEAVE",
//	                "payload":{...}, "dst":"<对方id>"}
//	服务器→客户端：{"type":"OPEN" | "HEARTBEAT"(PONG) | "OFFER" | "ANSWER" | "CANDIDATE"
//	                     | "LEAVE" | "EXPIRE" | "ERROR" | "ID-TAKEN" | "INVALID-KEY",
//	                "src":"<来源id>", "payload":{...}}
//	HTTP API：  GET {path}{key}/id      → 随机 ID（纯文本）
//	            GET {path}{key}/peers   → 在线 ID 列表（JSON 数组；allow_discovery 开启才给）
//
// 注意：本服务只做信令；WebRTC 媒体/数据通道仍是 PeerJS 客户端之间直连（STUN 打洞）。
package pmsignal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultListen = "127.0.0.1:9600"
	defaultKey    = "peerjs"
	defaultPath   = "/"
	// 客户端心跳间隔 5s（peerjs 默认 pingInterval）。读超时给足余量：
	// 15s 太紧——CF 橙云中转 + TLS 抖动 + 客户端首次心跳调度延迟，实测会造成
	// 每 7s 一次误判断线（Lost connection to server）并触发自我顶替风暴。
	// 30s = 6 个心跳周期，仍远小于旧云端 35 分钟的残留期，快速过期语义不变。
	idleTimeout = 30 * time.Second
	writeWait   = 10 * time.Second
	pongWait    = idleTimeout
	// wsPingInterval 协议层 ping 周期。CF 橙云对 WS 有 ~100s 空闲上限，且不转发
	// 应用层帧时也可能掐连接；RFC 6455 的 ping/pong 是 CF 官方推荐的保活手段。
	// 25s 远小于 100s，同时短于 idleTimeout 以便 pong 失败能被及时判定。
	wsPingInterval = 25 * time.Second
	maxMsgSize     = 1 << 20 // 单帧上限 1MB（SDP 可能较大）
)

// 消息类型常量（与 peerjs ServerMessageType 一致，字符串枚举）。
const (
	msgHeartbeat  = "HEARTBEAT"
	msgOffer      = "OFFER"
	msgAnswer     = "ANSWER"
	msgCandidate  = "CANDIDATE"
	msgLeave      = "LEAVE"
	msgExpire     = "EXPIRE"
	msgError      = "ERROR"
	msgOpen       = "OPEN"
	msgIDTaken    = "ID-TAKEN"
	msgInvalidKey = "INVALID-KEY"
)

// client 一个已注册的 peer 连接。
type client struct {
	id      string
	token   string
	conn    *websocket.Conn
	writeMu sync.Mutex
	// 关闭时保证只广播/清理一次
	closed atomic.Bool
	reg    *registry
}

func (c *client) send(msg map[string]any) bool {
	body, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed.Load() {
		// 已关闭：不写（避免向已关闭 socket 写导致 panic）；但 kick() 在置位前
		// 先直接调用 sendRaw 发 LEAVE，见下。
		return false
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(websocket.TextMessage, body) == nil
}

// sendRaw 不检查 closed 的底层写，仅供 kick() 在被顶替时抢发 LEAVE 帧用。
// 调用方必须已持有 writeMu 或保证无并发写（kick 是单次终结路径）。
func (c *client) sendRaw(msg map[string]any) {
	body, err := json.Marshal(msg)
	if err != nil {
		return
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(time.Second))
	_ = c.conn.WriteMessage(websocket.TextMessage, body)
}

// kick 被顶替时通知并关闭旧连接。
//
// 顺序关键：先抢发 LEAVE（客户端据此立即 cleanup 旧的 Peer 状态），再发 Close 帧、
// 关 socket。若先把 closed 置位再 send()，LEAVE 会被 send() 的 closed 检查吞掉，
// 旧客户端只能等 TCP 断开才发现被顶替（实测表现为「旧连接没被踢」）。
func (c *client) kick() {
	if c.closed.Swap(true) {
		return
	}
	c.sendRaw(map[string]any{"type": msgLeave, "src": c.id})
	func() {
		c.writeMu.Lock()
		defer c.writeMu.Unlock()
		_ = c.conn.SetWriteDeadline(time.Now().Add(time.Second))
		_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "replaced"))
	}()
	_ = c.conn.Close()
}

// registry 在线 peer 表。核心语义：同 ID 后者顶替前者。
type registry struct {
	mu      sync.RWMutex
	clients map[string]*client
	// 统计
	replaced  atomic.Uint64
	opened    atomic.Uint64
	expired   atomic.Uint64
	key       string
	discovery bool
	// hbSamples peerId -> 心跳计数（仅用于节流日志，不参与逻辑）
	hbSamples map[string]int
}

func newRegistry(key string, discovery bool) *registry {
	return &registry{clients: make(map[string]*client), key: key, discovery: discovery, hbSamples: make(map[string]int)}
}

// claim 注册新连接。已有同 ID → 顶掉旧的（用户明确要求的行为）。
func (g *registry) claim(id, token string, conn *websocket.Conn) (*client, bool) {
	newClient := &client{id: id, token: token, conn: conn, reg: g}
	g.mu.Lock()
	old := g.clients[id]
	g.clients[id] = newClient
	g.mu.Unlock()
	if old != nil && old != newClient {
		g.replaced.Add(1)
		log.Printf("[pm-signal] peer %s: 新连接顶替旧连接（token 不比对，按裁定直接顶掉）", shortID(id))
		old.kick()
		return newClient, true
	}
	g.opened.Add(1)
	return newClient, false
}

// release 连接断开时清理；只有当自己仍是当前持有者时才删（避免把新连接删掉）。
func (g *registry) release(c *client) {
	g.mu.Lock()
	if g.clients[c.id] == c {
		delete(g.clients, c.id)
		log.Printf("[pm-signal] peer %s: 断线释放 ID", shortID(c.id))
	}
	g.mu.Unlock()
	c.closed.Store(true)
}

func (g *registry) find(id string) *client {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.clients[id]
}

func (g *registry) snapshot() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ids := make([]string, 0, len(g.clients))
	for id := range g.clients {
		ids = append(ids, id)
	}
	return ids
}

func (g *registry) count() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.clients)
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12] + "…"
}

// server HTTP + WS 装配。
type server struct {
	reg      *registry
	path     string // 例如 "/"（与客户端 options.path 对齐）
	upgrader websocket.Upgrader
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "online": s.reg.count(),
			"opened": s.reg.opened.Load(), "replaced": s.reg.replaced.Load(), "expired": s.reg.expired.Load(),
		})
	})
	return mux
}

// handleRoot 同时承担三种请求：
//
//	GET  {path}peerjs?key=...&id=...   → WebSocket 升级（信令）
//	GET  {path}{key}/id                → 随机 ID
//	GET  {path}{key}/peers             → 在线列表
func (s *server) handleRoot(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	base := strings.TrimSuffix(s.path, "/")

	// WebSocket 信令入口：{base}/peerjs
	if strings.HasSuffix(urlPath, "/peerjs") || urlPath == base+"/peerjs" {
		s.handleSignal(w, r)
		return
	}
	// HTTP API：{base}/{key}/id 与 {base}/{key}/peers
	trimmed := strings.TrimPrefix(urlPath, base+"/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) == 2 && parts[0] == s.reg.key {
		switch parts[1] {
		case "id":
			w.Header().Set("content-type", "text/plain; charset=utf-8")
			w.Header().Set("cache-control", "no-store")
			_, _ = w.Write([]byte(randomID()))
			return
		case "peers":
			if !s.reg.discovery {
				http.Error(w, "discovery disabled", http.StatusUnauthorized)
				return
			}
			writeJSON(w, http.StatusOK, s.reg.snapshot())
			return
		}
	}
	http.NotFound(w, r)
}

func (s *server) handleSignal(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	key := q.Get("key")
	id := q.Get("id")
	token := q.Get("token")

	if key != s.reg.key {
		// 客户端期待 INVALID-KEY：先升级再发错误帧，它才能正确归类。
		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		_ = conn.WriteJSON(map[string]any{"type": msgInvalidKey})
		_ = conn.Close()
		return
	}
	if id == "" || token == "" {
		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		_ = conn.WriteJSON(map[string]any{"type": msgError, "payload": map[string]any{"msg": "Invalid WS parameters"}})
		_ = conn.Close()
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[pm-signal] upgrade failed: %v", err)
		return
	}
	conn.SetReadLimit(maxMsgSize)

	// ✅ 核心行为：直接顶掉同 ID 的旧连接，不做 token 比对。
	cl, replaced := s.reg.claim(id, token, conn)
	// 新连接（无论顶替还是首注）立即 OPEN，开始收发。
	cl.send(map[string]any{"type": msgOpen})
	log.Printf("[pm-signal] peer %s: OPEN（%s）", shortID(id),
		map[bool]string{true: "顶替旧连接", false: "新注册"}[replaced])

	// ── WS 协议层保活（RFC 6455 ping/pong）────────────────────────────
	// 应用层 HEARTBEAT 是 peerjs 自己的 5s 心跳；协议层 ping 是给中间设备（CF、
	// 运营商 NAT）看的，防止它们因「空闲」掐掉长连接。两者互不替代。
	// 收到 pong 即续读超时；ping 循环用 WriteControl 并发安全地写控制帧。
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	pingDone := make(chan struct{})
	var pingOnce sync.Once
	stopPing := func() { pingOnce.Do(func() { close(pingDone) }) }
	go func() {
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-pingDone:
				return
			case <-ticker.C:
				// 走 cl 的统一写锁：send() 用 SetWriteDeadline 后 WriteMessage，
				// 与 WriteControl 的 deadline 参数操作同一个 conn 写状态，不加锁会
				// 互相踩（gorilla 只保证 WriteControl 可与其他「已加锁的」写并发）。
				// 25s 一次，锁竞争可忽略。
				cl.writeMu.Lock()
				deadline := time.Now().Add(writeWait)
				err := cl.conn.WriteControl(websocket.PingMessage, []byte(time.Now().Format(time.RFC3339)), deadline)
				cl.writeMu.Unlock()
				if err != nil {
					stopPing()
					return
				}
			}
		}
	}()

	// 读超时即过期：客户端每 5s 发 HEARTBEAT，读循环续期；无消息超过 pongWait 即判死。
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))

	func() {
		defer stopPing()
		defer s.reg.release(cl)
		defer func() { _ = conn.Close() }()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					// 超时也会走到这里：快速过期生效
					if ne, ok := err.(net.Error); ok && ne.Timeout() {
						s.reg.expired.Add(1)
						log.Printf("[pm-signal] peer %s: 心跳超时（%s），快速释放 ID", shortID(id), pongWait)
					}
				}
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(pongWait))
			var msg map[string]any
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			s.dispatch(cl, msg)
		}
	}()
}

// dispatch 处理客户端消息，按 peerjs 语义转发或回应。
func (s *server) dispatch(from *client, msg map[string]any) {
	typ, _ := msg["type"].(string)
	switch typ {
	case msgHeartbeat:
		// 刷新由读循环的 SetReadDeadline 完成。
		// 注意：**不回 PONG**。peerjs 1.5.4 的 _handleMessage 没有 HEARTBEAT
		// case，收到会落到 default 分支并 warn "malformed message"；它的
		// _sendHeartbeat 只是保活，不需要应答。原实现回包反而制造噪音。
		// 心跳可见性：首次 + 每 60s 打一条，能区分「没收到心跳」与「收到了但不回」。
		s.reg.mu.Lock()
		n := s.reg.hbSamples[from.id] + 1
		s.reg.hbSamples[from.id] = n
		s.reg.mu.Unlock()
		if n == 1 || n%12 == 0 {
			log.Printf("[pm-signal] peer %s: HEARTBEAT #%d（链路存活）", shortID(from.id), n)
		}
		return
	case msgLeave:
		s.reg.release(from)
		from.closed.Store(true)
		return
	}

	dst, _ := msg["dst"].(string)
	payload := msg["payload"]
	if dst == "" {
		return
	}
	target := s.reg.find(dst)
	if target == nil {
		// 对方不在线：peerjs 客户端把 EXPIRE 归成 peer-unavailable（好友离线），不是本端故障。
		from.send(map[string]any{"type": msgExpire, "src": dst})
		return
	}
	// 补 src 后原样转发，并回 ACK 语义由客户端自行处理（无需额外 ACK 帧）。
	target.send(map[string]any{"type": typ, "src": from.id, "payload": payload})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, `{"ok":false}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "application/json")
	w.Header().Set("cache-control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func randomID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// Run 启动信令服务器。addr 为空时回退到默认 loopback 地址。
//
// env（可选，覆盖默认值）：
//
//	PM_SIGNAL_KEY        客户端 key（默认 peerjs）
//	PM_SIGNAL_PATH       客户端 path（默认 /）
//	PM_SIGNAL_DISCOVERY  1/true 时允许 /peers 列在线 ID
func Run(addr string) {
	if addr == "" {
		addr = defaultListen
	}
	key := envOr("PM_SIGNAL_KEY", defaultKey)
	path := envOr("PM_SIGNAL_PATH", defaultPath)
	discovery := envOr("PM_SIGNAL_DISCOVERY", "") == "1" || strings.EqualFold(envOr("PM_SIGNAL_DISCOVERY", ""), "true")

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	srv := &server{
		reg:  newRegistry(key, discovery),
		path: path,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			// 自托管信令：允许任意 Origin（客户端是 Node 或本地页面）
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpServer := &http.Server{
		Handler:           srv.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[pm-signal] listen %s: %v", addr, err)
		return
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("[pm-signal] listening on %s (key=%q path=%q idle-timeout=%s discovery=%t)",
		addr, key, path, idleTimeout, discovery)
	log.Printf("[pm-signal] 行为：同 ID 新连接直接顶掉旧连接；%s 无心跳即释放 ID", idleTimeout)
	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("[pm-signal] serve: %v", err)
	}
	log.Println("[pm-signal] stopped")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
