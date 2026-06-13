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
	"github.com/radcolor/trishna-go/internal/modules/youtube"
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

	registry, err := modules.NewRegistry(
		ping.New(),
	)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ytService, err := youtube.NewService(logger)
	if err != nil {
		return err
	}

	app, err := trishnabot.New(cfg, registry, logger, ytService)
	if err != nil {
		return err
	}
	defer app.Close(context.Background())

	return app.Run(ctx)
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
