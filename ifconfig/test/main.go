package main

import (
	"io"
	"log"
	"net"
	"net/http"

	"github.com/Hana-ame/api-pack/tools/debug"
	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch/v2"
)

func main() {
	{
		resp, err := myfetch.Fetch(
			"GET", "https://ifconfig.me/ip",
			nil, nil,
		)
		if err != nil {
			debug.E("why", err.Error())
			return
		}
		defer resp.Body.Close()

		html, err := io.ReadAll(resp.Body)
		if err != nil {
			debug.E("why", err.Error())
			return
		}
		log.Printf("%s\n", html)
	}
	{
		manager, err := myfetch.NewManager("sit1", "")
		if err != nil {
			debug.E("why", err)
		}

		for i := 1; i < 500; i++ {

			ip, err := manager.GenerateIP()
			if err != nil {
				debug.E("why", err)
			}

			manager.AddAddr(ip)
			defer manager.DelAddr(ip)

			mc := &myfetch.Client{
				Client: http.Client{
					Transport: &http.Transport{
						DialContext: (&net.Dialer{ // dialer
							// LocalAddr 用于指定本地 IP 地址
							LocalAddr: &net.TCPAddr{
								IP: net.IP(ip.AsSlice()),
							},
						}).DialContext,
					},
					Jar: nil,
					CheckRedirect: func(req *http.Request, via []*http.Request) error {
						return http.ErrUseLastResponse // 返回错误以阻止重定向
					}},
			}
			resp, err := mc.Fetch(
				"GET", "https://ifconfig.me/ip",
				nil, nil,
			)
			if err != nil {
				debug.E("why", err.Error())
				return
			}
			defer resp.Body.Close()

			html, err := io.ReadAll(resp.Body)
			if err != nil {
				debug.E("why", err.Error())
				return
			}
			log.Printf("%s\n", html)
		}
	}
}
