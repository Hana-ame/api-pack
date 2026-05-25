package main

import (
	"flag"
	"fmt"
	"net"
	"os"
)

func main() {
	// 定义命令行标志，用于指定监听端口
	port := flag.String("port", "8080", "UDP port to listen on")
	flag.Parse()

	// 解析 UDP 地址
	address, err := net.ResolveUDPAddr("udp", ":"+*port)
	if err != nil {
		fmt.Println("Error resolving address:", err)
		os.Exit(1)
	}

	// 创建 UDP 连接
	conn, err := net.ListenUDP("udp", address)
	if err != nil {
		fmt.Println("Error starting UDP server:", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("UDP Server started on :%s\n", *port)

	buffer := make([]byte, 1024)

	for {
		// 读取客户端发送的数据
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading from connection:", err)
			continue
		}

		// 打印收到的数据
		received := string(buffer[:n])
		fmt.Printf("Received from %s: %s\n", addr.String(), received)

		// 回环发送数据
		_, err = conn.WriteToUDP(buffer[:n], addr)
		if err != nil {
			fmt.Println("Error writing to connection:", err)
			continue
		}
	}
}
