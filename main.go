package main

import (
	"fmt"

	"github.com/Hana-ame/api-pack/echo"
	"github.com/Hana-ame/api-pack/proxy"
	"github.com/gofiber/fiber/v2"
)

func main() {

	fmt.Println("0.1.0.1")

	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World 👋!")
	})

	app.Mount("/echo", echo.App())
	app.Mount("/proxy", proxy.App())

	app.Listen(":3000")
}
