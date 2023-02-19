package kv

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
)

func App() *fiber.App {
	app := fiber.New()

	app.Get("/*", func(c *fiber.Ctx) error {
		path := c.Params("*")

		data, err := os.ReadFile(path)
		if err != nil {
			log.Println("KV: Get", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		_, err = c.Write(data)
		return err
	})

	app.Post("/*", func(c *fiber.Ctx) error {
		path := c.Params("*")

		data := (c.Body())

		os.MkdirAll(filepath.Dir(path), 0644)
		if err := os.WriteFile(path, data, 0644); err != nil {
			log.Println("KV: Post", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return nil
	})

	return app
}
