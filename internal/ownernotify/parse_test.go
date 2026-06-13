package ownernotify

import (
	"context"
	"testing"
)

func TestDecodeParseResponse(t *testing.T) {
	got, err := decodeParseResponse(`{"notify_owner":true,"category":"ask_contact","summary":"User asked owner to call"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !got.NotifyOwner || got.Category != "ask_contact" {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestParserNeedsOwner(t *testing.T) {
	parser := NewParser(&mockLLM{
		reply: `{"notify_owner":true,"category":"needs_owner","summary":"User asked where the owner is"}`,
	})

	result, err := parser.Parse(t.Context(), "hey are you there?")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Notify || result.Category != CategoryNeedsOwner {
		t.Fatalf("unexpected %+v", result)
	}
}

func TestParserNotImportant(t *testing.T) {
	parser := NewParser(&mockLLM{reply: `{"notify_owner":false}`})

	result, err := parser.Parse(t.Context(), "hey")
	if err != nil {
		t.Fatal(err)
	}
	if result.Notify {
		t.Fatal("expected no notify")
	}
}

func TestNormalizeCategory(t *testing.T) {
	if normalizeCategory("unknown") != CategoryImportant {
		t.Fatal("expected important fallback")
	}
	if normalizeCategory("missing_him") != CategoryNeedsOwner {
		t.Fatal("expected legacy alias")
	}
}

type mockLLM struct {
	reply string
}

func (m *mockLLM) Complete(_ context.Context, _, _ string) (string, error) {
	return m.reply, nil
}
