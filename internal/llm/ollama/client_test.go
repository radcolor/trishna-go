package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		if req.Options["num_predict"] == nil {
			t.Fatal("expected num_predict option")
		}
		if len(req.Messages) != 2 {
			t.Fatalf("messages = %+v", req.Messages)
		}
		if req.Messages[0].Role != "system" || !strings.Contains(req.Messages[0].Content, "Security (always enforced)") {
			t.Fatalf("system = %+v", req.Messages[0])
		}
		if req.Messages[1].Role != "user" || !strings.Contains(req.Messages[1].Content, "BEGIN_USER_MESSAGE") {
			t.Fatalf("user = %+v", req.Messages[1])
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

func TestClientChatContinuesLengthResponse(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		calls++
		switch calls {
		case 1:
			if len(req.Messages) != 2 {
				t.Fatalf("first messages = %+v", req.Messages)
			}
			_ = json.NewEncoder(w).Encode(chatResponse{
				Message:    Message{Role: "assistant", Content: "technical details start,"},
				DoneReason: "length",
			})
		case 2:
			if len(req.Messages) != 4 {
				t.Fatalf("second messages = %+v", req.Messages)
			}
			if req.Messages[2].Role != "assistant" || req.Messages[2].Content != "technical details start," {
				t.Fatalf("assistant continuation context = %+v", req.Messages[2])
			}
			if req.Messages[3].Role != "user" || !strings.Contains(req.Messages[3].Content, "Continue exactly") {
				t.Fatalf("continuation prompt = %+v", req.Messages[3])
			}
			_ = json.NewEncoder(w).Encode(chatResponse{
				Message: Message{Role: "assistant", Content: "technical details finish."},
			})
		default:
			t.Fatalf("unexpected call %d", calls)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "gemma4:e2b", "you are shawn")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	reply, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "explain"}})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if reply != "technical details start,\ntechnical details finish." {
		t.Fatalf("reply = %q", reply)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}
