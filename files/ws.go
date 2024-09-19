package files

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	wsmux "api-pack/Tools/ws_mux"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 这里可以自定义校验规则，例如允许所有来源
		return true
	},
}

var server *wsmux.WsMux = nil

// 直接在这里写是好的。
func ServerHandler(c *gin.Context) {
	// 升级HTTP连接为WebSocket连接
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	defer ws.Close()

	if server != nil {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	server = wsmux.NewWsMux(ws, wsmux.MuxSeqServer)
	server.ReadDaemon(ws)

	// conn, _ := server.Dial()
	// conn.Write([]byte("../source.txt"))

	// file, _ := os.Create("dest.txt")
	// buf := make([]byte, 1024)
	// io.CopyBuffer(file, conn, buf)

	server = nil
}

// TODO
func ClientWsHandler(c *gin.Context) {
	// 升级HTTP连接为WebSocket连接
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ws.Close()

	_, data, _ := ws.ReadMessage()
	muc, _ := server.Dial()
	// buf := make([]byte, 1024)
	muc.Write(data)
	defer muc.Close()

	for {
		pkg := muc.ReadPackage()
		if len(pkg.Message) == 0 {
			return
		}
		ws.WriteMessage(websocket.BinaryMessage, pkg.Message)
	}
}

func ClientRESTHandler(c *gin.Context) {
	if server == nil {
		c.AbortWithError(http.StatusBadGateway, fmt.Errorf("server not ready"))
		return
	}

	sha1sum := c.Param("sha1sum")

	log.Println(sha1sum)

	muc, _ := server.Dial()
	defer muc.Close()
	// buf := make([]byte, 1024)
	muc.Write([]byte(sha1sum))

	// for {
	// 	pkg := muc.ReadPackage()
	// 	if len(pkg.Message) == 0 {
	// 		break
	// 	}
	// 	c.Writer.Write(pkg.Message)
	// }

	// 这里会卡住，不知道为啥 // solved
	// if n, err := io.CopyBuffer(c.Writer, muc, make([]byte, 1024)); err != nil {
	// 	c.AbortWithError(http.StatusInternalServerError, err)
	// 	return
	// } else {
	// 	log.Println(n)
	// }
	c.Header("Content-Type", "application/octet-stream")
	for {
		pkg := muc.ReadPackage()
		if n, err := c.Writer.Write(pkg.Message); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			break
		} else if n == 0 {
			break
		} else {
			log.Println(n)
		}
	}
}
