package bot

import (
	"context"
	"log/slog"
	"sync"

	"github.com/disgoorg/disgo"
	disgobot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"

	"github.com/radcolor/trishna-go/internal/config"
	"github.com/radcolor/trishna-go/internal/modules"
)

type App struct {
	client    *disgobot.Client
	registry  *modules.Registry
	cfg       config.Config
	logger    *slog.Logger
	services  []modules.BackgroundService
}

func New(cfg config.Config, registry *modules.Registry, logger *slog.Logger, services ...modules.BackgroundService) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}

	router := handler.New()
	registry.Register(router)

	client, err := disgo.New(cfg.DiscordToken,
		disgobot.WithLogger(logger),
		disgobot.WithDefaultGateway(),
		disgobot.WithEventListeners(router),
	)
	if err != nil {
		return nil, err
	}

	return &App{
		client:   client,
		registry: registry,
		cfg:      cfg,
		logger:   logger,
		services: append([]modules.BackgroundService(nil), services...),
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, service := range a.services {
		wg.Add(1)
		go func(svc modules.BackgroundService) {
			defer wg.Done()
			if err := svc.Run(ctx); err != nil && ctx.Err() == nil {
				a.logger.Error("background service stopped",
					slog.String("service", svc.Name()),
					slog.String("error", err.Error()),
				)
			}
		}(service)
	}

	if err := a.syncCommands(); err != nil {
		return err
	}

	if err := a.client.OpenGateway(ctx); err != nil {
		return err
	}

	a.logger.Info("trishna running")
	<-ctx.Done()
	a.logger.Info("trishna shutting down")
	wg.Wait()
	return nil
}

func (a *App) Close(ctx context.Context) {
	a.client.Close(ctx)
}

func (a *App) syncCommands() error {
	var guildIDs []snowflake.ID
	if a.cfg.HasDiscordGuildID {
		guildIDs = []snowflake.ID{a.cfg.DiscordGuildID}
		a.logger.Info("syncing guild commands", slog.String("guild_id", a.cfg.DiscordGuildID.String()))
	} else {
		a.logger.Info("syncing global commands")
	}
	return handler.SyncCommands(a.client, a.registry.Commands(), guildIDs)
}
