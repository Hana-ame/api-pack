package handler

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true }, // 允许所有来源
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket 升级失败:", err)
		return
	}
	defer conn.Close()

	for {
		// 读取客户端消息
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("消息读取失败:", err)
			break
		}

		// 业务逻辑处理（示例：原样返回消息）
		if err := conn.WriteMessage(messageType, p); err != nil {
			log.Println("消息回写失败:", err)
			break
		}
	}
}

var (
	connections = make(map[*websocket.Conn]bool) // 全局连接池
	connLock    sync.RWMutex                     // 读写锁
)

// 定义 WebSocket 路由
func WS(c *gin.Context) {
	// 协议升级
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket 升级失败:", err)
		return
	}
	defer conn.Close()

	conn.SetPingHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return conn.WriteControl(websocket.PongMessage, nil, time.Now().Add(10*time.Second))
	})

	// 加入连接池
	connLock.Lock()
	connections[conn] = true
	connLock.Unlock()

	// 消息监听循环
	for {	
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break // 连接断开时退出循环
		}

		// 广播消息到所有连接
		broadcastMessage(msg)
	}

	// 连接断开后移除
	connLock.Lock()
	delete(connections, conn)
	connLock.Unlock()
}

func broadcastMessage(message []byte) {
	connLock.RLock()
	defer connLock.RUnlock()

	for conn := range connections {
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Println("广播失败:", err)
			conn.Close()
			delete(connections, conn) // 自动清理失效连接
		}
	}
}
