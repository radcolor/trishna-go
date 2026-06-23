package bot

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	disgobot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"

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
	newApp   appFactory

	mu        sync.Mutex
	running   bool
	detail    string
	lastOK    *time.Time
	lastError string
}

type appRunner interface {
	Run(context.Context) error
	Close(context.Context)
}

type appFactory func(config.Config, *modules.Registry, *slog.Logger, *runtime.State, Options, ...modules.BackgroundService) (appRunner, error)

var discordServiceRetrySchedule = append([]time.Duration(nil), runtime.DefaultRetrySchedule...)

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
		newApp: func(cfg config.Config, registry *modules.Registry, logger *slog.Logger, state *runtime.State, opts Options, services ...modules.BackgroundService) (appRunner, error) {
			return New(cfg, registry, logger, state, opts, services...)
		},
		detail: "not started",
	}
}

func (s *Service) Name() string {
	if s.opts.HealthName != "" {
		return s.opts.HealthName
	}
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

	var failures atomic.Int32
	opts := s.opts
	opts.ExtraListeners = append([]disgobot.EventListener{
		disgobot.NewListenerFunc(func(*events.Ready) {
			failures.Store(0)
			s.recordRunning("connected")
		}),
	}, opts.ExtraListeners...)

	for {
		app, err := s.newApp(s.cfg, s.registry, s.logger, s.state, opts, s.services...)
		if err != nil {
			attempt := int(failures.Add(1))
			if !s.handleRetry(ctx, attempt, "failed to start", err) {
				return nil
			}
			continue
		}

		s.recordRunning("connecting")
		err = app.Run(ctx)
		app.Close(context.Background())
		if ctx.Err() != nil {
			s.recordStopped("stopped", nil)
			return nil
		}
		if err != nil {
			attempt := int(failures.Add(1))
			if !s.handleRetry(ctx, attempt, "stopped", err) {
				return nil
			}
		} else {
			attempt := int(failures.Add(1))
			if !s.handleRetry(ctx, attempt, "stopped without error", nil) {
				return nil
			}
		}
	}
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

func (s *Service) handleRetry(ctx context.Context, attempt int, detail string, err error) bool {
	delay, ok := runtime.RetryDelay(discordServiceRetrySchedule, attempt)
	if !ok {
		s.recordStopped(detail+"; gave up", err)
		args := []any{
			slog.String("service", s.Name()),
			slog.Int("attempts", attempt-1),
		}
		if err != nil {
			args = append(args, slog.String("error", err.Error()))
		}
		s.logger.Error("discord service gave up; manual restart required", args...)
		<-ctx.Done()
		s.recordStopped("stopped", nil)
		return false
	}

	s.recordStopped(detail+"; retrying", err)
	args := []any{
		slog.String("service", s.Name()),
		slog.Int("attempt", attempt),
		slog.Int("max_attempts", len(discordServiceRetrySchedule)),
		slog.Duration("retry_in", delay),
	}
	if err != nil {
		args = append(args, slog.String("error", err.Error()))
	}
	s.logger.Warn("discord service retry scheduled", args...)

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		s.recordStopped("stopped", nil)
		return false
	case <-timer.C:
		return true
	}
}
