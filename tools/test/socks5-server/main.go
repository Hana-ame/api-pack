package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"time"

	"github.com/things-go/go-socks5"
)

var ipstr string

func main() {
	flag.StringVar(&ipstr, "ip", "::1", "ipv6")
	flag.Parse()

	// Create a SOCKS5 server
	server := socks5.NewServer(
		socks5.WithLogger(socks5.NewLogger(log.New(os.Stdout, "socks5: ", log.LstdFlags))),
		socks5.WithBindIP(net.ParseIP(ipstr)),
		socks5.WithDial((&net.Dialer{ // dialer
			// LocalAddr 用于指定本地 IP 地址
			LocalAddr: &net.TCPAddr{
				IP: net.ParseIP(ipstr), // 将 "your_specific_ip" 替换为你要使用的特定 IP 地址
			},
			Timeout:   5 * time.Second,  // 连接超时时间
			KeepAlive: 30 * time.Second, // Keep-Alive 超时时间
			Resolver: &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					return net.Dial("udp", "1.1.1.1:53")
				},
			},
		}).DialContext),
	)

	// Create SOCKS5 proxy on localhost
	if err := server.ListenAndServe("tcp", "127.0.0.1:1080"); err != nil {
		panic(err)
	}
}
