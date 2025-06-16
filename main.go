package main

import (
  "fmt"
  "log"
  "os"

  "github.com/go-resty/resty/v2"
  "github.com/gofiber/fiber/v2"
)

// webhookPayload models just the parts we need
type webhookPayload struct {
  PullRequest struct {
    ID int `json:"id"`
  } `json:"pullrequest"`

  Repository struct {
    FullName string `json:"full_name"` // e.g. "workspace/repo-slug"
  } `json:"repository"`
}

func main() {
  // initialize Resty client once
  client := resty.New().
    SetAuthScheme("Bearer").
    SetAuthToken(os.Getenv("BB_REPO_ACCESS_TOKEN")).
    SetHeader("Content-Type", "application/json")

  app := fiber.New()

  app.Post("/bitbucket", func(c *fiber.Ctx) error {
    // 1) parse payload
    var payload webhookPayload
    if err := c.BodyParser(&payload); err != nil {
      return c.Status(fiber.StatusBadRequest).SendString("invalid JSON")
    }

    // 2) build the comment‐URL
    commentURL := fmt.Sprintf(
      "https://api.bitbucket.org/2.0/repositories/%s/pullrequests/%d/comments",
      payload.Repository.FullName,
      payload.PullRequest.ID,
    )

    // 3) prepare the static comment body
    body := map[string]interface{}{
      "content": map[string]string{"raw": "LGTM!"},
    }

    // 4) send it via Resty (Bearer token already applied on client)
    resp, err := client.R().
      SetBody(body).
      Post(commentURL)

    if err != nil {
      log.Printf("request error: %v", err)
      return c.Status(fiber.StatusInternalServerError).SendString("failed to post comment")
    }
    if resp.IsError() {
      log.Printf("bad status: %s – %s", resp.Status(), resp.String())
      return c.Status(fiber.StatusInternalServerError).SendString("error from Bitbucket API")
    }

    log.Println("Posted: LGTM!")
    return c.SendStatus(fiber.StatusOK)
  })

  log.Fatal(app.Listen(":3000"))
}
