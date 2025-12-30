package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch/v2"
)

func main() {
	manager, _ := myfetch.NewManager("sit1", "")
	ip, _ := manager.GenerateIP()
	manager.AddAddr(ip)

	client := &myfetch.Client{
		Client: &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					LocalAddr: &net.TCPAddr{
						IP: ip.AsSlice(),
					},
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	resp, err := client.Get("https://ifconfig.me/ip")
	fmt.Println(err)
	body, err := io.ReadAll(resp.Body)
	fmt.Println(err)
	fmt.Printf("%s", body)
}
