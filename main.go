package main

import (
	"fmt"
	"log"
	"os"

	"github.com/go-resty/resty/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
)

// webhookPayload models the parts we need from Bitbucket webhook
type webhookPayload struct {
	PullRequest struct {
		ID int `json:"id"`
	} `json:"pullrequest"`

	Repository struct {
		FullName string `json:"full_name"` // e.g. "workspace/repo-slug"
	} `json:"repository"`
}

// commentRequest represents the structure for Bitbucket comment API
type commentRequest struct {
	Content struct {
		Raw string `json:"raw"`
	} `json:"content"`
}

func main() {
	// Get token from environment
	godotenv.Load() 
	token := os.Getenv("BB_REPO_ACCESS_TOKEN")
	if token == "" {
		log.Fatal("BB_REPO_ACCESS_TOKEN environment variable is required")
	}

	// Initialize Resty client
	client := resty.New()

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			log.Printf("Fiber error: %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
		},
	})

	// Add logging middleware
	app.Use(logger.New())

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Bitbucket webhook handler
	app.Post("/bitbucket", func(c *fiber.Ctx) error {
		// Parse webhook payload
		var payload webhookPayload
		if err := c.BodyParser(&payload); err != nil {
			log.Printf("Failed to parse webhook payload: %v", err)
			return c.Status(fiber.StatusBadRequest).SendString("Invalid JSON payload")
		}

		// Validate payload
		if payload.Repository.FullName == "" {
			log.Printf("Missing repository full_name in payload")
			return c.Status(fiber.StatusBadRequest).SendString("Missing repository information")
		}

		if payload.PullRequest.ID == 0 {
			log.Printf("Missing or invalid pull request ID in payload")
			return c.Status(fiber.StatusBadRequest).SendString("Missing pull request information")
		}

		// Build comment URL
		commentURL := fmt.Sprintf(
			"https://api.bitbucket.org/2.0/repositories/%s/pullrequests/%d/comments",
			payload.Repository.FullName,
			payload.PullRequest.ID,
		)

		log.Printf("Posting comment to: %s", commentURL)

		// Prepare comment body
		comment := commentRequest{
			Content: struct {
				Raw string `json:"raw"`
			}{
				Raw: "LGTM! 🚀",
			},
		}

		// Make API request
		resp, err := client.R().
			SetAuthToken(token).
			SetHeader("Content-Type", "application/json").
			SetHeader("Accept", "application/json").
			SetBody(comment).
			Post(commentURL)

		if err != nil {
			log.Printf("Request failed: %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to post comment")
		}

		// Check response
		if resp.IsError() {
			log.Printf("API error - Status: %s, Body: %s", resp.Status(), resp.String())
			return c.Status(fiber.StatusInternalServerError).SendString("Bitbucket API returned an error")
		}

		log.Printf("Successfully posted comment to PR #%d in %s", payload.PullRequest.ID, payload.Repository.FullName)
		return c.JSON(fiber.Map{
			"status":  "success",
			"message": "Comment posted successfully",
		})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Starting server on port %s", port)
	log.Fatal(app.Listen(":" + port))
}