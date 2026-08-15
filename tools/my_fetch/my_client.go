package myfetch

import (
	"context"
	"net"
	"net/http"
	"net/http/cookiejar"
	"time"
)

type Dialer struct {
	host string
	*net.Dialer
}

func (dialer Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return dialer.Dialer.DialContext(ctx, network, dialer.host)
}

func NewV4Client(cookieJar *cookiejar.Jar) *http.Client {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext(ctx, "tcp4", addr)
		},
	}
	if cookieJar == nil {
		cookieJar = jar
	}
	return &http.Client{
		Transport: tr,
		Jar:       cookieJar,
		Timeout:   5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 返回错误以阻止重定向
		},
	}
}

func NewV6Client(ip net.IP, cookieJar *cookiejar.Jar) *http.Client {

	tr := &http.Transport{
		DialContext: (Dialer{"exhentai.org:443", &net.Dialer{ // dialer
			// LocalAddr 用于指定本地 IP 地址
			LocalAddr: &net.TCPAddr{
				IP: ip, // 将 "your_specific_ip" 替换为你要使用的特定 IP 地址
			},
			Timeout:   3 * time.Second,  // 连接超时时间
			KeepAlive: 30 * time.Second, // Keep-Alive 超时时间
			Resolver: &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					return net.Dial("udp", "1.1.1.1:53")
				},
			},
		}}).DialContext,
	}
	if cookieJar == nil {
		cookieJar = jar
	}
	return &http.Client{
		Transport: tr,
		Jar:       cookieJar,
		Timeout:   5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 返回错误以阻止重定向
		},
	}
}
