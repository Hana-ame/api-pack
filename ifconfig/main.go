package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"github.com/Hana-ame/api-pack/tools/debug"
	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch"
	"github.com/Hana-ame/api-pack/tools/my_fetch/my_if"
	tools "github.com/Hana-ame/api-pack/tools/utils"
)

var jar *cookiejar.Jar = nil

func main() {

	prefix := tools.NewSlice(
		os.Getenv("EXHENTAI_PROXY_PREFIX"),
		"2001:470:c:6c:",
	).FirstUnequal("")

	ips := []net.IP{my_if.NewAddr(prefix), my_if.NewAddr(prefix)}
	ipidx := 0

	my_if.AddAddr(ips[ipidx].String())
	cp := myfetch.NewClientPool([]*http.Client{
		myfetch.NewV6Client(ips[ipidx], jar),
	})

	// cp = nil // debug
	mf := myfetch.NewFetcher(nil, cp)

	for i := 0; i < 2000; i++ {
		resp, err := mf.Fetch(
			"GET", "https://ifconfig.me/ip",
			nil, nil)
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

		if mf.Count() > 2 {
			ipidx = (ipidx + 1) % len(ips)
			defer func(ip string) {
				time.Sleep(10 * time.Second)
				my_if.DelAddr(ip)
			}(ips[ipidx].String())
			ips[ipidx] = my_if.NewAddr(prefix)
			my_if.AddAddr(ips[ipidx].String())
			newCp := myfetch.NewClientPool([]*http.Client{myfetch.NewV6Client(ips[ipidx], jar)})
			mf.SetClientPool(newCp)
		}
	}
}
