package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

func main() {
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Bedrock PoC API")
	})

	app.Post("/bitbucket", func(c *fiber.Ctx) error {
		var payload map[string]interface{}

		if err := c.BodyParser(&payload); 
		err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid JSON")
		}

		log.Infof("Received Bitbucket webhook: %+v", payload)

		return c.SendStatus(fiber.StatusOK)
	})

	app.Listen(":3000")
}