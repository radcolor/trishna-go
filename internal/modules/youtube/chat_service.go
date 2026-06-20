package youtube

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/radcolor/trishna-go/internal/runtime"
	"github.com/radcolor/trishna-go/internal/streambot"
)

const (
	EnvYouTubeChatEnabled     = "YOUTUBE_CHAT_ENABLED"
	EnvYouTubeClientID        = "YOUTUBE_CLIENT_ID"
	EnvYouTubeClientSecret    = "YOUTUBE_CLIENT_SECRET"
	EnvYouTubeTokenPath       = "YOUTUBE_TOKEN_PATH"
	EnvYouTubeOwnerChannelIDs = "YOUTUBE_OWNER_CHANNEL_IDS"
	EnvYouTubeLiveVideoID     = "YOUTUBE_LIVE_VIDEO_ID"
	EnvStreambotStatePath     = "STREAMBOT_STATE_PATH"
	EnvStreambotResponsesDir  = "STREAMBOT_RESPONSES_DIR"

	chatServiceName       = "youtube-chat"
	defaultBroadcastPause = 30 * time.Second
	defaultChatBackoff    = 10 * time.Second
	defaultChatPoll       = 5 * time.Second
	defaultAnnouncement   = 30 * time.Minute
)

type ChatServiceConfig struct {
	Enabled         bool
	ClientID        string
	ClientSecret    string
	TokenPath       string
	OwnerChannelIDs map[string]struct{}
	LiveVideoID     string
	StatePath       string
	ResponsesDir    string
}

type ChatService struct {
	cfg       ChatServiceConfig
	api       *APIClient
	bot       *streambot.Bot
	logger    *slog.Logger
	seen      map[string]struct{}
	seenOrder []string
	sent      map[string]time.Time

	mu          sync.Mutex
	running     bool
	detail      string
	lastOK      *time.Time
	lastError   string
	lastErrorAt *time.Time
}

func NewChatServiceFromEnv(logger *slog.Logger) *ChatService {
	cfg := ChatServiceConfig{
		Enabled:         parseBool(os.Getenv(EnvYouTubeChatEnabled)),
		ClientID:        strings.TrimSpace(os.Getenv(EnvYouTubeClientID)),
		ClientSecret:    strings.TrimSpace(os.Getenv(EnvYouTubeClientSecret)),
		TokenPath:       strings.TrimSpace(os.Getenv(EnvYouTubeTokenPath)),
		OwnerChannelIDs: parseStringSet(os.Getenv(EnvYouTubeOwnerChannelIDs)),
		LiveVideoID:     strings.TrimSpace(os.Getenv(EnvYouTubeLiveVideoID)),
		StatePath:       strings.TrimSpace(os.Getenv(EnvStreambotStatePath)),
		ResponsesDir:    strings.TrimSpace(os.Getenv(EnvStreambotResponsesDir)),
	}
	return NewChatService(cfg, logger, nil, nil)
}

func NewChatService(cfg ChatServiceConfig, logger *slog.Logger, api *APIClient, bot *streambot.Bot) *ChatService {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.TokenPath == "" {
		cfg.TokenPath = defaultTokenPath
	}
	if api == nil {
		api = NewAPIClient(APIClientOptions{
			TokenSource: NewTokenSource(OAuthConfig{
				ClientID:     cfg.ClientID,
				ClientSecret: cfg.ClientSecret,
				TokenPath:    cfg.TokenPath,
			}),
		})
	}
	if bot == nil {
		bot = streambot.New(streambot.Options{
			StateStore: streambot.NewStore(cfg.StatePath),
			Responses:  streambot.NewResponses(cfg.ResponsesDir),
		})
	}
	return &ChatService{
		cfg:    cfg,
		api:    api,
		bot:    bot,
		logger: logger,
		seen:   make(map[string]struct{}),
		sent:   make(map[string]time.Time),
	}
}

func (s *ChatService) Name() string {
	return chatServiceName
}

