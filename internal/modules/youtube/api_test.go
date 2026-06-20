package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type staticTokenSource struct{}

func (staticTokenSource) Token(context.Context) (Token, error) {
	return Token{AccessToken: "access", Expiry: time.Now().Add(time.Hour)}, nil
}

func TestAPIClientActiveBroadcast(t *testing.T) {
	httpClient := roundTripClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/youtube/v3/liveBroadcasts" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("broadcastStatus"); got != "" {
			t.Fatalf("broadcastStatus = %q", got)
		}
		if got := r.URL.Query().Get("mine"); got != "true" {
			t.Fatalf("mine = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access" {
			t.Fatalf("authorization = %q", got)
		}
		return jsonResponse(http.StatusOK, map[string]any{
			"items": []map[string]any{{
				"id": "broadcast-1",
				"snippet": map[string]string{
					"title":      "Valorant",
					"channelId":  "owner-1",
					"liveChatId": "chat-1",
				},
				"status": map[string]string{
					"lifeCycleStatus": "live",
				},
			}},
		}), nil
	})

	client := NewAPIClient(APIClientOptions{
		BaseURL:     "https://youtube.test/youtube/v3",
		HTTPClient:  httpClient,
		TokenSource: staticTokenSource{},
	})

	broadcast, ok, err := client.ActiveBroadcast(context.Background(), map[string]struct{}{"owner-1": {}})
	if err != nil {
		t.Fatalf("active broadcast: %v", err)
	}
	if !ok {
		t.Fatal("expected active broadcast")
	}
	if broadcast.LiveChatID != "chat-1" || broadcast.Title != "Valorant" {
		t.Fatalf("broadcast = %#v", broadcast)
	}
}

func TestAPIClientStreamChatFallbackToList(t *testing.T) {
	var sawFallback bool
	httpClient := roundTripClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/youtube/v3/liveChat/messages:streamList":
			return textResponse(http.StatusNotFound, "not found"), nil
		case "/youtube/v3/liveChat/messages":
			sawFallback = true
			return jsonResponse(http.StatusOK, map[string]any{
				"nextPageToken":         "next",
				"pollingIntervalMillis": 1000,
				"items": []map[string]any{{
					"id": "msg-1",
					"snippet": map[string]any{
						"type": "textMessageEvent",
						"textMessageDetails": map[string]string{
							"messageText": "!ping",
						},
					},
					"authorDetails": map[string]any{
						"channelId":   "user-1",
						"displayName": "user",
					},
				}},
			}), nil
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
		return textResponse(http.StatusInternalServerError, "bad path"), nil
	})

	client := NewAPIClient(APIClientOptions{
		BaseURL:     "https://youtube.test/youtube/v3",
		HTTPClient:  httpClient,
		TokenSource: staticTokenSource{},
	})

	resp, err := client.StreamChatMessages(context.Background(), "chat-1", "")
	if err != nil {
		t.Fatalf("stream chat: %v", err)
	}
	if !sawFallback {
		t.Fatal("expected list fallback")
	}
	if len(resp.Messages) != 1 || resp.Messages[0].Text != "!ping" {
		t.Fatalf("messages = %#v", resp.Messages)
	}
}

func TestAPIClientBroadcastByVideoID(t *testing.T) {
	httpClient := roundTripClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/youtube/v3/videos" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("id"); got != "video-1" {
			t.Fatalf("id = %q", got)
		}
		if got := r.URL.Query().Get("part"); got != "snippet,liveStreamingDetails,status" {
			t.Fatalf("part = %q", got)
		}
		return jsonResponse(http.StatusOK, map[string]any{
			"items": []map[string]any{{
				"id": "video-1",
				"snippet": map[string]string{
					"title":     "Valorant",
					"channelId": "owner-1",
				},
				"liveStreamingDetails": map[string]string{
					"activeLiveChatId": "chat-1",
				},
			}},
		}), nil
	})

	client := NewAPIClient(APIClientOptions{
		BaseURL:     "https://youtube.test/youtube/v3",
		HTTPClient:  httpClient,
		TokenSource: staticTokenSource{},
	})

	broadcast, ok, err := client.BroadcastByVideoID(context.Background(), "video-1")
	if err != nil {
		t.Fatalf("broadcast by video id: %v", err)
	}
	if !ok {
		t.Fatal("expected broadcast")
	}
	if broadcast.LiveChatID != "chat-1" || broadcast.ChannelID != "owner-1" {
		t.Fatalf("broadcast = %#v", broadcast)
	}
}

func TestAPIClientActiveBroadcastByChannelID(t *testing.T) {
	httpClient := roundTripClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/channel/owner-1/live":
			return textResponse(http.StatusOK, `{"videoId":"video-abc12"}`), nil
		case "/youtube/v3/videos":
			if got := r.URL.Query().Get("id"); got != "video-abc12" {
				t.Fatalf("id = %q", got)
			}
			return jsonResponse(http.StatusOK, map[string]any{
				"items": []map[string]any{{
					"id": "video-abc12",
					"snippet": map[string]string{
						"title":     "Valorant",
						"channelId": "owner-1",
					},
					"liveStreamingDetails": map[string]string{
						"activeLiveChatId": "chat-1",
					},
				}},
			}), nil
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
		return textResponse(http.StatusInternalServerError, "bad path"), nil
	})

	client := NewAPIClient(APIClientOptions{
		BaseURL:     "https://youtube.test/youtube/v3",
		WebURL:      "https://youtube.test",
		HTTPClient:  httpClient,
		TokenSource: staticTokenSource{},
	})

	broadcast, ok, err := client.ActiveBroadcastByChannelID(context.Background(), "owner-1")
	if err != nil {
		t.Fatalf("active broadcast by channel id: %v", err)
	}
	if !ok {
		t.Fatal("expected broadcast")
	}
	if broadcast.ID != "video-abc12" || broadcast.LiveChatID != "chat-1" {
		t.Fatalf("broadcast = %#v", broadcast)
	}
}

func TestAPIClientSendChatMessage(t *testing.T) {
	httpClient := roundTripClient(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/youtube/v3/liveChat/messages" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("part"); got != "snippet" {
			t.Fatalf("part = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if !strings.Contains(toJSON(t, body), "hello") {
			t.Fatalf("body = %#v", body)
		}
		return jsonResponse(http.StatusOK, map[string]any{}), nil
	})

	client := NewAPIClient(APIClientOptions{
		BaseURL:     "https://youtube.test/youtube/v3",
		HTTPClient:  httpClient,
		TokenSource: staticTokenSource{},
	})
	if err := client.SendChatMessage(context.Background(), "chat-1", "hello"); err != nil {
		t.Fatalf("send chat: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func roundTripClient(f roundTripFunc) *http.Client {
	return &http.Client{Transport: f}
}

func jsonResponse(status int, v any) *http.Response {
	body, _ := json.Marshal(v)
	return textResponse(status, string(body))
}

func textResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

func toJSON(t *testing.T, v any) string {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
