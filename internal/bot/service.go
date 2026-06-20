package bot

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/radcolor/trishna-go/internal/config"
	"github.com/radcolor/trishna-go/internal/modules"
	"github.com/radcolor/trishna-go/internal/runtime"
)

type Service struct {
	cfg      config.Config
	registry *modules.Registry
	logger   *slog.Logger
	state    *runtime.State
	opts     Options
	services []modules.BackgroundService

	mu        sync.Mutex
	running   bool
	detail    string
	lastOK    *time.Time
	lastError string
}

func NewService(cfg config.Config, registry *modules.Registry, logger *slog.Logger, state *runtime.State, opts Options, services ...modules.BackgroundService) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if state == nil {
		state = runtime.NewState()
	}
	if opts.LogName == "" {
		opts.LogName = "discord"
	}
	return &Service{
		cfg:      cfg,
		registry: registry,
		logger:   logger,
		state:    state,
		opts:     opts,
		services: append([]modules.BackgroundService(nil), services...),
		detail:   "not started",
	}
}

func (s *Service) Name() string {
	if s.opts.LogName != "" {
		return s.opts.LogName
	}
	return "discord"
}

func (s *Service) Health() runtime.ServiceHealth {
	s.mu.Lock()
	defer s.mu.Unlock()
	return runtime.ServiceHealth{
		Name:      s.Name(),
		Running:   s.running,
		Detail:    s.detail,
		LastOK:    s.lastOK,
		LastError: s.lastError,
	}
}

func (s *Service) Run(ctx context.Context) error {
	if s.cfg.DiscordToken == "" {
		s.recordStopped("disabled (DISCORD_TRISHNA_TOKEN not set)", errors.New("DISCORD_TRISHNA_TOKEN is not set"))
		<-ctx.Done()
		return nil
	}

	app, err := New(s.cfg, s.registry, s.logger, s.state, s.opts, s.services...)
	if err != nil {
		s.recordStopped("failed to start", err)
		<-ctx.Done()
		return nil
	}
	defer app.Close(context.Background())

	s.recordRunning("connecting")
	err = app.Run(ctx)
	if err != nil && ctx.Err() == nil {
		s.recordStopped("stopped", err)
		<-ctx.Done()
		return nil
	}
	s.recordStopped("stopped", nil)
	return nil
}

func (s *Service) recordRunning(detail string) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
	s.detail = detail
	s.lastOK = &now
	s.lastError = ""
}

func (s *Service) recordStopped(detail string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	s.detail = detail
	if err != nil {
		s.lastError = err.Error()
	}
}
