package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	app := fiber.New()
	app.Use(logger.New())
	app.Use(recover.New())

	// Main route
	app.Get("/*", func(c *fiber.Ctx) error {
		path := c.Params("*")
		if path != "" {
			return c.SendString(fmt.Sprintf("Hello, %s!", path))
		}
		return c.SendString("Hello World Openshift!")
	})

	// Liveness probe
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Readiness probe
	app.Get("/readyz", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Prometheus metrics
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}
