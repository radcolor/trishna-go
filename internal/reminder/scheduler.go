package reminder

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/radcolor/trishna-go/internal/chatlog"
	"github.com/radcolor/trishna-go/internal/runtime"
)

const serviceName = "reminder"

type MessageSender interface {
	CreateMessage(channelID snowflake.ID, create discord.MessageCreate, opts ...rest.RequestOpt) (*discord.Message, error)
}

type Scheduler struct {
	store    *Store
	llm      SoulLLM
	sender   MessageSender
	chatLog  *chatlog.Store
	runtime  *runtime.State
	logger   *slog.Logger
	now      func() time.Time
	tickEvery time.Duration
}

func NewScheduler(store *Store, llm SoulLLM, sender MessageSender, runtimeState *runtime.State, chatLog *chatlog.Store, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		store:     store,
		llm:       llm,
		sender:    sender,
		chatLog:   chatLog,
		runtime:   runtimeState,
		logger:    logger,
		now:       time.Now,
		tickEvery: tickInterval,
	}
}

func (s *Scheduler) Name() string {
	return serviceName
}

func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.tickEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.tick(ctx); err != nil && ctx.Err() == nil {
				s.logger.Error("reminder tick failed", slog.String("error", err.Error()))
			}
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) error {
	if s.runtime == nil || !s.runtime.BotSnapshot().Ready {
		return nil
	}
	if s.sender == nil {
		return nil
	}

	due, err := s.store.Due(s.now())
	if err != nil {
		return err
	}

	for _, item := range due {
		if err := s.fire(ctx, item); err != nil && ctx.Err() == nil {
			s.logger.Error("fire reminder failed",
				slog.String("id", item.ID),
				slog.String("error", err.Error()),
			)
		}
	}
	return nil
}

func (s *Scheduler) fire(ctx context.Context, item Reminder) error {
	if item.SendAttempts >= maxSendAttempts {
		s.logger.Warn("dropping reminder after max send attempts", slog.String("id", item.ID))
		return s.store.Remove(item.ID)
	}

	reply, err := s.craftReminder(ctx, item)
	if err != nil {
		return err
	}
	reply = trimForDiscord(reply)

	if _, err := s.sender.CreateMessage(item.ChannelID, discord.MessageCreate{Content: reply}, rest.WithCtx(ctx)); err != nil {
		_ = s.store.IncrementSendAttempts(item.ID)
		return fmt.Errorf("send reminder: %w", err)
	}

	if s.chatLog != nil {
		if err := s.chatLog.Append(chatlog.Entry{
			UserID:    item.UserID.String(),
			ChannelID: item.ChannelID.String(),
			Role:      "assistant",
			Content:   reply,
		}); err != nil {
			s.logger.Error("append reminder chat log failed", slog.String("error", err.Error()))
		}
	}

	return s.store.Remove(item.ID)
}

func (s *Scheduler) craftReminder(ctx context.Context, item Reminder) (string, error) {
	loc, err := LoadLocation()
	if err != nil {
		loc = time.UTC
	}
	when := item.DueAt.In(loc).Format("Mon Jan 2, 3:04 PM MST")
	prompt := fmt.Sprintf(
		"It's time to send a scheduled reminder about: %q. Originally scheduled for %s (Asia/Kolkata). Write a short reminder in your usual tone. One message only.",
		item.Event,
		when,
	)
	system := s.llm.Soul() + "\n\nYou are sending a scheduled reminder notification, not replying to a live chat."
	reply, err := s.llm.Complete(ctx, system, prompt)
	if err != nil {
		return fallbackReminderReply(item.Event), nil
	}
	return strings.TrimSpace(reply), nil
}

func fallbackReminderReply(event string) string {
	return fmt.Sprintf("Reminder: %s", event)
}
