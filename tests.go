package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Fiber API", func() {
	var app *fiber.App

	ginkgo.BeforeEach(func() {
		app = fiber.New()

		// Setup routes as in main.go
		app.Get("/", func(c *fiber.Ctx) error {
			return c.SendString("Hello World Openshift!")
		})

		app.Get("/status", func(c *fiber.Ctx) error {
			statusCh := make(chan string)
			go func() {
				statusCh <- "OK"
			}()
			status := <-statusCh
			return c.SendString("System status: " + status)
		})

		// Confinement route
		reqChan := make(chan request)
		go confinementHandler(reqChan)

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

		// Worker pool route
		app.Post("/process", func(c *fiber.Ctx) error {
			var payload []string
			if err := c.BodyParser(&payload); err != nil {
				return c.Status(fiber.StatusBadRequest).SendString("Invalid JSON")
			}
			jobs := make(chan string, len(payload))
			var wg sync.WaitGroup
			results = []string{}

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
	})

	ginkgo.AfterEach(func() {
		app.Shutdown()
	})

	ginkgo.It("should return hello world", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		resp, _ := app.Test(req)

		body, _ := ioutil.ReadAll(resp.Body)
		gomega.Expect(string(body)).To(gomega.Equal("Hello World Openshift!"))
	})

	ginkgo.It("should return system status", func() {
		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		resp, _ := app.Test(req)

		body, _ := ioutil.ReadAll(resp.Body)
		gomega.Expect(string(body)).To(gomega.Equal("System status: OK"))
	})

	ginkgo.It("should handle confinement", func() {
		payload := []byte(`{"data": "test-msg"}`)
		req := httptest.NewRequest(http.MethodPost, "/confined", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)

		body, _ := ioutil.ReadAll(resp.Body)
		gomega.Expect(string(body)).To(gomega.Equal("confined processed: test-msg"))
	})

	ginkgo.It("should process batch data", func() {
		data := []string{"task1", "task2", "task3"}
		jsonData, _ := json.Marshal(data)

		req := httptest.NewRequest(http.MethodPost, "/process", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)

		body, _ := ioutil.ReadAll(resp.Body)
		gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))
		gomega.Expect(string(body)).To(gomega.ContainSubstring("task1"))
	})
})

func TestFiberAPI(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Fiber API Suite")
}
