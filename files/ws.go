package files

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	wsmux "api-pack/Tools/ws_mux"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 32,
	WriteBufferSize: 1024 * 32,
	CheckOrigin: func(r *http.Request) bool {
		// 这里可以自定义校验规则，例如允许所有来源
		return true
	},
}

func wsHandler(c *gin.Context) {
	// 升级HTTP连接为WebSocket连接
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ws.Close()

	wsm := &wsmux.WsMux{Conn: ws}
	// 处理WebSocket消息
	for {
		messageType, data, err := ws.ReadMessage()
		if err != nil {
			break
		}
		if messageType != websocket.BinaryMessage {
			continue
		}

		pack, err := wsmux.FromBytes(data)
		if err != nil {
			continue
		}
		// fmt.Println(string(data))

		// 发送消息给客户端
		// go func() {
		// time.Sleep(time.Duration(rand.Intn(10)) * time.Second)
		pack.ID ^= 1
		wsm.WriteMessage(websocket.BinaryMessage, pack.ToBytes())
		// }()

	}
}
