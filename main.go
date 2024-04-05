package main

import (
	doh "api-pack/doh"
	"api-pack/jsdeliver"
	_ "api-pack/nyaa-proxy"
	_ "api-pack/sukebei-proxy"
	"log"
)

func main() {
	go func() {
		err := jsdeliver.Router().Run("127.111.111.200:8080")
		log.Println(err)
	}()
	err := doh.Router().Run("127.111.111.120:8080")
	log.Println(err)

}
