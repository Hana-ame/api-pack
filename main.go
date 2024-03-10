package main

import (
	doh "api-pack/doh"
	"log"
)

func main() {
	err := doh.Router().Run("127.111.111.120:8080")
	log.Println(err)
}
