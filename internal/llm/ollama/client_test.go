package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSoul(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SOUL.md")
	if err := os.WriteFile(path, []byte("  hello soul  \n"), 0o600); err != nil {
		t.Fatalf("write soul: %v", err)
	}

	got, err := LoadSoul(path)
	if err != nil {
		t.Fatalf("load soul: %v", err)
	}
	if got != "hello soul" {
		t.Fatalf("soul = %q", got)
	}
}

func TestClientChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "gemma4:e2b" {
			t.Fatalf("model = %q", req.Model)
		}
		if len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Content != "hey" {
			t.Fatalf("messages = %+v", req.Messages)
		}

		_ = json.NewEncoder(w).Encode(chatResponse{
			Message: Message{Role: "assistant", Content: "hey love"},
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "gemma4:e2b", "you are shawn")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	reply, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hey"}})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if reply != "hey love" {
		t.Fatalf("reply = %q", reply)
	}
}
