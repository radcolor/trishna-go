package status

import (
	"context"
	"fmt"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"

	"github.com/radcolor/trishna-go/internal/llm/ollama"
	"github.com/radcolor/trishna-go/internal/platform"
	"github.com/radcolor/trishna-go/internal/runtime"
	"github.com/radcolor/trishna-go/internal/shawnb/monitor"
)

const CommandName = "status"

type HostCollector interface {
	Snapshot(ctx context.Context) (platform.HostSnapshot, error)
}

type Deps struct {
	Runtime         *runtime.State
	Host            HostCollector
	TrishnaServices []runtime.HealthReporter
	Shawnb          ShawnbReporter
	Ollama          OllamaReporter
	Allowlist       []snowflake.ID
}

type ShawnbReporter interface {
	Status() monitor.Status
}

type OllamaReporter interface {
	Snapshot(ctx context.Context) ollama.Status
}

type Module struct {
	runtime         *runtime.State
	host            HostCollector
	trishnaServices []runtime.HealthReporter
	shawnb          ShawnbReporter
	ollama          OllamaReporter
	allowlist       map[snowflake.ID]struct{}
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
		runtime:         deps.Runtime,
		host:            host,
		trishnaServices: append([]runtime.HealthReporter(nil), deps.TrishnaServices...),
		shawnb:          deps.Shawnb,
		ollama:          deps.Ollama,
		allowlist:       allowlist,
	}
}

func (m *Module) SetTrishnaServices(services []runtime.HealthReporter) {
	m.trishnaServices = append([]runtime.HealthReporter(nil), services...)
}

func (Module) Name() string {
	return CommandName
}

func (Module) Commands() []discord.ApplicationCommandCreate {
	return []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        CommandName,
			Description: "Show Trishna, shawnb, and Mac Mini health.",
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

	return event.CreateMessage(discord.MessageCreate{Content: m.ResponseText(event.Ctx)})
}

func (m Module) ResponseText(ctx context.Context) string {
	return BuildMessage(m.Report(ctx))
}

func (m Module) HTMLResponseText(ctx context.Context) string {
	return BuildHTMLMessage(m.Report(ctx))
}

func (m Module) Report(ctx context.Context) Report {
	var botSnap runtime.BotSnapshot
	if m.runtime != nil {
		botSnap = m.runtime.BotSnapshot()
	}

	var hostSnap platform.HostSnapshot
	var hostErr error
	if m.host != nil {
		hostSnap, hostErr = m.host.Snapshot(ctx)
	}

	trishnaServices := make([]runtime.ServiceHealth, 0, len(m.trishnaServices))
	for _, reporter := range m.trishnaServices {
		if reporter != nil {
			trishnaServices = append(trishnaServices, reporter.Health())
		}
	}

	var shawnbStatus monitor.Status
	if m.shawnb != nil {
		shawnbStatus = m.shawnb.Status()
	}

	var ollamaStatus ollama.Status
	if m.ollama != nil {
		ollamaStatus = m.ollama.Snapshot(ctx)
	}

	return Report{
		TrishnaBot:      botSnap,
		TrishnaServices: trishnaServices,
		Shawnb:          shawnbStatus,
		Ollama:          ollamaStatus,
		Host:            hostSnap,
		HostErr:         hostErr,
	}
}

func (m Module) allowed(userID snowflake.ID) bool {
	if len(m.allowlist) == 0 {
		return false
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
