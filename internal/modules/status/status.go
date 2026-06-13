package status

import (
	"context"
	"fmt"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"

	"github.com/radcolor/trishna-go/internal/platform"
	"github.com/radcolor/trishna-go/internal/runtime"
)

const CommandName = "status"

type HostCollector interface {
	Snapshot(ctx context.Context) (platform.HostSnapshot, error)
}

type Deps struct {
	Runtime   *runtime.State
	Host      HostCollector
	Services  []runtime.HealthReporter
	Allowlist []snowflake.ID
}

type Module struct {
	runtime   *runtime.State
	host      HostCollector
	services  []runtime.HealthReporter
	allowlist map[snowflake.ID]struct{}
}

func New(deps Deps) Module {
	allowlist := make(map[snowflake.ID]struct{}, len(deps.Allowlist))
	for _, id := range deps.Allowlist {
		allowlist[id] = struct{}{}
	}

	host := deps.Host
	if host == nil {
		host = platform.NewCollector()
	}

	return Module{
		runtime:   deps.Runtime,
		host:      host,
		services:  append([]runtime.HealthReporter(nil), deps.Services...),
		allowlist: allowlist,
	}
}

func (Module) Name() string {
	return CommandName
}

func (Module) Commands() []discord.ApplicationCommandCreate {
	return []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        CommandName,
			Description: "Show Trishna bot and Mac Mini health.",
		},
	}
}

func (m Module) Register(router handler.Router) {
	router.Command("/"+CommandName, m.HandleInteraction)
}

func (m Module) HandleInteraction(event *handler.CommandEvent) error {
	if !m.allowed(event.User().ID) {
		return event.CreateMessage(discord.MessageCreate{
			Content: "You are not allowed to use this command.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	botSnap := m.runtime.BotSnapshot()
	hostSnap, hostErr := m.host.Snapshot(event.Ctx)
	serviceHealth := make([]runtime.ServiceHealth, 0, len(m.services))
	for _, reporter := range m.services {
		serviceHealth = append(serviceHealth, reporter.Health())
	}

	content := BuildMessage(botSnap, hostSnap, hostErr, serviceHealth)
	return event.CreateMessage(discord.MessageCreate{Content: content})
}

func (m Module) allowed(userID snowflake.ID) bool {
	if len(m.allowlist) == 0 {
		return true
	}
	_, ok := m.allowlist[userID]
	return ok
}

func ParseAllowlist(raw string) ([]snowflake.ID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	var ids []snowflake.ID
	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := snowflake.Parse(part)
		if err != nil {
			return nil, fmt.Errorf("parse status allowlist entry %q: %w", part, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
