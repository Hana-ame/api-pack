package proxy

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
)

const dstBase = "https://moonchan.xyz/img/api/download/"

// how to use stream?
func App() *fiber.App {
	app := fiber.New()

	// app.Use(func(c *fiber.Ctx) error {
	// 	fmt.Println(c.Path()+"middleware")
	// 	return c.Next()
	// })

	app.All("/:id/:fn", func(c *fiber.Ctx) error {
		id := c.Params("id")
		fn := c.Params("fn")
		url := dstBase + id + "/" + fn

		if err := proxy.Do(c, url); err != nil {
			return err
		}

		c.Response().Header.Set("Cache-Control", "public, max-age=31536000")
		c.Response().Header.Set("Cache-Access-Control-Allow-Origin", "*")

		return nil
	})

	return app
}
