package ownernotify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/radcolor/trishna-go/internal/llm/prompt"
	"github.com/radcolor/trishna-go/internal/ratelimit"
)

type RESTProvider interface {
	Rest() rest.Rest
}

type LazyRESTProvider struct {
	mu   sync.RWMutex
	rest rest.Rest
}

func (p *LazyRESTProvider) Bind(r rest.Rest) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rest = r
}

func (p *LazyRESTProvider) Rest() rest.Rest {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.rest
}

func (p *LazyRESTProvider) CreateMessage(channelID snowflake.ID, create discord.MessageCreate, opts ...rest.RequestOpt) (*discord.Message, error) {
	r := p.Rest()
	if r == nil {
		return nil, fmt.Errorf("discord rest not bound")
	}
	return r.CreateMessage(channelID, create, opts...)
}

const ownerNotifyCooldown = 2 * time.Minute
const maxConcurrentNotify = 2

type Notifier struct {
	parser    *Parser
	ownerID   snowflake.ID
	rest      RESTProvider
	logger    *slog.Logger
	cooldown  *ratelimit.Cooldown
	notifySem chan struct{}
}

func NewNotifier(parser *Parser, ownerID snowflake.ID, restProvider RESTProvider, logger *slog.Logger) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{
		parser:    parser,
		ownerID:   ownerID,
		rest:      restProvider,
		logger:    logger,
		cooldown:  ratelimit.NewCooldown(ownerNotifyCooldown),
		notifySem: make(chan struct{}, maxConcurrentNotify),
	}
}

func (n *Notifier) MaybeNotify(ctx context.Context, userID, channelID snowflake.ID, isDM bool, message string) {
	if n.ownerID == 0 || n.parser == nil {
		return
	}
	if n.cooldown != nil && !n.cooldown.Allow(userID.String()) {
		n.logger.Debug("owner notify rate limited", slog.String("user_id", userID.String()))
		return
	}

	select {
	case n.notifySem <- struct{}{}:
		defer func() { <-n.notifySem }()
	default:
		n.logger.Debug("owner notify concurrency limit")
		return
	}

	result, err := n.parser.Parse(ctx, message)
	if err != nil {
		n.logger.Debug("owner notify parse skipped", slog.String("error", err.Error()))
		return
	}
	if !result.Notify {
		return
	}

	if err := n.send(ctx, userID, channelID, isDM, message, result); err != nil {
		n.logger.Error("owner notify failed",
			slog.String("category", string(result.Category)),
			slog.String("error", err.Error()),
		)
	}
}

func (n *Notifier) send(ctx context.Context, userID, channelID snowflake.ID, isDM bool, message string, result Result) error {
	r := n.rest.Rest()
	if r == nil {
		return fmt.Errorf("discord rest not bound")
	}

	dm, err := r.CreateDMChannel(n.ownerID, rest.WithCtx(ctx))
	if err != nil {
		return fmt.Errorf("create owner dm: %w", err)
	}

	surface := "channel"
	if isDM {
		surface = "DM"
	}

	body := fmt.Sprintf(
		"**Owner alert** — %s\n\n**User message:**\n> %s\n\n**Note:** %s\n\n_(shawnb · %s · user %s)_",
		categoryLabel(result.Category),
		quoteForDiscord(message),
		prompt.TruncateSummary(result.Summary),
		surface,
		userID.String(),
	)
	body = trimForDiscord(body)

	if _, err := r.CreateMessage(dm.ID(), discord.MessageCreate{Content: body}, rest.WithCtx(ctx)); err != nil {
		return fmt.Errorf("send owner dm: %w", err)
	}

	n.logger.Info("notified owner",
		slog.String("category", string(result.Category)),
		slog.String("channel_id", channelID.String()),
	)
	return nil
}

func quoteForDiscord(message string) string {
	message = strings.TrimSpace(message)
	message = strings.ReplaceAll(message, "\r\n", "\n")
	message = strings.ReplaceAll(message, "\n", "\n> ")
	if len(message) > 500 {
		message = message[:497] + "..."
	}
	return message
}

func trimForDiscord(content string) string {
	content = prompt.SanitizeDiscordOutput(content)
	content = strings.TrimSpace(content)
	if len(content) <= maxDiscordMessage {
		return content
	}
	return content[:maxDiscordMessage-3] + "..."
}
