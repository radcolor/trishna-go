package reminder

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/snowflake/v2"

	"github.com/radcolor/trishna-go/internal/chatlog"
	"github.com/radcolor/trishna-go/internal/llm/prompt"
	"github.com/radcolor/trishna-go/internal/ratelimit"
)

type SoulLLM interface {
	Complete(ctx context.Context, system, user string) (string, error)
	Soul() string
}

const reminderParseCooldown = 15 * time.Second

type Coordinator struct {
	parser   *Parser
	store    *Store
	llm      SoulLLM
	chatLog  *chatlog.Store
	logger   *slog.Logger
	cooldown *ratelimit.Cooldown
}

func NewCoordinator(parser *Parser, store *Store, llm SoulLLM, chatLog *chatlog.Store, logger *slog.Logger) *Coordinator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Coordinator{
		parser:   parser,
		store:    store,
		llm:      llm,
		chatLog:  chatLog,
		logger:   logger,
		cooldown: ratelimit.NewCooldown(reminderParseCooldown),
	}
}

func (c *Coordinator) TrySchedule(ctx context.Context, userID, channelID snowflake.ID, message string) (handled bool, reply string, err error) {
	if c.cooldown != nil && !c.cooldown.Allow(userID.String()) {
		return false, "", nil
	}

	result, err := c.parser.Parse(ctx, message)
	if err != nil {
		c.logger.Debug("reminder parse skipped", slog.String("error", err.Error()))
		return false, "", nil
	}

	switch result.Kind {
	case ParseCancel:
		removed, removeErr := c.store.RemoveAllForUser(userID)
		if removeErr != nil {
			return false, "", removeErr
		}
		reply, err = c.craftCancelConfirm(ctx, removed)
		if err != nil {
			return true, fallbackCancelReply(removed), nil
		}
		return true, trimForDiscord(reply), nil

	case ParseSchedule:
		_, addErr := c.store.Add(userID, channelID, result.Event, result.DueAt, message)
		if addErr != nil {
			return false, "", addErr
		}
		reply, err = c.craftScheduleConfirm(ctx, result.Event, result.DueAt)
		if err != nil {
			c.logger.Error("reminder confirm llm failed", slog.String("error", err.Error()))
			return true, fallbackScheduleReply(result.Event, result.DueAt), nil
		}
		return true, trimForDiscord(reply), nil

	default:
		return false, "", nil
	}
}

func (c *Coordinator) craftScheduleConfirm(ctx context.Context, event string, dueAt time.Time) (string, error) {
	loc, err := LoadLocation()
	if err != nil {
		loc = time.UTC
	}
	when := dueAt.In(loc).Format("Mon Jan 2, 3:04 PM MST")
	userPrompt := fmt.Sprintf(
		"The user just asked for a reminder about %q at %s (Asia/Kolkata). The reminder is already scheduled. Reply briefly in your usual tone confirming you'll ping them then. One short message only.",
		event,
		when,
	)
	reply, err := c.llm.Complete(ctx, c.llm.Soul(), userPrompt)
	if err != nil {
		return "", err
	}
	return prompt.SanitizeDiscordOutput(reply), nil
}

func (c *Coordinator) craftCancelConfirm(ctx context.Context, removed int) (string, error) {
	userPrompt := fmt.Sprintf(
		"The user asked to cancel their reminders. You cleared %d pending reminder(s). Reply briefly confirming they're cancelled. One short message only.",
		removed,
	)
	reply, err := c.llm.Complete(ctx, c.llm.Soul(), userPrompt)
	if err != nil {
		return "", err
	}
	return prompt.SanitizeDiscordOutput(reply), nil
}

func fallbackScheduleReply(event string, dueAt time.Time) string {
	loc, err := LoadLocation()
	if err != nil {
		loc = time.UTC
	}
	when := dueAt.In(loc).Format("Mon Jan 2, 3:04 PM MST")
	return fmt.Sprintf("Got it — I'll remind you about %s on %s", event, when)
}

func fallbackCancelReply(removed int) string {
	if removed == 0 {
		return "You don't have any pending reminders right now."
	}
	return fmt.Sprintf("Okay, I cancelled your %d reminder(s).", removed)
}

func trimForDiscord(content string) string {
	content = prompt.SanitizeDiscordOutput(content)
	content = strings.TrimSpace(content)
	if len(content) <= maxDiscordMessage {
		return content
	}
	return content[:maxDiscordMessage-3] + "..."
}
