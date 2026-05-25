package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	// 建立 WebSocket 连接
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/echo", nil)
	if err != nil {
		log.Fatal("连接失败:", err)
	}
	defer conn.Close()

	// 启动消息接收协程
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("接收回显失败:", err)
				return
			}
			log.Printf("收到回显: %s\n", message)
		}
	}()

	// 持续发送测试消息
	for i := 0; i < 5; i++ {
		msg := []byte(fmt.Sprintf("测试消息-%d", i))
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Fatal("发送失败:", err)
		}
		time.Sleep(1 * time.Second)
	}
}
