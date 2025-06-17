package main

import (
	"fmt"
	"log"
	"os"
	"encoding/json"
	"context"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/go-resty/resty/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
)

type InvokeModelWrapper struct {
	BedrockRuntimeClient *bedrockruntime.Client
}

// Updated request structure for Messages API
type ClaudeRequest struct {
	AnthropicVersion string    `json:"anthropic_version"`
	MaxTokens        int       `json:"max_tokens"`
	Messages         []Message `json:"messages"`
	Temperature      float64   `json:"temperature,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Updated response structure for Messages API
type ClaudeResponse struct {
	Content []struct {
		Text string `json:"text"`
		Type string `json:"type"`
	} `json:"content"`
	ID           string `json:"id"`
	Model        string `json:"model"`
	Role         string `json:"role"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Type         string `json:"type"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

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
		FullName string `json:"full_name"`
	} `json:"repository"`
}

type commentRequest struct {
	Content struct {
		Raw string `json:"raw"`
	} `json:"content"`
}

func (wrapper InvokeModelWrapper) InvokeClaude(ctx context.Context, prompt string) (string, error) {
	modelId := "anthropic.claude-3-haiku-20240307-v1:0"

	// Use Messages API format
	body, err := json.Marshal(ClaudeRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        200,
		Temperature:      0.5,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	})

	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	output, err := wrapper.BedrockRuntimeClient.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelId),
		ContentType: aws.String("application/json"),
		Body:        body,
	})

	if err != nil {
		return "", fmt.Errorf("failed to invoke model: %w", err)
	}

	var response ClaudeResponse
	if err := json.Unmarshal(output.Body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Content) > 0 {
		return response.Content[0].Text, nil
	}

	return "", fmt.Errorf("no content in response")
}

func main() {
	godotenv.Load()
	token := os.Getenv("BB_REPO_ACCESS_TOKEN")
	if token == "" {
		log.Fatal("BB_REPO_ACCESS_TOKEN environment variable is required")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(
			aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
				os.Getenv("AWS_ACCESS_KEY_ID"),
				os.Getenv("AWS_SECRET_ACCESS_KEY"),
				"",
			)),
		),
		config.WithRegion(os.Getenv("AWS_REGION")))

	if err != nil {
		log.Fatalf("unable to load aws sdk config %v", err)
	}

	bedrockClient := bedrockruntime.NewFromConfig(cfg)
	wrapper := InvokeModelWrapper{
		BedrockRuntimeClient: bedrockClient,
	}

	// Test invocation
	response, err := wrapper.InvokeClaude(context.TODO(), "Hello, world!")
	if err != nil {
		log.Printf("Test invocation failed: %v", err)
	} else {
		log.Printf("Test response: %s", response)
	}

	client := resty.New()

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			log.Printf("Fiber error: %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
		},
	})

	app.Use(logger.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Post("/bitbucket", func(c *fiber.Ctx) error {
		log.Printf("Received webhook payload: %+v", string(c.Body()))

		var payload webhookPayload
		if err := c.BodyParser(&payload); err != nil {
			log.Printf("Failed to parse webhook payload: %v", err)
			return c.Status(fiber.StatusBadRequest).SendString("Invalid JSON payload")
		}

		log.Printf("Received webhook payload: %+v", payload)

		repo := payload.Repository.FullName
		if repo == "" {
			log.Printf("Missing repository full_name in payload")
			return c.Status(fiber.StatusBadRequest).SendString("Missing repository information")
		}

		if payload.PullRequest.ID != 0 {
			claudeResponse, err := wrapper.InvokeClaude(c.Context(), `Create a PR comment for these changes, please check for vulnerabilities and bugs for this code package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
)

func vulnerableHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Command Injection
	userInput := r.URL.Query().Get("cmd")
	out, _ := exec.Command("sh", "-c", userInput).Output()
	fmt.Fprintf(w, "Command output:\n%s\n", out)

	// 2. SQL Injection
	username := r.URL.Query().Get("username")
	db, _ := sql.Open("mysql", "root:password@tcp(localhost:3306)/testdb")
	query := "SELECT password FROM users WHERE username = '" + username + "'"
	rows, _ := db.Query(query)
	defer rows.Close()
	for rows.Next() {
		var password string
		rows.Scan(&password)
		fmt.Fprintf(w, "Password for %s is %s\n", username, password)
	}

	// 3. Path Traversal
	file := r.URL.Query().Get("file")
	content, _ := ioutil.ReadFile("/var/www/" + file)
	fmt.Fprintf(w, "File content:\n%s\n", content)

	// 4. Hardcoded credentials
	if r.URL.Query().Get("auth") == "supersecret123" {
		fmt.Fprintf(w, "Authenticated as admin\n")
	}

	// 5. Information Disclosure
	envVars := os.Environ()
	fmt.Fprintf(w, "Environment variables:\n%v\n", envVars)
}

func main() {
	http.HandleFunc("/", vulnerableHandler)
	fmt.Println("Listening on :8080")
	http.ListenAndServe(":8080", nil)
}
`)
			if err != nil {
				log.Printf("Failed to invoke Claude: %v", err)
				return c.Status(fiber.StatusInternalServerError).SendString("Failed to generate comment")
			}

			commentURL := fmt.Sprintf(
				"https://api.bitbucket.org/2.0/repositories/%s/pullrequests/%d/comments",
				repo,
				payload.PullRequest.ID,
			)

			log.Printf("Posting comment to: %s", commentURL)

			comment := commentRequest{
				Content: struct {
					Raw string `json:"raw"`
				}{
					Raw: claudeResponse,
				},
			}

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

		commitListURL := fmt.Sprintf(
			"https://api.bitbucket.org/2.0/repositories/%s/pullrequests/%d/commits",
			repo,
			payload.PullRequest.ID,
		)

		resp, err := client.R().
			SetAuthToken(os.Getenv("BB_REPO_ACCESS_TOKEN")).
			SetHeader("Accept", "application/json").
			Get(commitListURL)

		if err != nil {
			log.Printf("Failed to fetch commits for PR #%d: %v", payload.PullRequest.ID, err)
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to fetch PR commits")
		}

		if resp.IsError() {
			log.Printf("Error from Bitbucket API for PR #%d: %s", payload.PullRequest.ID, resp.String())
			return c.Status(fiber.StatusInternalServerError).SendString("Bitbucket API error on PR commits")
		}

		log.Printf("Commits for PR #%d:\n%s", payload.PullRequest.ID, resp.String())

		return c.JSON(fiber.Map{
			"status":  "success",
			"message": "Webhook processed",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Starting server on port %s", port)
	log.Fatal(app.Listen(":" + port))
}
