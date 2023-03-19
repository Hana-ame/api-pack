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

	fmt.Println("0.5.2")

	go exproxy.Main("127.111.111.113:8080")

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

	http.HandleFunc("/43df14f5", ipx.Img)
	go http.ListenAndServe("127.111.111.112:8080", nil)

	err := app.Listen("127.111.111.111:8080")
	// err := app.Listen(":3000")
	fmt.Println(err)

	defer func() {
		err := recover()
		fmt.Println(err)
	}()
}
