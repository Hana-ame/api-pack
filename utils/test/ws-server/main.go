// go run main.go

package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有跨域请求
	},
}

func main() {
	// 定义命令行标志，用于指定监听端口
	port := flag.String("port", "8080", "WebSocket port to listen on")
	flag.Parse()

	// 设置 WebSocket 路由
	http.HandleFunc("/ws", handleConnection)

	// 启动 HTTP 服务器
	fmt.Printf("WebSocket Server started on :%s\n", *port)
	err := http.ListenAndServe(":"+*port, nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

// 处理 WebSocket 连接
func handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading connection:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Client connected")

	for {
		// 读取消息
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Error reading message:", err)
			break
		}

		// 打印收到的消息
		fmt.Printf("Received: %s\n", message)

		// 回环发送消息
		err = conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			fmt.Println("Error writing message:", err)
			break
		}
	}
}