func (s *ChatService) Health() runtime.ServiceHealth {
	s.mu.Lock()
	defer s.mu.Unlock()

	detail := s.detail
	if detail == "" && !s.cfg.Enabled {
		detail = "disabled"
	}
	return runtime.ServiceHealth{
		Name:      chatServiceName,
		Running:   s.running,
		Detail:    detail,
		LastOK:    s.lastOK,
		LastError: s.lastError,
	}
}

func (s *ChatService) Run(ctx context.Context) error {
	if !s.cfg.Enabled {
		s.setStopped("disabled", nil)
		<-ctx.Done()
		return nil
	}
	if s.cfg.ClientID == "" {
		err := errors.New("YOUTUBE_CLIENT_ID is required")
		s.setStopped("misconfigured", err)
		<-ctx.Done()
		return nil
	}

	s.setRunning("starting")
	for {
		if err := ctx.Err(); err != nil {
			s.setStopped("stopped", nil)
			return nil
		}

		broadcast, ok, err := s.activeBroadcast(ctx)
		if err != nil {
			s.recordError(err)
			s.logger.Error("youtube chat active broadcast lookup failed", slog.String("error", err.Error()))
			sleep(ctx, defaultBroadcastPause)
			continue
		}
		if !ok {
			s.setRunning("waiting for active broadcast")
			sleep(ctx, defaultBroadcastPause)
			continue
		}

		s.logger.Info("youtube chat connected to broadcast",
			slog.String("broadcast_id", broadcast.ID),
			slog.String("title", broadcast.Title),
		)
		s.detectAndSetGame(ctx, broadcast)
		s.setRunning("connected: " + broadcast.Title)
		s.runChat(ctx, broadcast)
		sleep(ctx, defaultChatBackoff)
	}
}

func (s *ChatService) detectAndSetGame(ctx context.Context, broadcast Broadcast) {
	game := streambot.DetectGame(broadcast.Title, broadcast.Tags)
	if game == "" {
		return
	}
	if _, err := s.bot.SetGame(ctx, game); err != nil {
		s.recordError(err)
		s.logger.Error("streambot game auto-detect failed", slog.String("error", err.Error()))
		return
	}
	s.logger.Info("streambot game auto-detected",
		slog.String("game", game),
		slog.String("broadcast_id", broadcast.ID),
	)
}

func (s *ChatService) activeBroadcast(ctx context.Context) (Broadcast, bool, error) {
	if s.cfg.LiveVideoID != "" {
		return s.api.BroadcastByVideoID(ctx, s.cfg.LiveVideoID)
	}
	broadcast, ok, err := s.api.ActiveBroadcast(ctx, s.cfg.OwnerChannelIDs)
	if err == nil && ok {
		return broadcast, true, nil
	}
	if err != nil {
		s.logger.Warn("youtube owned broadcast lookup failed; trying public channel live pages", slog.String("error", err.Error()))
	}
	for channelID := range s.cfg.OwnerChannelIDs {
		broadcast, ok, fallbackErr := s.api.ActiveBroadcastByChannelID(ctx, channelID)
		if fallbackErr != nil {
			if err != nil {
				return Broadcast{}, false, err
			}
			return Broadcast{}, false, fallbackErr
		}
		if ok {
			return broadcast, true, nil
		}
	}
	return broadcast, ok, err
}

func (s *ChatService) runChat(ctx context.Context, broadcast Broadcast) {
	pageToken := ""
	baseline := true
	nextAnnouncement := time.Now()
	for {
		if ctx.Err() != nil {
			return
		}

		resp, err := s.api.StreamChatMessages(ctx, broadcast.LiveChatID, pageToken)
		if err != nil {
			s.recordError(err)
			s.logger.Error("youtube chat stream failed", slog.String("error", err.Error()))
			return
		}
		s.recordOK("connected: " + broadcast.Title)
		if resp.NextPageToken != "" {
			pageToken = resp.NextPageToken
		}
		if resp.OfflineAt != "" {
			s.setRunning("broadcast offline")
			return
		}

		if baseline {
			for _, msg := range resp.Messages {
				if msg.ID != "" {
					s.alreadySeen(msg.ID)
				}
			}
			baseline = false
		} else {
			for _, msg := range resp.Messages {
				s.handleMessage(ctx, broadcast.LiveChatID, msg)
			}
		}
		if !baseline && !time.Now().Before(nextAnnouncement) {
			s.sendAnnouncement(ctx, broadcast.LiveChatID)
			nextAnnouncement = time.Now().Add(defaultAnnouncement)
		}

		wait := time.Duration(resp.PollingIntervalMillis) * time.Millisecond
		if wait <= 0 {
			wait = defaultChatPoll
		}
		sleep(ctx, wait)
	}
}

