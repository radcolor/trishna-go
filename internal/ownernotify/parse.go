package ownernotify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const parseSystemPrompt = `You decide if the bot owner (human operator, offline) should receive a Discord DM about a chat user's message.
Reply with JSON only. No markdown, no explanation.

Schema:
{"notify_owner":false}
{"notify_owner":true,"category":"needs_owner","summary":"one line for the owner"}

Categories (use exactly one):
- needs_owner: user is looking for the owner, asking where they are, or seems to need their presence
- ask_contact: user wants the owner to call, text, or reach out
- emotional: user is upset, anxious, lonely, or needs more support than casual chat
- security: scams, OTP/password requests, hacked account, suspicious links, phishing, account safety
- important: other clearly important messages the owner should see soon

Rules:
- notify_owner true when the owner should be pinged in Discord DM.
- summary: short actionable note for the owner.
- Normal small talk, games, jokes, reminders, casual greetings → notify_owner false.
- When in doubt about emotional distress or safety, notify_owner true.`

type parseResponse struct {
	NotifyOwner bool   `json:"notify_owner"`
	Category    string `json:"category"`
	Summary     string `json:"summary"`
}

type LLMCompleter interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

type Parser struct {
	llm LLMCompleter
}

func NewParser(llm LLMCompleter) *Parser {
	return &Parser{llm: llm}
}

func (p *Parser) Parse(ctx context.Context, message string) (Result, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return Result{}, nil
	}

	raw, err := p.llm.Complete(ctx, parseSystemPrompt, "User message:\n"+message)
	if err != nil {
		return Result{}, err
	}

	parsed, err := decodeParseResponse(raw)
	if err != nil {
		return Result{}, err
	}
	if !parsed.NotifyOwner {
		return Result{Notify: false}, nil
	}

	category := normalizeCategory(parsed.Category)
	summary := strings.TrimSpace(parsed.Summary)
	if summary == "" {
		summary = message
		if len(summary) > 200 {
			summary = summary[:197] + "..."
		}
	}

	return Result{
		Notify:   true,
		Category: category,
		Summary:  summary,
	}, nil
}

func decodeParseResponse(raw string) (parseResponse, error) {
	raw = strings.TrimSpace(raw)
	raw = stripJSONFence(raw)

	var parsed parseResponse
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return parseResponse{}, fmt.Errorf("decode owner notify response: %w", err)
	}
	return parsed, nil
}

func stripJSONFence(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "```") {
		return raw
	}
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	return strings.TrimSpace(raw)
}

func normalizeCategory(raw string) Category {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(CategoryNeedsOwner), "missing_him":
		return CategoryNeedsOwner
	case string(CategoryAskContact), "ask_call":
		return CategoryAskContact
	case string(CategoryEmotional):
		return CategoryEmotional
	case string(CategorySecurity):
		return CategorySecurity
	default:
		return CategoryImportant
	}
}

func categoryLabel(category Category) string {
	switch category {
	case CategoryNeedsOwner:
		return "User looking for owner"
	case CategoryAskContact:
		return "Request to contact owner"
	case CategoryEmotional:
		return "Emotional — may need attention"
	case CategorySecurity:
		return "Security / safety concern"
	default:
		return "Important message"
	}
}
