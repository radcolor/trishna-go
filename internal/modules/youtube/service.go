package youtube

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/radcolor/trishna-go/internal/runtime"
)

const (
	serviceName  = "youtube"
	pollInterval = 5 * time.Second
)

type LookupEnv func(key string) (string, bool)

type Service struct {
	logger        *slog.Logger
	feedClient    *FeedClient
	webhookClient *WebhookClient
	state         *State
	lookupEnv     LookupEnv

	mu          sync.Mutex
	running     bool
	watchCount  int
	lastPollAt  *time.Time
	lastError   string
	lastErrorAt *time.Time
}

func NewService(logger *slog.Logger) (*Service, error) {
	return NewServiceWithDeps(logger, NewFeedClient(), NewWebhookClient(), sysLookupEnv)
}

func NewServiceWithDeps(
	logger *slog.Logger,
	feedClient *FeedClient,
	webhookClient *WebhookClient,
	lookup LookupEnv,
) (*Service, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if feedClient == nil {
		feedClient = NewFeedClient()
	}
	if webhookClient == nil {
		webhookClient = NewWebhookClient()
	}
	if lookup == nil {
		lookup = sysLookupEnv
	}

	state, err := LoadState("")
	if err != nil {
		return nil, err
	}

	return &Service{
		logger:        logger,
		feedClient:    feedClient,
		webhookClient: webhookClient,
		state:         state,
		lookupEnv:     lookup,
	}, nil
}

func (s *Service) Name() string {
	return serviceName
}

func (s *Service) Health() runtime.ServiceHealth {
	s.mu.Lock()
	defer s.mu.Unlock()

	health := runtime.ServiceHealth{
		Name:      serviceName,
		Running:   s.running,
		LastOK:    s.lastPollAt,
		LastError: s.lastError,
	}

	switch {
	case s.watchCount == 0:
		health.Detail = "disabled (no webhooks configured)"
	case s.running:
		health.Detail = fmt.Sprintf("%d channel(s)", s.watchCount)
	default:
		health.Detail = "not started"
	}

	return health
}

func (s *Service) recordPollSuccess() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastPollAt = &now
	s.lastError = ""
	s.lastErrorAt = nil
}

func (s *Service) recordPollError(err error) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastError = err.Error()
	s.lastErrorAt = &now
}

func (s *Service) Run(ctx context.Context) error {
	watches, err := s.resolveWatches()
	if err != nil {
		return err
	}
	if len(watches) == 0 {
		s.logger.Warn("youtube service disabled: no channels with webhook URLs configured")
		<-ctx.Done()
		return nil
	}

	s.mu.Lock()
	s.running = true
	s.watchCount = len(watches)
	s.mu.Unlock()

	s.logger.Info("youtube service starting", slog.Int("channels", len(watches)))

	for _, watch := range watches {
		if err := s.pollChannel(ctx, watch); err != nil {
			s.recordPollError(err)
			s.logger.Error("youtube initial poll failed",
				slog.String("channel_id", watch.channel.ID),
				slog.String("channel_name", watch.channel.Name),
				slog.String("error", err.Error()),
			)
		} else {
			s.recordPollSuccess()
		}
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("youtube service stopping")
			return nil
		case <-ticker.C:
			for _, watch := range watches {
				if err := s.pollChannel(ctx, watch); err != nil {
					s.recordPollError(err)
					s.logger.Error("youtube poll failed",
						slog.String("channel_id", watch.channel.ID),
						slog.String("channel_name", watch.channel.Name),
						slog.String("error", err.Error()),
					)
				} else {
					s.recordPollSuccess()
				}
			}
		}
	}
}

type channelWatch struct {
	channel    Channel
	webhookURL string
}

func (s *Service) resolveWatches() ([]channelWatch, error) {
	var watches []channelWatch

	for _, channel := range Channels {
		rawURL, ok := s.lookupEnv(channel.WebhookEnvKey)
		webhookURL := strings.TrimSpace(rawURL)
		if !ok || webhookURL == "" {
			s.logger.Warn("youtube channel skipped: webhook URL not configured",
				slog.String("channel_id", channel.ID),
				slog.String("channel_name", channel.Name),
				slog.String("env_key", channel.WebhookEnvKey),
			)
			continue
		}
		watches = append(watches, channelWatch{
			channel:    channel,
			webhookURL: webhookURL,
		})
	}

	return watches, nil
}

func (s *Service) pollChannel(ctx context.Context, watch channelWatch) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	videos, err := s.feedClient.FetchVideos(watch.channel.ID)
	if err != nil {
		return err
	}
	if len(videos) == 0 {
		return nil
	}

	lastSeen := s.state.LastSeen(watch.channel.ID)
	newVideos, rebaseline := findNewVideos(videos, lastSeen)

	if rebaseline {
		s.logger.Info("youtube rebaselined channel feed",
			slog.String("channel_id", watch.channel.ID),
			slog.String("channel_name", watch.channel.Name),
			slog.String("video_id", videos[0].ID),
		)
		return s.state.SetLastSeen(watch.channel.ID, videos[0].ID)
	}

	if lastSeen == "" {
		s.logger.Info("youtube baselined channel feed",
			slog.String("channel_id", watch.channel.ID),
			slog.String("channel_name", watch.channel.Name),
			slog.String("video_id", videos[0].ID),
		)
		return s.state.SetLastSeen(watch.channel.ID, videos[0].ID)
	}

	if len(newVideos) == 0 {
		return nil
	}

	for i := len(newVideos) - 1; i >= 0; i-- {
		video := newVideos[i]
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.webhookClient.PostVideo(watch.webhookURL, video); err != nil {
			return fmt.Errorf("post video %s: %w", video.ID, err)
		}
		s.logger.Info("youtube posted update",
			slog.String("channel_id", watch.channel.ID),
			slog.String("channel_name", watch.channel.Name),
			slog.String("video_id", video.ID),
			slog.String("title", video.Title),
		)
	}

	return s.state.SetLastSeen(watch.channel.ID, videos[0].ID)
}

func findNewVideos(videos []Video, lastSeen string) (newVideos []Video, rebaseline bool) {
	if lastSeen == "" {
		return nil, false
	}

	foundLastSeen := false
	for _, video := range videos {
		if video.ID == lastSeen {
			foundLastSeen = true
			break
		}
		newVideos = append(newVideos, video)
	}

	if !foundLastSeen {
		return nil, true
	}
	return newVideos, false
}

func sysLookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}
