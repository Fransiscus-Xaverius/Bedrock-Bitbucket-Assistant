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
// type webhookPayload struct {
// 	PullRequest struct {
// 		ID int `json:"id"`
// 	} `json:"pullrequest"`

// 	Repository struct {
// 		FullName string `json:"full_name"` // e.g. "workspace/repo-slug"
// 	} `json:"repository"`
// }

type webhookPayload struct {
	Push struct {
		Changes []struct {
			Commits []struct {
				Hash    string `json:"hash"`
				Message string `json:"message"`
			} `json:"commits"`
		} `json:"changes"`
	} `json:"push"`

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
		var payload webhookPayload
		if err := c.BodyParser(&payload); err != nil {
			log.Printf("Failed to parse webhook payload: %v", err)
			return c.Status(fiber.StatusBadRequest).SendString("Invalid JSON payload")
		}

		repo := payload.Repository.FullName
		if repo == "" {
			log.Printf("Missing repository full_name in payload")
			return c.Status(fiber.StatusBadRequest).SendString("Missing repository information")
		}

		// If Pull Request exists in payload -> Comment
		if payload.PullRequest.ID != 0 {
			// Build comment URL
			commentURL := fmt.Sprintf(
				"https://api.bitbucket.org/2.0/repositories/%s/pullrequests/%d/comments",
				repo,
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

			// Send comment to Bitbucket API
			resp, err := client.R().
				SetAuthToken(os.Getenv("BB_REPO_ACCESS_TOKEN")).
				SetHeader("Content-Type", "application/json").
				SetHeader("Accept", "application/json").
				SetBody(comment).
				Post(commentURL)

			if err != nil {
				log.Printf("Request failed: %v", err)
				return c.Status(fiber.StatusInternalServerError).SendString("Failed to post comment")
			}

			if resp.IsError() {
				log.Printf("API error - Status: %s, Body: %s", resp.Status(), resp.String())
				return c.Status(fiber.StatusInternalServerError).SendString("Bitbucket API returned an error")
			}

			log.Printf("Successfully posted comment to PR #%d in %s", payload.PullRequest.ID, repo)
		}

		// If Push Commits exist -> Fetch & print changes
		if len(payload.Push.Changes) > 0 {
			for _, change := range payload.Push.Changes {
				for _, commit := range change.Commits {
					log.Printf("Processing commit: %s", commit.Hash)

					commitURL := fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s/commit/%s", repo, commit.Hash)

					resp, err := client.R().
						SetAuthToken(os.Getenv("BB_REPO_ACCESS_TOKEN")).
						SetHeader("Accept", "application/json").
						Get(commitURL)

					if err != nil {
						log.Printf("Failed to fetch commit %s: %v", commit.Hash, err)
						continue
					}

					if resp.IsError() {
						log.Printf("Error from Bitbucket API for commit %s: %s", commit.Hash, resp.String())
						continue
					}

					log.Printf("Commit %s details:\n%s", commit.Hash, resp.String())
				}
			}
		}

		return c.JSON(fiber.Map{
			"status":  "success",
			"message": "Webhook processed",
		})
	})


	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Starting server on port %s", port)
	log.Fatal(app.Listen(":" + port))
}