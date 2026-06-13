package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	trishnabot "github.com/radcolor/trishna-go/internal/bot"
	"github.com/radcolor/trishna-go/internal/config"
	"github.com/radcolor/trishna-go/internal/modules"
	"github.com/radcolor/trishna-go/internal/modules/ping"
	"github.com/radcolor/trishna-go/internal/modules/status"
	"github.com/radcolor/trishna-go/internal/modules/youtube"
	"github.com/radcolor/trishna-go/internal/runtime"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	runtimeState := runtime.NewState()

	ytService, err := youtube.NewService(logger)
	if err != nil {
		return err
	}

	allowlist, err := status.ParseAllowlist(os.Getenv(config.EnvStatusAllowedUserIDs))
	if err != nil {
		return err
	}

	registry, err := modules.NewRegistry(
		ping.New(),
		status.New(status.Deps{
			Runtime:   runtimeState,
			Services:  []runtime.HealthReporter{ytService},
			Allowlist: allowlist,
		}),
	)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := trishnabot.New(cfg, registry, logger, runtimeState, ytService)
	if err != nil {
		return err
	}
	defer app.Close(context.Background())

	return app.Run(ctx)
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
