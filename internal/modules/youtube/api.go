package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultAPIBaseURL     = "https://www.googleapis.com/youtube/v3"
	defaultYouTubeWebURL  = "https://www.youtube.com"
	youtubeVideoIDCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
)

type APITokenSource interface {
	Token(ctx context.Context) (Token, error)
}

type APIClient struct {
	baseURL    string
	webURL     string
	httpClient *http.Client
	tokens     APITokenSource
}

type APIClientOptions struct {
	BaseURL     string
	WebURL      string
	HTTPClient  *http.Client
	TokenSource APITokenSource
}

type Broadcast struct {
	ID         string
	Title      string
	ChannelID  string
	LiveChatID string
	Tags       []string
}

type ChatMessage struct {
	ID              string
	AuthorID        string
	AuthorName      string
	Text            string
	Type            string
	IsChatOwner     bool
	IsChatModerator bool
}

type ChatMessagesResponse struct {
	Messages              []ChatMessage
	NextPageToken         string
	PollingIntervalMillis int
	OfflineAt             string
}

func NewAPIClient(opts APIClientOptions) *APIClient {
	baseURL := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}
	webURL := strings.TrimRight(strings.TrimSpace(opts.WebURL), "/")
	if webURL == "" {
		webURL = defaultYouTubeWebURL
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &APIClient{
		baseURL:    baseURL,
		webURL:     webURL,
		httpClient: httpClient,
		tokens:     opts.TokenSource,
	}
}

