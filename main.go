package main

import (
	"fmt"

	"github.com/Hana-ame/api-pack/echo"
	"github.com/Hana-ame/api-pack/proxy"
	missakujo "github.com/Hana-ame/missakujo/backend"
	"github.com/gofiber/fiber/v2"
)

func main() {

	fmt.Println("0.2.0")

	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World 👋!")
	})

	app.Mount("/echo", echo.App())
	app.Mount("/proxy", proxy.App())
	app.Mount("/43df14f5", proxy.App())
	app.Mount("/missakujo", missakujo.App())

	err := app.Listen("127.111.111.111:8080")
	// err := app.Listen(":3000")
	fmt.Println(err)

	defer func() {
		err := recover()
		fmt.Println(err)
	}()
}
