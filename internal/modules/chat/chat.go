package chat

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	disgobot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/radcolor/trishna-go/internal/chatlog"
	"github.com/radcolor/trishna-go/internal/llm/ollama"
)

const (
	ModuleName        = "chat"
	ResetCommandName  = "reset"
	fallbackReply     = "I'm having trouble replying right now. Try again in a moment."
	maxDiscordMessage = 2000
)

type LLM interface {
	Chat(ctx context.Context, history []ollama.Message) (string, error)
}

type ReminderHandler interface {
	TrySchedule(ctx context.Context, userID, channelID snowflake.ID, message string) (handled bool, reply string, err error)
}

type OwnerNotifier interface {
	MaybeNotify(ctx context.Context, userID, channelID snowflake.ID, isDM bool, message string)
}

type Deps struct {
	LLM               LLM
	ChatLog           *chatlog.Store
	AllowedUserIDs    []snowflake.ID
	AllowedChannelIDs []snowflake.ID
	HistoryLimit      int
	Logger            *slog.Logger
	Reminder          ReminderHandler
	OwnerNotifier     OwnerNotifier
}

type Module struct {
	llm             LLM
	chatLog         *chatlog.Store
	allowedUsers    map[snowflake.ID]struct{}
	allowedChannels map[snowflake.ID]struct{}
	logger          *slog.Logger
	history         *conversationHistory
	reminder        ReminderHandler
	ownerNotifier   OwnerNotifier
}

type conversationHistory struct {
	mu    sync.Mutex
	limit int
	byKey map[string][]ollama.Message
}

func New(deps Deps) Module {
	allowedUsers := make(map[snowflake.ID]struct{}, len(deps.AllowedUserIDs))
	for _, id := range deps.AllowedUserIDs {
		allowedUsers[id] = struct{}{}
	}

	allowedChannels := make(map[snowflake.ID]struct{}, len(deps.AllowedChannelIDs))
	for _, id := range deps.AllowedChannelIDs {
		allowedChannels[id] = struct{}{}
	}

	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	limit := deps.HistoryLimit
	if limit < 1 {
		limit = 20
	}

	return Module{
		llm:             deps.LLM,
		chatLog:         deps.ChatLog,
		allowedUsers:    allowedUsers,
		allowedChannels: allowedChannels,
		logger:          logger,
		reminder:        deps.Reminder,
		ownerNotifier:   deps.OwnerNotifier,
		history: &conversationHistory{
			limit: limit,
			byKey: make(map[string][]ollama.Message),
		},
	}
}

func (Module) Name() string {
	return ModuleName
}

func (Module) Commands() []discord.ApplicationCommandCreate {
	return []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        ResetCommandName,
			Description: "Clear your chat history with this bot.",
		},
	}
}

func (m Module) Register(router handler.Router) {
	router.Command("/"+ResetCommandName, m.handleReset)
}

func (m Module) EventListener() disgobot.EventListener {
	return disgobot.NewListenerFunc(m.handleMessageCreate)
}

