package main

import (

	// _ "github.com/Hana-ame/api-pack/nyaa-proxy"
	// _ "github.com/Hana-ame/api-pack/sukebei-proxy"

	"github.com/Hana-ame/api-pack/google"
	_ "github.com/joho/godotenv/autoload"
	// _ "github.com/Hana-ame/api-pack/files/autorun"
)

func main() {

	// go jsdeliver.Router().Run("127.111.111.200:8080")
	// go doh.Router().Run("127.111.111.120:8080")

	google.Router().Run("127.24.10.25:8080")
	// stuck
	// sucker := make(chan struct{})
	// <-sucker
}
