package bot

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/disgoorg/disgo"
	disgobot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/radcolor/trishna-go/internal/config"
	"github.com/radcolor/trishna-go/internal/modules"
	"github.com/radcolor/trishna-go/internal/runtime"
)

type Options struct {
	LogName           string
	HealthName        string
	Username          string
	Activity          string
	GatewayConfigOpts []gateway.ConfigOpt
	ExtraListeners    []disgobot.EventListener
	OnRestReady       func(rest.Rest)
}

type App struct {
	client   *disgobot.Client
	registry *modules.Registry
	cfg      config.Config
	logger   *slog.Logger
	services []modules.BackgroundService
	runtime  *runtime.State
	opts     Options
}

func New(cfg config.Config, registry *modules.Registry, logger *slog.Logger, runtimeState *runtime.State, opts Options, services ...modules.BackgroundService) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if runtimeState == nil {
		runtimeState = runtime.NewState()
	}
	if opts.LogName == "" {
		opts.LogName = "bot"
	}

	router := handler.New()
	registry.Register(router)

	listeners := make([]disgobot.EventListener, 0, 1+len(opts.ExtraListeners))
	listeners = append(listeners, router)
	listeners = append(listeners, opts.ExtraListeners...)

	gatewayOpts := append([]gateway.ConfigOpt(nil), opts.GatewayConfigOpts...)
	if opts.Activity != "" {
		presenceOpts := []gateway.PresenceOpt{
			gateway.WithOnlineStatus(discord.OnlineStatusOnline),
			gateway.WithCustomActivity(opts.Activity),
		}
		gatewayOpts = append(gatewayOpts, gateway.WithPresenceOpts(presenceOpts...))
	}

	client, err := disgo.New(cfg.DiscordToken,
		disgobot.WithLogger(logger),
		disgobot.WithDefaultGateway(),
		disgobot.WithGatewayConfigOpts(gatewayOpts...),
		disgobot.WithEventListeners(listeners...),
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
		runtime:  runtimeState,
		opts:     opts,
	}, nil
}

func (a *App) Runtime() *runtime.State {
	return a.runtime
}

func (a *App) Rest() rest.Rest {
	return a.client.Rest
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
	if a.opts.OnRestReady != nil {
		a.opts.OnRestReady(a.client.Rest)
	}

	if err := a.ensureUsername(ctx); err != nil {
		a.logger.Warn("failed to update bot username", slog.String("error", err.Error()))
	}

	a.runtime.MarkReady()

	a.logger.Info(a.opts.LogName+" running", slog.String("bot", a.opts.LogName))
	<-ctx.Done()
	a.logger.Info(a.opts.LogName+" shutting down", slog.String("bot", a.opts.LogName))
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

func (a *App) ensureUsername(ctx context.Context) error {
	if a.opts.Username == "" {
		return nil
	}

	if selfUser, ok := a.client.Caches.SelfUser(); ok && selfUser.Username == a.opts.Username {
		return nil
	}

	_, err := a.client.Rest.UpdateCurrentUser(
		discord.UserUpdate{Username: a.opts.Username},
		rest.WithCtx(ctx),
	)
	if err != nil {
		return fmt.Errorf("update bot username: %w", err)
	}

	a.logger.Info("updated bot username", slog.String("username", a.opts.Username))
	return nil
}
