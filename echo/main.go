package echo

import (
	"bytes"
	"fmt"

	"github.com/gofiber/fiber/v2"
)

func App() *fiber.App {
	app := fiber.New()

	// app.Use(func(c *fiber.Ctx) error {
	// 	fmt.Println(c.Path()+"middleware")
	// 	return c.Next()
	// })

	app.All("/*", func(c *fiber.Ctx) error {
		// var err error
		buf := bytes.NewBuffer(nil)

		// c.Method() is /echo/fsdf/sdf
		if _, err := buf.WriteString(fmt.Sprintf("%s %s\n", c.Method(), c.Path())); err != nil {
			return err
		}

		// GET /echo/fsdf/sdf, * = fsdf/sdf
		if _, err := buf.WriteString(fmt.Sprintf("* = %s\n", c.Params("*"))); err != nil {
			return err
		}

		for k, v := range c.GetReqHeaders() {
			if _, err := buf.WriteString(fmt.Sprintf("%s: %s\n", k, v)); err != nil {
				return err
			}
		}
		buf.WriteString("==============================\n")

		if _, err := buf.Write(c.Body()); err != nil {
			return err
		}

		_, err := c.Write(buf.Bytes())

		return err
	})

	return app
}
