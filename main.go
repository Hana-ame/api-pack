package main

import (
	"api-pack/doh"
	"api-pack/jsdeliver"
	_ "api-pack/nyaa-proxy"
	_ "api-pack/sukebei-proxy"
	"fmt"

	_ "github.com/joho/godotenv/autoload"

	_ "api-pack/files/autorun"
)

func main() {
	fmt.Printf("%d", 123)

	go jsdeliver.Router().Run("127.111.111.200:8080")

	go doh.Router().Run("127.111.111.120:8080")

	// 	proxyClient := myfetch.NewProxyClient("http://localhost:10809")
	// 	myfetch.SetClients([]*http.Client{proxyClient})

	// 	router := gin.Default()

	// 	router.GET("/*any", functions.Proxy("www.jsdelivr.com", nil, nil))

	// router.Run("127.111.111.111:8080")

	// stuck
	sucker := make(chan struct{})
	<-sucker
}