func (c *APIClient) ActiveBroadcast(ctx context.Context, ownerChannelIDs map[string]struct{}) (Broadcast, bool, error) {
	values := url.Values{}
	values.Set("part", "snippet,status")
	values.Set("broadcastType", "all")
	values.Set("mine", "true")
	values.Set("maxResults", "5")

	var raw struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title      string   `json:"title"`
				ChannelID  string   `json:"channelId"`
				LiveChatID string   `json:"liveChatId"`
				Tags       []string `json:"tags"`
			} `json:"snippet"`
			Status struct {
				LifeCycleStatus string `json:"lifeCycleStatus"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := c.getJSON(ctx, "/liveBroadcasts", values, &raw); err != nil {
		return Broadcast{}, false, err
	}

	for _, item := range raw.Items {
		if item.Status.LifeCycleStatus != "live" {
			continue
		}
		if item.Snippet.LiveChatID == "" {
			continue
		}
		if len(ownerChannelIDs) > 0 {
			if _, ok := ownerChannelIDs[item.Snippet.ChannelID]; !ok {
				continue
			}
		}
		return Broadcast{
			ID:         item.ID,
			Title:      item.Snippet.Title,
			ChannelID:  item.Snippet.ChannelID,
			LiveChatID: item.Snippet.LiveChatID,
			Tags:       item.Snippet.Tags,
		}, true, nil
	}
	return Broadcast{}, false, nil
}

func (c *APIClient) BroadcastByVideoID(ctx context.Context, videoID string) (Broadcast, bool, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return Broadcast{}, false, nil
	}

	values := url.Values{}
	values.Set("part", "snippet,liveStreamingDetails,status")
	values.Set("id", videoID)

	var raw struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title     string   `json:"title"`
				ChannelID string   `json:"channelId"`
				Tags      []string `json:"tags"`
			} `json:"snippet"`
			Status struct {
				PrivacyStatus string `json:"privacyStatus"`
			} `json:"status"`
			LiveStreamingDetails struct {
				ActiveLiveChatID string `json:"activeLiveChatId"`
				ActualEndTime    string `json:"actualEndTime"`
			} `json:"liveStreamingDetails"`
		} `json:"items"`
	}
	if err := c.getJSON(ctx, "/videos", values, &raw); err != nil {
		return Broadcast{}, false, err
	}

	for _, item := range raw.Items {
		if item.LiveStreamingDetails.ActualEndTime != "" {
			continue
		}
		if item.LiveStreamingDetails.ActiveLiveChatID == "" {
			continue
		}
		return Broadcast{
			ID:         item.ID,
			Title:      item.Snippet.Title,
			ChannelID:  item.Snippet.ChannelID,
			LiveChatID: item.LiveStreamingDetails.ActiveLiveChatID,
			Tags:       item.Snippet.Tags,
		}, true, nil
	}
	return Broadcast{}, false, nil
}

func (c *APIClient) ActiveBroadcastByChannelID(ctx context.Context, channelID string) (Broadcast, bool, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return Broadcast{}, false, nil
	}

	videoID, ok, err := c.LiveVideoIDByChannelID(ctx, channelID)
	if err != nil || !ok {
		return Broadcast{}, ok, err
	}
	return c.BroadcastByVideoID(ctx, videoID)
}

func (c *APIClient) LiveVideoIDByChannelID(ctx context.Context, channelID string) (string, bool, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return "", false, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.webURL+"/channel/"+url.PathEscape(channelID)+"/live", nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("User-Agent", "trishna-go")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", false, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       strings.TrimSpace(string(body)),
		}
	}

	videoID, ok := extractYouTubeVideoID(string(body))
	return videoID, ok, nil
}

func (c *APIClient) StreamChatMessages(ctx context.Context, liveChatID, pageToken string) (ChatMessagesResponse, error) {
	values := url.Values{}
	values.Set("liveChatId", liveChatID)
	values.Set("part", "id,snippet,authorDetails")
	values.Set("maxResults", "200")
	if pageToken != "" {
		values.Set("pageToken", pageToken)
	}

	resp, err := c.chatMessages(ctx, "/liveChat/messages:streamList", values)
	if err == nil {
		return resp, nil
	}

	var apiErr APIError
	if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusNotFound || apiErr.StatusCode == http.StatusMethodNotAllowed) {
		return c.chatMessages(ctx, "/liveChat/messages", values)
	}
	return ChatMessagesResponse{}, err
}

func (c *APIClient) SendChatMessage(ctx context.Context, liveChatID, text string) error {
	payload := map[string]any{
		"snippet": map[string]any{
			"liveChatId": liveChatID,
			"type":       "textMessageEvent",
			"textMessageDetails": map[string]string{
				"messageText": text,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	values := url.Values{}
	values.Set("part", "snippet")
	return c.doJSON(ctx, http.MethodPost, "/liveChat/messages", values, bytes.NewReader(body), nil)
}

func (c *APIClient) chatMessages(ctx context.Context, path string, values url.Values) (ChatMessagesResponse, error) {
	var raw struct {
		NextPageToken         string `json:"nextPageToken"`
		PollingIntervalMillis int    `json:"pollingIntervalMillis"`
		OfflineAt             string `json:"offlineAt"`
		Items                 []struct {
			ID      string `json:"id"`
			Snippet struct {
				Type               string `json:"type"`
				TextMessageDetails struct {
					MessageText string `json:"messageText"`
				} `json:"textMessageDetails"`
			} `json:"snippet"`
			AuthorDetails struct {
				ChannelID       string `json:"channelId"`
				DisplayName     string `json:"displayName"`
				IsChatOwner     bool   `json:"isChatOwner"`
				IsChatModerator bool   `json:"isChatModerator"`
			} `json:"authorDetails"`
		} `json:"items"`
	}
	if err := c.getJSON(ctx, path, values, &raw); err != nil {
		return ChatMessagesResponse{}, err
	}

	out := ChatMessagesResponse{
		NextPageToken:         raw.NextPageToken,
		PollingIntervalMillis: raw.PollingIntervalMillis,
		OfflineAt:             raw.OfflineAt,
		Messages:              make([]ChatMessage, 0, len(raw.Items)),
	}
	for _, item := range raw.Items {
		out.Messages = append(out.Messages, ChatMessage{
			ID:              item.ID,
			AuthorID:        item.AuthorDetails.ChannelID,
			AuthorName:      item.AuthorDetails.DisplayName,
			Text:            item.Snippet.TextMessageDetails.MessageText,
			Type:            item.Snippet.Type,
			IsChatOwner:     item.AuthorDetails.IsChatOwner,
			IsChatModerator: item.AuthorDetails.IsChatModerator,
		})
	}
	return out, nil
}

func (c *APIClient) getJSON(ctx context.Context, path string, values url.Values, dst any) error {
	return c.doJSON(ctx, http.MethodGet, path, values, nil, dst)
}

func (c *APIClient) doJSON(ctx context.Context, method, path string, values url.Values, body io.Reader, dst any) error {
	if c.tokens == nil {
		return errors.New("youtube token source is nil")
	}
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return err
	}

	u := c.baseURL + path
	if len(values) > 0 {
		u += "?" + values.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       strings.TrimSpace(string(respBody)),
		}
	}
	if dst == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, dst); err != nil {
		return fmt.Errorf("decode youtube response: %w", err)
	}
	return nil
}

func extractYouTubeVideoID(body string) (string, bool) {
	for _, marker := range []string{`"videoId":"`, `watch?v=`} {
		if videoID, ok := extractAfterMarker(body, marker); ok {
			return videoID, true
		}
	}
	return "", false
}

func extractAfterMarker(body, marker string) (string, bool) {
	idx := strings.Index(body, marker)
	if idx < 0 {
		return "", false
	}
	start := idx + len(marker)
	end := start
	for end < len(body) && strings.ContainsRune(youtubeVideoIDCharset, rune(body[end])) {
		end++
	}
	if end-start != 11 {
		return "", false
	}
	return body[start:end], true
}

type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e APIError) Error() string {
	if e.Body == "" {
		return "youtube api: " + e.Status
	}
	return "youtube api: " + e.Status + ": " + e.Body
}
