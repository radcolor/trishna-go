package streambot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleBasicCommands(t *testing.T) {
	now := time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC)
	bot := New(Options{
		StateStore:      NewStore(filepath.Join(t.TempDir(), "state.json")),
		StartedAt:       now.Add(-90 * time.Second),
		CommandCooldown: time.Nanosecond,
		Now:             func() time.Time { return now },
	})

	resp, ok, err := bot.Handle(context.Background(), Message{Text: "!ping", IsOwner: true})
	if err != nil {
		t.Fatalf("handle ping: %v", err)
	}
	if !ok || resp.Text != "pong" {
		t.Fatalf("ping response = %#v, %v", resp, ok)
	}

	resp, ok, err = bot.Handle(context.Background(), Message{Text: "!uptime", IsOwner: true})
	if err != nil {
		t.Fatalf("handle uptime: %v", err)
	}
	if !ok || resp.Text != "Stream bot uptime: 1m30s" {
		t.Fatalf("uptime response = %#v, %v", resp, ok)
	}
}

func TestOwnerCanSetGame(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	bot := New(Options{StateStore: NewStore(path), CommandCooldown: time.Nanosecond})

	resp, ok, err := bot.Handle(context.Background(), Message{Text: "!setgame generic", IsOwner: true})
	if err != nil {
		t.Fatalf("handle setgame: %v", err)
	}
	if !ok || resp.Text != "Current game set to Generic." {
		t.Fatalf("setgame response = %#v, %v", resp, ok)
	}

	state, err := NewStore(path).Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.CurrentGame != "generic" {
		t.Fatalf("current game = %q", state.CurrentGame)
	}
}

func TestDetectGame(t *testing.T) {
	tests := []struct {
		name  string
		title string
		tags  []string
		want  string
	}{
		{name: "valorant title", title: "Ranked Valorant grind", want: "valorant"},
		{name: "sky hashtag", title: "Cozy stream", tags: []string{"skycotl"}, want: "sky"},
		{name: "generic fallback", title: "Late night stream", want: "generic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectGame(tt.title, tt.tags); got != tt.want {
				t.Fatalf("DetectGame() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestViewerCommands(t *testing.T) {
	bot := New(Options{
		StateStore:      NewStore(filepath.Join(t.TempDir(), "state.json")),
		CommandCooldown: time.Nanosecond,
	})

	for _, command := range []string{"!specs", "!crosshair", "!isekai"} {
		resp, ok, err := bot.Handle(context.Background(), Message{Text: command})
		if err != nil {
			t.Fatalf("handle %s: %v", command, err)
		}
		if !ok || resp.Text == "" {
			t.Fatalf("%s response = %#v, %v", command, resp, ok)
		}
	}
}

func TestNonOwnerPingIgnored(t *testing.T) {
	bot := New(Options{StateStore: NewStore(filepath.Join(t.TempDir(), "state.json"))})

	resp, ok, err := bot.Handle(context.Background(), Message{Text: "!ping"})
	if err != nil {
		t.Fatalf("handle ping: %v", err)
	}
	if ok || resp.Text != "" {
		t.Fatalf("non-owner ping response = %#v, %v", resp, ok)
	}
}

func TestNonOwnerSetGameIgnored(t *testing.T) {
	bot := New(Options{StateStore: NewStore(filepath.Join(t.TempDir(), "state.json"))})

	resp, ok, err := bot.Handle(context.Background(), Message{Text: "!setgame valorant"})
	if err != nil {
		t.Fatalf("handle setgame: %v", err)
	}
	if ok || resp.Text != "" {
		t.Fatalf("non-owner response = %#v, %v", resp, ok)
	}
}

func TestAnnouncementUsesCurrentGame(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := NewStore(path)
	if err := store.Save(State{CurrentGame: "sky"}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	bot := New(Options{StateStore: store})

	resp, ok, err := bot.Announcement(context.Background())
	if err != nil {
		t.Fatalf("announcement: %v", err)
	}
	if !ok || resp.Text == "" || !strings.Contains(resp.Text, "Sky Companion Overlay") {
		t.Fatalf("announcement response = %#v, %v", resp, ok)
	}
}

func TestResponseFileOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "socials.txt"), []byte("youtube.example\n"), 0o600); err != nil {
		t.Fatalf("write response: %v", err)
	}
	bot := New(Options{
		StateStore:      NewStore(filepath.Join(t.TempDir(), "state.json")),
		Responses:       NewResponses(dir),
		CommandCooldown: time.Nanosecond,
	})

	resp, ok, err := bot.Handle(context.Background(), Message{Text: "!socials"})
	if err != nil {
		t.Fatalf("handle socials: %v", err)
	}
	if !ok || resp.Text != "youtube.example" {
		t.Fatalf("socials response = %#v, %v", resp, ok)
	}
}

func TestCooldownSuppressesRepeatedCommand(t *testing.T) {
	now := time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC)
	bot := New(Options{
		StateStore:      NewStore(filepath.Join(t.TempDir(), "state.json")),
		CommandCooldown: time.Minute,
		Now:             func() time.Time { return now },
	})

	if _, ok, err := bot.Handle(context.Background(), Message{Text: "!specs"}); err != nil || !ok {
		t.Fatalf("first specs ok = %v err = %v", ok, err)
	}
	if resp, ok, err := bot.Handle(context.Background(), Message{Text: "!specs"}); err != nil || ok || resp.Text != "" {
		t.Fatalf("second specs response = %#v ok = %v err = %v", resp, ok, err)
	}
}
