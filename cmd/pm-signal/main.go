// pm-signal：独立启动 peer-message 自托管 PeerJS 信令服务器。
// 也可由 api-pack 主程序按 PM_SIGNAL env 门控启动（见 internal/app/app.go）。
package main

import (
	"flag"
	"os"

	"github.com/Hana-ame/api-pack/internal/service/pmsignal"
)

func main() {
	addr := flag.String("listen", "127.0.0.1:9600", "监听地址 host:port")
	flag.Parse()
	target := *addr
	if v := os.Getenv("PM_SIGNAL"); v != "" {
		target = v
	}
	pmsignal.Run(target)
}
