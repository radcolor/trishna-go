package reminder

import (
	"context"
	"testing"
	"time"
)

func TestValidateDueAt(t *testing.T) {
	loc, err := LoadLocation()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, loc)

	t.Run("valid future", func(t *testing.T) {
		got, err := validateDueAt("2026-06-14T06:00:00+05:30", now)
		if err != nil {
			t.Fatal(err)
		}
		if got.IsZero() {
			t.Fatal("expected due time")
		}
	})

	t.Run("past", func(t *testing.T) {
		_, err := validateDueAt("2026-06-13T09:00:00+05:30", now)
		if err == nil {
			t.Fatal("expected error for past due_at")
		}
	})

	t.Run("too soon", func(t *testing.T) {
		_, err := validateDueAt(now.Format(time.RFC3339), now)
		if err == nil {
			t.Fatal("expected error for due_at too soon")
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := validateDueAt("tomorrow 6am", now)
		if err == nil {
			t.Fatal("expected parse error")
		}
	})
}

func TestDecodeParseResponse(t *testing.T) {
	t.Run("plain json", func(t *testing.T) {
		got, err := decodeParseResponse(`{"is_reminder":true,"event":"wake up","due_at":"2026-06-14T06:00:00+05:30"}`)
		if err != nil {
			t.Fatal(err)
		}
		if !got.IsReminder || got.Event != "wake up" {
			t.Fatalf("unexpected %+v", got)
		}
	})

	t.Run("fenced json", func(t *testing.T) {
		got, err := decodeParseResponse("```json\n{\"is_cancel\":true}\n```")
		if err != nil {
			t.Fatal(err)
		}
		if !got.IsCancel {
			t.Fatal("expected cancel")
		}
	})
}

func TestParserParse(t *testing.T) {
	loc, err := LoadLocation()
	if err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, 6, 13, 22, 0, 0, 0, loc)

	parser := &Parser{
		llm: &mockLLM{
			reply: `{"is_reminder":true,"event":"wake up early","due_at":"2026-06-14T06:00:00+05:30"}`,
		},
		location: loc,
		now:      func() time.Time { return fixedNow },
	}

	result, err := parser.Parse(t.Context(), "babe remind me to wake up early tmr at 6")
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != ParseSchedule {
		t.Fatalf("kind = %v", result.Kind)
	}
	if result.Event != "wake up early" {
		t.Fatalf("event = %q", result.Event)
	}
}

func TestParserNotReminder(t *testing.T) {
	loc, _ := LoadLocation()
	parser := &Parser{
		llm:      &mockLLM{reply: `{"is_reminder":false}`},
		location: loc,
		now:      time.Now,
	}

	result, err := parser.Parse(t.Context(), "hey there")
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != ParseNone {
		t.Fatalf("kind = %v", result.Kind)
	}
}

type mockLLM struct {
	reply string
	err   error
}

func (m *mockLLM) Complete(_ context.Context, _, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.reply, nil
}
