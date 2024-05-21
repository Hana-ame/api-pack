package main

import (
	"api-pack/doh"
	"api-pack/jsdeliver"
	_ "api-pack/nyaa-proxy"
	_ "api-pack/sukebei-proxy"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	{
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}
	}

	go func() {
		err := jsdeliver.Router().Run("127.111.111.200:8080")
		log.Println(err)
	}()
	err := doh.Router().Run("127.111.111.120:8080")
	log.Println(err)

	// 	proxyClient := myfetch.NewProxyClient("http://localhost:10809")
	// 	myfetch.SetClients([]*http.Client{proxyClient})

	// 	router := gin.Default()

	// 	router.GET("/*any", functions.Proxy("www.jsdelivr.com", nil, nil))

	// router.Run("127.111.111.111:8080")
}
