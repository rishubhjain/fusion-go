package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Confinement pattern using goroutine-owned state
type request struct {
	data   string
	result chan string
}

func confinementHandler(reqChan chan request) {
	state := []string{}
	for req := range reqChan {
		state = append(state, req.data)
		req.result <- fmt.Sprintf("confined processed: %s", req.data)
	}
}

var results []string
var mu sync.Mutex

func worker(id int, jobs <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		time.Sleep(100 * time.Millisecond) // Simulate work
		mu.Lock()
		results = append(results, fmt.Sprintf("worker-%d processed %s", id, job))
		mu.Unlock()
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	app := fiber.New()
	app.Use(logger.New())
	app.Use(recover.New())

	// Set up goroutine confinement
	reqChan := make(chan request)
	go confinementHandler(reqChan)

	// Route using confinement
	app.Post("/confined", func(c *fiber.Ctx) error {
		var input struct {
			Data string `json:"data"`
		}
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid input")
		}

		respChan := make(chan string)
		reqChan <- request{data: input.Data, result: respChan}
		result := <-respChan
		return c.SendString(result)
	})

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

	// Rate-limited route
	app.Get("/limited", limiter.New(limiter.Config{
		Max:        2,
		Expiration: 3 * time.Second,
	}), func(c *fiber.Ctx) error {
		return c.SendString("Rate limited endpoint accessed")
	})

	// Concurrent GET using channel
	app.Get("/status", func(c *fiber.Ctx) error {
		statusCh := make(chan string)
		go func() {
			time.Sleep(200 * time.Millisecond)
			statusCh <- "OK"
		}()
		status := <-statusCh
		return c.SendString("System status: " + status)
	})

	// POST handler using worker pool and WaitGroup
	app.Post("/process", func(c *fiber.Ctx) error {
		var payload []string
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid JSON")
		}

		jobs := make(chan string, len(payload))
		var wg sync.WaitGroup

		for w := 1; w <= 3; w++ {
			wg.Add(1)
			go worker(w, jobs, &wg)
		}

		for _, item := range payload {
			jobs <- item
		}
		close(jobs)

		wg.Wait()
		return c.JSON(fiber.Map{"processed": results})
	})

	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}
