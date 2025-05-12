package main

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	app := fiber.New()

	app.Get("/*", func(c *fiber.Ctx) error {
		path := c.Params("*")
		if path != "" {
			return c.SendString(fmt.Sprintf("Hello, %s!", path))
		}
		return c.SendString("Hello World this is awesome!")
	})

	if err := app.Listen(":" + port); err != nil {
		panic(err)
	}
}