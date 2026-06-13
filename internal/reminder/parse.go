package reminder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const parseSystemPrompt = `You extract reminder intent from chat messages.
Reply with JSON only. No markdown, no explanation.

Timezone: Asia/Kolkata (IST, UTC+5:30).
Use RFC3339 with +05:30 offset for due_at.

Schema:
{"is_reminder":false}
{"is_reminder":true,"event":"short description","due_at":"2026-06-14T06:00:00+05:30"}
{"is_cancel":true}

Rules:
- is_reminder true only when the user clearly wants a future reminder with a specific or inferable time.
- event: short label for what to remind about (e.g. "wake up early").
- due_at: when to fire the reminder, in Asia/Kolkata.
- is_cancel true when the user wants to cancel pending reminders.
- If not a reminder or cancel request, return {"is_reminder":false}.`

type parseResponse struct {
	IsReminder bool   `json:"is_reminder"`
	IsCancel   bool   `json:"is_cancel"`
	Event      string `json:"event"`
	DueAt      string `json:"due_at"`
}

type ParseResult struct {
	Kind  ParseKind
	Event string
	DueAt time.Time
}

type ParseKind int

const (
	ParseNone ParseKind = iota
	ParseSchedule
	ParseCancel
)

type LLMCompleter interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

type Parser struct {
	llm      LLMCompleter
	location *time.Location
	now      func() time.Time
}

func NewParser(llm LLMCompleter, location *time.Location) *Parser {
	if location == nil {
		location = mustLoadLocation(LocationName)
	}
	return &Parser{
		llm:      llm,
		location: location,
		now:      time.Now,
	}
}

func (p *Parser) Parse(ctx context.Context, message string) (ParseResult, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return ParseResult{Kind: ParseNone}, nil
	}

	now := p.now().In(p.location)
	userPrompt := fmt.Sprintf(
		"Current time (Asia/Kolkata): %s\nTomorrow's date (Asia/Kolkata): %s\n\nUser message:\n%s",
		now.Format(time.RFC3339),
		now.Add(24*time.Hour).Format("2006-01-02"),
		message,
	)

	raw, err := p.llm.Complete(ctx, parseSystemPrompt, userPrompt)
	if err != nil {
		return ParseResult{}, err
	}

	parsed, err := decodeParseResponse(raw)
	if err != nil {
		return ParseResult{Kind: ParseNone}, err
	}

	if parsed.IsCancel {
		return ParseResult{Kind: ParseCancel}, nil
	}
	if !parsed.IsReminder {
		return ParseResult{Kind: ParseNone}, nil
	}

	event := strings.TrimSpace(parsed.Event)
	if event == "" {
		return ParseResult{Kind: ParseNone}, fmt.Errorf("reminder missing event")
	}

	dueAt, err := validateDueAt(parsed.DueAt, now)
	if err != nil {
		return ParseResult{Kind: ParseNone}, err
	}

	return ParseResult{
		Kind:  ParseSchedule,
		Event: event,
		DueAt: dueAt,
	}, nil
}

func decodeParseResponse(raw string) (parseResponse, error) {
	raw = strings.TrimSpace(raw)
	raw = stripJSONFence(raw)

	var parsed parseResponse
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return parseResponse{}, fmt.Errorf("decode parse response: %w", err)
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

func validateDueAt(raw string, now time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("reminder missing due_at")
	}

	dueAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse due_at: %w", err)
	}

	nowUTC := now.UTC()
	dueUTC := dueAt.UTC()
	minDue := nowUTC.Add(minLeadTime)
	maxDue := nowUTC.Add(maxFutureDuration)

	if !dueUTC.After(minDue) {
		return time.Time{}, fmt.Errorf("due_at is in the past or too soon")
	}
	if dueUTC.After(maxDue) {
		return time.Time{}, fmt.Errorf("due_at is too far in the future")
	}
	return dueUTC, nil
}

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("load location %q: %v", name, err))
	}
	return loc
}

func LoadLocation() (*time.Location, error) {
	return time.LoadLocation(LocationName)
}
