package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/radcolor/trishna-go/internal/config"
	"github.com/radcolor/trishna-go/internal/modules"
	"github.com/radcolor/trishna-go/internal/runtime"
)

func TestServiceRetriesAppCreationFailure(t *testing.T) {
	restore := setServiceRetrySchedule(t, []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond})
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	attempts := 0
	service := NewService(config.Config{DiscordToken: "token"}, nil, testLogger(), nil, Options{})
	service.newApp = func(config.Config, *modules.Registry, *slog.Logger, *runtime.State, Options, ...modules.BackgroundService) (appRunner, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("discord unavailable")
		}
		cancel()
		return fakeAppRunner{}, nil
	}

	if err := service.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestServiceRetriesRunFailure(t *testing.T) {
	restore := setServiceRetrySchedule(t, []time.Duration{time.Millisecond, time.Millisecond})
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	attempts := 0
	service := NewService(config.Config{DiscordToken: "token"}, nil, testLogger(), nil, Options{})
	service.newApp = func(config.Config, *modules.Registry, *slog.Logger, *runtime.State, Options, ...modules.BackgroundService) (appRunner, error) {
		attempts++
		if attempts == 1 {
			return fakeAppRunner{runErr: errors.New("gateway unavailable")}, nil
		}
		cancel()
		return fakeAppRunner{}, nil
	}

	if err := service.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestServiceGivesUpAfterRetrySchedule(t *testing.T) {
	restore := setServiceRetrySchedule(t, []time.Duration{time.Millisecond, time.Millisecond})
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gaveUp := make(chan struct{})
	service := NewService(config.Config{DiscordToken: "token"}, nil, testLogger(), nil, Options{})
	service.newApp = func(config.Config, *modules.Registry, *slog.Logger, *runtime.State, Options, ...modules.BackgroundService) (appRunner, error) {
		return nil, errors.New("discord unavailable")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- service.Run(ctx)
	}()
	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if service.Health().Detail == "failed to start; gave up" {
				close(gaveUp)
				return
			}
		}
	}()

	select {
	case <-gaveUp:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for give up")
	}
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

type fakeAppRunner struct {
	runErr error
}

func (f fakeAppRunner) Run(context.Context) error {
	return f.runErr
}

func (f fakeAppRunner) Close(context.Context) {}

func setServiceRetrySchedule(t *testing.T, schedule []time.Duration) func() {
	t.Helper()
	old := discordServiceRetrySchedule
	discordServiceRetrySchedule = append([]time.Duration(nil), schedule...)
	return func() {
		discordServiceRetrySchedule = old
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
