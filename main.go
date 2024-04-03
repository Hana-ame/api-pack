package main

import (
	doh "api-pack/doh"
	nyaa "api-pack/nyaa-proxy"
	sukebei "api-pack/sukebei-proxy"
	"log"
)

func main() {
	// nyaa
	go nyaa.Run()
	go sukebei.Run()

	err := doh.Router().Run("127.111.111.120:8080")
	log.Println(err)
}
