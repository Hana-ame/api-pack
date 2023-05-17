package main

import (
	"fmt"
	"net/http"

	"github.com/Hana-ame/api-pack/echo"
	"github.com/Hana-ame/api-pack/exproxy"
	"github.com/Hana-ame/api-pack/kv"
	ipx "github.com/Hana-ame/api-pack/proxy"
	missakujo "github.com/Hana-ame/missakujo/backend"
	"github.com/gofiber/fiber/v2"
)

func main() {

	fmt.Println("v0.5.6")

	// exhentai proxy
	go exproxy.Main("127.111.111.113:8080")
	go exproxy.S("127.111.111.114:8080")

	// img proxy use stream
	http.HandleFunc("/43df14f5", ipx.Img)
	go http.ListenAndServe("127.111.111.112:8080", nil)

	// use fiber.
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World 👋!")
	})

	app.Mount("/echo", echo.App())
	app.Mount("/proxy", ipx.App())
	app.Mount("/43df14f5", ipx.App())
	app.Mount("/missakujo", missakujo.App())
	// app.Mount("/8b92d4de", sign.App())
	app.Mount("/kv", kv.App())

	err := app.Listen("127.111.111.111:8080")
	// err := app.Listen(":3000")
	fmt.Println(err)

	defer func() {
		err := recover()
		fmt.Println(err)
	}()
}