func (s *ChatService) sendAnnouncement(ctx context.Context, liveChatID string) {
	resp, ok, err := s.bot.Announcement(ctx)
	if err != nil {
		s.recordError(err)
		s.logger.Error("streambot announcement failed", slog.String("error", err.Error()))
		return
	}
	if !ok || strings.TrimSpace(resp.Text) == "" {
		return
	}
	if err := s.api.SendChatMessage(ctx, liveChatID, resp.Text); err != nil {
		s.recordError(fmt.Errorf("send chat announcement: %w", err))
		s.logger.Error("youtube chat announcement failed", slog.String("error", err.Error()))
		return
	}
	s.recordSent(resp.Text)
	s.recordOK(s.detail)
}

func (s *ChatService) handleMessage(ctx context.Context, liveChatID string, msg ChatMessage) {
	if msg.ID != "" {
		if s.alreadySeen(msg.ID) {
			return
		}
	}
	if s.wasSentRecently(msg.Text) {
		return
	}
	if msg.Type != "textMessageEvent" || strings.TrimSpace(msg.Text) == "" {
		return
	}

	_, ownerID := s.cfg.OwnerChannelIDs[msg.AuthorID]
	resp, ok, err := s.bot.Handle(ctx, streambot.Message{
		Platform:   "youtube",
		AuthorID:   msg.AuthorID,
		AuthorName: msg.AuthorName,
		Text:       msg.Text,
		IsOwner:    ownerID || msg.IsChatOwner,
	})
	if err != nil {
		s.recordError(err)
		s.logger.Error("streambot command failed", slog.String("error", err.Error()))
		return
	}
	if !ok || strings.TrimSpace(resp.Text) == "" {
		return
	}
	s.logger.Info("streambot command reply",
		slog.String("author", msg.AuthorName),
		slog.String("command", msg.Text),
	)
	if err := s.api.SendChatMessage(ctx, liveChatID, resp.Text); err != nil {
		s.recordError(fmt.Errorf("send chat reply: %w", err))
		s.logger.Error("youtube chat reply failed", slog.String("error", err.Error()))
		return
	}
	s.recordSent(resp.Text)
	s.recordOK(s.detail)
}

func (s *ChatService) alreadySeen(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.seen[id]; ok {
		return true
	}
	s.seen[id] = struct{}{}
	s.seenOrder = append(s.seenOrder, id)
	if len(s.seenOrder) > 1000 {
		oldest := s.seenOrder[0]
		s.seenOrder = s.seenOrder[1:]
		delete(s.seen, oldest)
	}
	return false
}

func (s *ChatService) recordSent(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, at := range s.sent {
		if now.Sub(at) > time.Minute {
			delete(s.sent, key)
		}
	}
	s.sent[text] = now
}

func (s *ChatService) wasSentRecently(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	at, ok := s.sent[text]
	if !ok {
		return false
	}
	if now.Sub(at) > time.Minute {
		delete(s.sent, text)
		return false
	}
	return true
}

func (s *ChatService) setRunning(detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
	s.detail = detail
}

func (s *ChatService) setStopped(detail string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	s.detail = detail
	if err != nil {
		now := time.Now()
		s.lastError = err.Error()
		s.lastErrorAt = &now
	}
}

func (s *ChatService) recordOK(detail string) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
	if detail != "" {
		s.detail = detail
	}
	s.lastOK = &now
	s.lastError = ""
	s.lastErrorAt = nil
}

func (s *ChatService) recordError(err error) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
	s.lastError = err.Error()
	s.lastErrorAt = &now
}

func sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func parseStringSet(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out[part] = struct{}{}
		}
	}
	return out
}
