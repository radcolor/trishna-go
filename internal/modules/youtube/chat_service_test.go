package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/radcolor/trishna-go/internal/streambot"
)

func TestChatServiceHandleMessageSendsCommandReply(t *testing.T) {
	var posts int
	httpClient := roundTripClient(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		posts++
		return jsonResponse(http.StatusOK, map[string]any{}), nil
	})

	api := NewAPIClient(APIClientOptions{
		BaseURL:     "https://youtube.test/youtube/v3",
		HTTPClient:  httpClient,
		TokenSource: staticTokenSource{},
	})
	bot := streambot.New(streambot.Options{
		StateStore:      streambot.NewStore(filepath.Join(t.TempDir(), "state.json")),
		CommandCooldown: time.Nanosecond,
	})
	service := NewChatService(ChatServiceConfig{Enabled: true}, nil, api, bot)

	service.handleMessage(context.Background(), "chat-1", ChatMessage{
		ID:          "msg-1",
		Type:        "textMessageEvent",
		AuthorID:    "user-1",
		Text:        "!ping",
		IsChatOwner: true,
	})

	if posts != 1 {
		t.Fatalf("posts = %d", posts)
	}
}

func TestChatServiceOwnerCanSetGame(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	httpClient := roundTripClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, map[string]any{}), nil
	})

	api := NewAPIClient(APIClientOptions{
		BaseURL:     "https://youtube.test/youtube/v3",
		HTTPClient:  httpClient,
		TokenSource: staticTokenSource{},
	})
	bot := streambot.New(streambot.Options{
		StateStore:      streambot.NewStore(statePath),
		CommandCooldown: time.Nanosecond,
	})
	service := NewChatService(ChatServiceConfig{
		Enabled:         true,
		OwnerChannelIDs: map[string]struct{}{"owner-1": {}},
	}, nil, api, bot)

	service.handleMessage(context.Background(), "chat-1", ChatMessage{
		ID:       "msg-1",
		Type:     "textMessageEvent",
		AuthorID: "owner-1",
		Text:     "!setgame valorant",
	})

	body, err := json.Marshal(mustLoadStreambotState(t, statePath))
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if string(body) != `{"current_game":"valorant"}` {
		t.Fatalf("state = %s", body)
	}
}

func TestChatServiceIgnoresRecentlySentText(t *testing.T) {
	var posts int
	httpClient := roundTripClient(func(r *http.Request) (*http.Response, error) {
		posts++
		return jsonResponse(http.StatusOK, map[string]any{}), nil
	})
	api := NewAPIClient(APIClientOptions{
		BaseURL:     "https://youtube.test/youtube/v3",
		HTTPClient:  httpClient,
		TokenSource: staticTokenSource{},
	})
	bot := streambot.New(streambot.Options{
		StateStore:      streambot.NewStore(filepath.Join(t.TempDir(), "state.json")),
		CommandCooldown: time.Nanosecond,
	})
	service := NewChatService(ChatServiceConfig{Enabled: true}, nil, api, bot)
	service.recordSent("!ping")

	service.handleMessage(context.Background(), "chat-1", ChatMessage{
		ID:       "msg-1",
		Type:     "textMessageEvent",
		AuthorID: "owner-1",
		Text:     "!ping",
	})

	if posts != 0 {
		t.Fatalf("posts = %d", posts)
	}
}

func mustLoadStreambotState(t *testing.T, path string) streambot.State {
	t.Helper()
	state, err := streambot.NewStore(path).Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	return state
}
