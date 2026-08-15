package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// Conn 接口
type Conn interface {
	io.ReadWriteCloser
}

// Mux 多路复用器
type Mux struct {
	tcpConn net.Conn
	clients map[uint32]Conn
	mu      sync.Mutex
	nextID  uint32
}

// NewMux 创建一个新的多路复用器
func NewMux(tcpConn net.Conn) *Mux {
	return &Mux{
		tcpConn: tcpConn,
		clients: make(map[uint32]Conn),
		nextID:  1,
	}
}

// Dial 创建一个新的连接
func (m *Mux) Dial() (Conn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := m.nextID
	m.nextID++
	client := &client{
		mux:    m,
		id:     id,
		rw:     bufio.NewReadWriter(bufio.NewReader(m.tcpConn), bufio.NewWriter(m.tcpConn)),
		closed: false,
	}

	m.clients[id] = client
	return client, nil
}

// Close 关闭多路复用器
func (m *Mux) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		client.Close()
	}
	delete(m.clients, 0)
	return m.tcpConn.Close()
}

// client 表示一个客户端连接
type client struct {
	mux    *Mux
	id     uint32
	rw     *bufio.ReadWriter
	closed bool
}

// Read 从连接中读取数据
func (c *client) Read(p []byte) (n int, err error) {
	if c.closed {
		return 0, io.EOF
	}

	// 读取数据包
	header := make([]byte, 4)
	_, err = io.ReadFull(c.rw.Reader, header)
	if err != nil {
		return 0, err
	}

	// 解析数据包
	id := binary.BigEndian.Uint32(header)
	if id != c.id {
		return 0, fmt.Errorf("invalid client ID: %d, expected: %d", id, c.id)
	}

	// 读取数据
	n, err = c.rw.Reader.Read(p)
	if err != nil {
		return 0, err
	}

	return n, nil
}

// Write 向连接中写入数据
func (c *client) Write(p []byte) (n int, err error) {
	if c.closed {
		return 0, io.EOF
	}

	// 创建数据包
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, c.id)
	_, err = c.rw.Writer.Write(header)
	if err != nil {
		return 0, err
	}

	// 写入数据
	n, err = c.rw.Writer.Write(p)
	if err != nil {
		return 0, err
	}

	// 刷新缓冲区
	err = c.rw.Writer.Flush()
	if err != nil {
		return 0, err
	}

	return n, nil
}

// Close 关闭连接
func (c *client) Close() error {
	// c.mu.Lock()
	// defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	delete(c.mux.clients, c.id)
	return nil
}

// main 函数
func main() {
	// 创建一个TCP监听器
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		log.Fatalf("Error starting listener: %v", err)
	}
	defer listener.Close()

	fmt.Println("Listening on 127.0.0.1:8080")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		go handleConnection(conn)
	}
}

// handleConnection 处理客户端连接
func handleConnection(conn net.Conn) {
	defer conn.Close()

	mux := NewMux(conn)

	// 创建多个客户端连接
	client1, err := mux.Dial()
	if err != nil {
		log.Printf("Error dialing client: %v", err)
		return
	}
	defer client1.Close()

	client2, err := mux.Dial()
	if err != nil {
		log.Printf("Error dialing client: %v", err)
		return
	}
	defer client2.Close()

	// 读取和写入数据
	go func() {
		for {
			message := "Hello from client 1"
			_, err := client1.Write([]byte(message))
			if err != nil {
				log.Printf("Error writing to client 1: %v", err)
				return
			}
			fmt.Printf("Client 1 sent: %s\n", message)

			time.Sleep(1 * time.Second)
		}
	}()

	go func() {
		for {
			message := "Hello from client 2"
			_, err := client2.Write([]byte(message))
			if err != nil {
				log.Printf("Error writing to client 2: %v", err)
				return
			}
			fmt.Printf("Client 2 sent: %s\n", message)

			time.Sleep(1 * time.Second)
		}
	}()

	go func() {
		buffer := make([]byte, 1024)
		for {
			n, err := client1.Read(buffer)
			if err != nil {
				log.Printf("Error reading from client 1: %v", err)
				return
			}
			fmt.Printf("Client 1 received: %s\n", buffer[:n])
		}
	}()

	go func() {
		buffer := make([]byte, 1024)
		for {
			n, err := client2.Read(buffer)
			if err != nil {
				log.Printf("Error reading from client 2: %v", err)
				return
			}
			fmt.Printf("Client 2 received: %s\n", buffer[:n])
		}
	}()

	// 保持连接
	select {}
}
