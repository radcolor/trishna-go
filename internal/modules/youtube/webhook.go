package youtube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const embedColorYouTube = 0xFF0000

type WebhookClient struct {
	httpClient *http.Client
}

func NewWebhookClient() *WebhookClient {
	return &WebhookClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *WebhookClient) PostVideo(webhookURL string, video Video) error {
	embed := webhookEmbed{
		Color: embedColorYouTube,
		Title: video.Title,
		URL:   video.URL,
		Image: &webhookImage{
			URL: video.BestThumbnail(),
		},
	}
	if description := truncateDescription(video.Description); description != "" {
		embed.Description = description
	}

	payload := webhookPayload{
		Embeds: []webhookEmbed{embed},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		detail := strings.TrimSpace(string(errorBody))
		if detail == "" {
			return fmt.Errorf("post webhook: %s", resp.Status)
		}
		return fmt.Errorf("post webhook: %s: %s", resp.Status, detail)
	}
	return nil
}

func isLiveStream(title string) bool {
	return strings.Contains(strings.ToLower(title), "live")
}

func truncateDescription(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return ""
	}

	lines := strings.Split(description, "\n")
	kept := make([]string, 0, 2)
	nonEmptyCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		nonEmptyCount++
		if len(kept) < 2 {
			kept = append(kept, trimmed)
		}
	}

	if len(kept) == 0 {
		return ""
	}

	result := strings.Join(kept, "\n")
	if nonEmptyCount > 2 {
		result += "..."
	}
	return result
}

func watchButtonLabel(live bool) string {
	if live {
		return "Watch Live"
	}
	return "Watch Video"
}

type webhookPayload struct {
	Embeds []webhookEmbed `json:"embeds"`
}

type webhookEmbed struct {
	Color       int           `json:"color"`
	Title       string        `json:"title"`
	URL         string        `json:"url"`
	Description string        `json:"description,omitempty"`
	Image       *webhookImage `json:"image,omitempty"`
}

type webhookImage struct {
	URL string `json:"url"`
}
