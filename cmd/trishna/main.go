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
	"github.com/radcolor/trishna-go/internal/llm/ollama"
	"github.com/radcolor/trishna-go/internal/modules"
	"github.com/radcolor/trishna-go/internal/modules/ping"
	"github.com/radcolor/trishna-go/internal/modules/status"
	"github.com/radcolor/trishna-go/internal/modules/youtube"
	"github.com/radcolor/trishna-go/internal/runtime"
	"github.com/radcolor/trishna-go/internal/shawnb/monitor"
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
	if len(allowlist) == 0 {
		return fmt.Errorf("%s is required", config.EnvStatusAllowedUserIDs)
	}

	shawnbMonitor := monitor.New(os.Getenv(config.EnvShawnbHeartbeatPath))
	ollamaMonitor := ollama.NewMonitor(os.Getenv(config.EnvOllamaBaseURL), os.Getenv(config.EnvOllamaModel))

	registry, err := modules.NewRegistry(
		ping.New(),
		status.New(status.Deps{
			Runtime:         runtimeState,
			TrishnaServices: []runtime.HealthReporter{ytService},
			Shawnb:          shawnbMonitor,
			Ollama:          ollamaMonitor,
			Allowlist:       allowlist,
		}),
	)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := trishnabot.New(cfg, registry, logger, runtimeState, trishnabot.Options{
		LogName:  "trishna",
		Username: "trishna",
		Activity: "i like touching people...",
	}, ytService)
	if err != nil {
		return err
	}
	defer app.Close(context.Background())

	return app.Run(ctx)
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
