package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	shawnbot "github.com/radcolor/trishna-go/internal/bot"
	"github.com/radcolor/trishna-go/internal/chatlog"
	"github.com/radcolor/trishna-go/internal/llm/ollama"
	"github.com/radcolor/trishna-go/internal/modules"
	"github.com/radcolor/trishna-go/internal/modules/chat"
	"github.com/radcolor/trishna-go/internal/ownernotify"
	"github.com/radcolor/trishna-go/internal/reminder"
	"github.com/radcolor/trishna-go/internal/runtime"
	shawnbconfig "github.com/radcolor/trishna-go/internal/shawnb/config"
	"github.com/radcolor/trishna-go/internal/shawnb/heartbeat"

	disgobot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/gateway"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := shawnbconfig.LoadFromEnv()
	if err != nil {
		return err
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	soul, err := ollama.LoadSoul(cfg.SoulMDPath)
	if err != nil {
		return err
	}

	llmClient, err := ollama.NewClient(cfg.OllamaBaseURL, cfg.OllamaModel, soul)
	if err != nil {
		return err
	}

	chatStore, err := chatlog.NewStore(cfg.ChatLogDir)
	if err != nil {
		return err
	}

	reminderStore, err := reminder.NewStore(cfg.RemindersPath)
	if err != nil {
		return err
	}

	kolkata, err := reminder.LoadLocation()
	if err != nil {
		return err
	}

	reminderParser := reminder.NewParser(llmClient, kolkata)
	reminderCoordinator := reminder.NewCoordinator(reminderParser, reminderStore, llmClient, chatStore, logger)

	ownerParser := ownernotify.NewParser(llmClient)
	restProvider := &ownernotify.LazyRESTProvider{}
	ownerNotifier := ownernotify.NewNotifier(ownerParser, cfg.OwnerUserID, restProvider, logger)

	chatModule := chat.New(chat.Deps{
		LLM:               llmClient,
		ChatLog:           chatStore,
		AllowedUserIDs:    cfg.AllowedUserIDs,
		AllowedChannelIDs: cfg.AllowedChannelIDs,
		HistoryLimit:      cfg.HistoryLimit,
		Logger:            logger,
		Reminder:          reminderCoordinator,
		OwnerNotifier:     ownerNotifier,
	})

	registry, err := modules.NewRegistry(chatModule)
	if err != nil {
		return err
	}

	runtimeState := runtime.NewState()

	heartbeatStore, err := heartbeat.NewStore(cfg.HeartbeatPath)
	if err != nil {
		return err
	}
	heartbeatService := heartbeat.NewService(heartbeatStore, runtimeState, "shawnb", cfg.OllamaModel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	reminderScheduler := reminder.NewScheduler(reminderStore, llmClient, restProvider, runtimeState, chatStore, logger)

	app, err := shawnbot.New(cfg.BotConfig(), registry, logger, runtimeState, shawnbot.Options{
		LogName:  "shawnb",
		Username: "shawnb",
		Activity: "chatting",
		GatewayConfigOpts: []gateway.ConfigOpt{
			gateway.WithIntents(
				gateway.IntentGuilds,
				gateway.IntentGuildMessages,
				gateway.IntentDirectMessages,
				gateway.IntentMessageContent,
			),
		},
		ExtraListeners: []disgobot.EventListener{chatModule.EventListener()},
	}, heartbeatService, reminderScheduler)
	if err != nil {
		return err
	}
	defer app.Close(context.Background())

	restProvider.Bind(app.Rest())

	return app.Run(ctx)
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
