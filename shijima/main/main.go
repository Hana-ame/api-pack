package main

import (
	_ "github.com/Hana-ame/api-pack/Tools"
	"github.com/Hana-ame/api-pack/shijima"
)

func main() {
	shijima.Run("127.25.5.18:8080")
	// URL, _ := url.Parse("/api/v2/preview/media/GrfZh0daUAAhFwi?format=jpg&name=small&host=pbs.twimg.com")
	// query := URL.Query()
	// query.Del("host")
	// fmt.Println(query.Encode())
}
