package main

import (
	"api-pack/doh"
	"api-pack/jsdeliver"
	_ "api-pack/nyaa-proxy"
	_ "api-pack/sukebei-proxy"
	"fmt"
	"log"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Printf("%d", 123)

	go func() {
		err := jsdeliver.Router().Run("127.111.111.200:8080")
		log.Println(err)
	}()

	{
		err := doh.Router().Run("127.111.111.120:8080")
		log.Println(err)
	}
	// 	proxyClient := myfetch.NewProxyClient("http://localhost:10809")
	// 	myfetch.SetClients([]*http.Client{proxyClient})

	// 	router := gin.Default()

	// 	router.GET("/*any", functions.Proxy("www.jsdelivr.com", nil, nil))

	// router.Run("127.111.111.111:8080")
}
