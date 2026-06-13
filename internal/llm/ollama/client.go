package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/radcolor/trishna-go/internal/llm/prompt"
)

const defaultBaseURL = "http://127.0.0.1:11434"
const defaultTimeout = 60 * time.Second

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	baseURL    string
	model      string
	system     string
	httpClient *http.Client
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type chatResponse struct {
	Message Message `json:"message"`
}

func LoadSoul(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read soul file %q: %w", path, err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", fmt.Errorf("soul file %q is empty", path)
	}
	return content, nil
}

func NewClient(baseURL, model, systemPrompt string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if err := ValidateLocalhostBaseURL(baseURL); err != nil {
		return nil, err
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, fmt.Errorf("ollama model is required")
	}
	systemPrompt = strings.TrimSpace(systemPrompt)
	if systemPrompt == "" {
		return nil, fmt.Errorf("system prompt is required")
	}

	return &Client{
		baseURL: baseURL,
		model:   model,
		system:  prompt.AppendChatSecurity(systemPrompt),
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}, nil
}

func (c *Client) Soul() string {
	return c.system
}

func (c *Client) Chat(ctx context.Context, history []Message) (string, error) {
	messages := make([]Message, 0, 1+len(history))
	messages = append(messages, Message{Role: "system", Content: c.system})
	for _, item := range history {
		content := item.Content
		if item.Role == "user" {
			content = prompt.WrapUserContent(content)
		}
		messages = append(messages, Message{Role: item.Role, Content: content})
	}
	reply, err := c.completeMessages(ctx, messages)
	if err != nil {
		return "", err
	}
	return prompt.SanitizeDiscordOutput(reply), nil
}

func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	system = strings.TrimSpace(system)
	user = strings.TrimSpace(user)
	if system == "" {
		return "", fmt.Errorf("system prompt is required")
	}
	if user == "" {
		return "", fmt.Errorf("user prompt is required")
	}
	return c.completeMessages(ctx, []Message{
		{Role: "system", Content: system},
		{Role: "user", Content: prompt.WrapUserContent(user)},
	})
}

func (c *Client) completeMessages(ctx context.Context, messages []Message) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return "", fmt.Errorf("marshal ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read ollama response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("decode ollama response: %w", err)
	}

	reply := strings.TrimSpace(parsed.Message.Content)
	if reply == "" {
		return "", fmt.Errorf("ollama returned empty reply")
	}
	return reply, nil
}
