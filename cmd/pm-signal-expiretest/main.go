// pm-signal-expiretest：验证自建信令的「快速过期」——注册后不发心跳，应被 15s 超时释放。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

func health(baseURL string) (online int, expired int) {
	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		return -1, -1
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var v struct {
		Online  int `json:"online"`
		Expired int `json:"expired"`
	}
	_ = json.Unmarshal(body, &v)
	return v.Online, v.Expired
}

func main() {
	addr := flag.String("addr", "127.0.0.1:9701", "信令服务器地址")
	wait := flag.Duration("wait", 18*time.Second, "静默等待时长（应 > idle timeout 15s）")
	flag.Parse()

	base := "http://" + *addr
	id := "dsh-pm-" + "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	url := fmt.Sprintf("ws://%s/peerjs?key=peerjs&id=%s&token=t&version=1.5.4", *addr, id)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		fmt.Println("dial failed:", err)
		os.Exit(1)
	}
	defer conn.Close()

	// 等 OPEN
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("read failed:", err)
			os.Exit(1)
		}
		var msg map[string]any
		_ = json.Unmarshal(data, &msg)
		if msg["type"] == "OPEN" {
			break
		}
	}
	online0, expired0 := health(base)
	fmt.Printf("已注册，online=%d expired=%d；开始静默 %s（不发心跳）\n", online0, expired0, *wait)

	time.Sleep(*wait)

	online1, expired1 := health(base)
	fmt.Printf("静默后 online=%d expired=%d\n", online1, expired1)

	pass := online1 < online0 || expired1 > expired0
	if pass {
		fmt.Println("PASS  快速过期：无心跳超时后被释放")
		os.Exit(0)
	}
	fmt.Println("FAIL  超时未释放")
	os.Exit(1)
}