func (m Module) handleReset(event *handler.CommandEvent) error {
	if !m.allowedUser(event.User().ID) {
		return event.CreateMessage(discord.MessageCreate{
			Content: "You are not allowed to use this command.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	m.clearHistory(event.User().ID, event.Channel().ID())
	return event.CreateMessage(discord.MessageCreate{
		Content: "Chat history cleared.",
		Flags:   discord.MessageFlagEphemeral,
	})
}

func (m Module) handleMessageCreate(event *events.MessageCreate) {
	if event.Message.Author.Bot {
		return
	}
	if !m.allowedUser(event.Message.Author.ID) {
		return
	}
	if !m.allowedSurface(event) {
		return
	}

	content := strings.TrimSpace(event.Message.Content)
	if content == "" {
		return
	}

	ctx := context.Background()
	userID := event.Message.Author.ID
	channelID := event.ChannelID
	isDM := event.GuildID == nil

	m.appendHistory(userID, channelID, ollama.Message{Role: "user", Content: content})
	m.logMessage(userID, channelID, isDM, "user", content)

	if m.ownerNotifier != nil {
		go m.ownerNotifier.MaybeNotify(ctx, userID, channelID, isDM, content)
	}

	_ = event.Client().Rest.SendTyping(channelID, rest.WithCtx(ctx))

	if m.reminder != nil {
		handled, confirm, err := m.reminder.TrySchedule(ctx, userID, channelID, content)
		if err != nil {
			m.logger.Error("reminder scheduling failed", slog.String("error", err.Error()))
		} else if handled {
			confirm = trimForDiscord(confirm)
			m.appendHistory(userID, channelID, ollama.Message{Role: "assistant", Content: confirm})
			m.logMessage(userID, channelID, isDM, "assistant", confirm)
			if _, err := event.Client().Rest.CreateMessage(channelID, discord.MessageCreate{Content: confirm}, rest.WithCtx(ctx)); err != nil {
				m.logger.Error("send reminder confirm failed", slog.String("error", err.Error()))
			}
			return
		}
	}

	history := m.snapshotHistory(userID, channelID)
	reply, err := m.llm.Chat(ctx, history)
	if err != nil {
		m.logger.Error("ollama chat failed", slog.String("error", err.Error()))
		reply = fallbackReply
	}

	reply = trimForDiscord(reply)
	m.appendHistory(userID, channelID, ollama.Message{Role: "assistant", Content: reply})
	m.logMessage(userID, channelID, isDM, "assistant", reply)

	if _, err := event.Client().Rest.CreateMessage(channelID, discord.MessageCreate{Content: reply}, rest.WithCtx(ctx)); err != nil {
		m.logger.Error("send chat reply failed", slog.String("error", err.Error()))
	}
}

func (m Module) allowedSurface(event *events.MessageCreate) bool {
	return m.allowedLocation(event.GuildID, event.ChannelID)
}

func (m Module) allowedLocation(guildID *snowflake.ID, channelID snowflake.ID) bool {
	if guildID == nil {
		return true
	}
	if len(m.allowedChannels) == 0 {
		return false
	}
	_, ok := m.allowedChannels[channelID]
	return ok
}

func (m Module) allowedUser(userID snowflake.ID) bool {
	_, ok := m.allowedUsers[userID]
	return ok
}

func (m Module) appendHistory(userID, channelID snowflake.ID, message ollama.Message) {
	key := historyKey(userID, channelID)

	m.history.mu.Lock()
	defer m.history.mu.Unlock()

	history := append(m.history.byKey[key], message)
	if len(history) > m.history.limit {
		history = history[len(history)-m.history.limit:]
	}
	m.history.byKey[key] = history
}

func (m Module) snapshotHistory(userID, channelID snowflake.ID) []ollama.Message {
	key := historyKey(userID, channelID)

	m.history.mu.Lock()
	defer m.history.mu.Unlock()

	history := m.history.byKey[key]
	return append([]ollama.Message(nil), history...)
}

func (m Module) clearHistory(userID, channelID snowflake.ID) {
	key := historyKey(userID, channelID)

	m.history.mu.Lock()
	defer m.history.mu.Unlock()

	delete(m.history.byKey, key)
}

func (m Module) logMessage(userID, channelID snowflake.ID, isDM bool, role, content string) {
	if m.chatLog == nil {
		return
	}
	if err := m.chatLog.Append(chatlog.Entry{
		UserID:    userID.String(),
		ChannelID: channelID.String(),
		IsDM:      isDM,
		Role:      role,
		Content:   content,
	}); err != nil {
		m.logger.Error("append chat log failed", slog.String("error", err.Error()))
	}
}

func trimForDiscord(content string) string {
	content = strings.TrimSpace(content)
	if len(content) <= maxDiscordMessage {
		return content
	}
	return content[:maxDiscordMessage-3] + "..."
}

func historyKey(userID, channelID snowflake.ID) string {
	return userID.String() + ":" + channelID.String()
}
