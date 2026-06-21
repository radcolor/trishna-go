package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	gotgbot "github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/radcolor/trishna-go/internal/modules/ping"
	"github.com/radcolor/trishna-go/internal/runtime"
)

const (
	serviceName          = "telegram"
	pollLimit            = 20
	pollTimeoutSeconds   = 30
	pollRequestTimeout   = 35 * time.Second
	shortRequestTimeout  = 10 * time.Second
	sendRequestTimeout   = 5 * time.Second
	retryBackoff         = 2 * time.Second
	allowedUpdateMessage = "message"
)

type Service struct {
	cfg        Config
	configErr  error
	logger     *slog.Logger
	owners     map[int64]struct{}
	status     func(context.Context) string
	statusHTML func(context.Context) string

	mu        sync.Mutex
	running   bool
	detail    string
	lastOK    *time.Time
	lastError string
}

func NewServiceFromEnv(logger *slog.Logger) *Service {
	cfg, err := LoadConfigFromEnv()
	return NewService(cfg, err, logger)
}

func NewService(cfg Config, configErr error, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	owners := make(map[int64]struct{}, len(cfg.OwnerUserIDs))
	for _, id := range cfg.OwnerUserIDs {
		owners[id] = struct{}{}
	}

	return &Service{
		cfg:       cfg,
		configErr: configErr,
		logger:    logger,
		owners:    owners,
		detail:    "not started",
	}
}

func (s *Service) SetStatusHandler(handler func(context.Context) string) {
	s.status = handler
}

func (s *Service) SetHTMLStatusHandler(handler func(context.Context) string) {
	s.statusHTML = handler
}

func (s *Service) Name() string {
	return serviceName
}

func (s *Service) Health() runtime.ServiceHealth {
	s.mu.Lock()
	defer s.mu.Unlock()
	return runtime.ServiceHealth{
		Name:      s.Name(),
		Running:   s.running,
		Detail:    s.detail,
		LastOK:    s.lastOK,
		LastError: s.lastError,
	}
}

func (s *Service) Run(ctx context.Context) error {
	if s.configErr != nil {
		s.recordStopped("disabled (config error)", s.configErr)
		s.logger.Warn("telegram disabled by config error", slog.String("error", s.configErr.Error()))
		<-ctx.Done()
		return nil
	}

	if s.cfg.Token == "" {
		s.recordStopped("disabled (TELEGRAM_TRISHNA_TOKEN not set)", nil)
		s.logger.Info("telegram disabled", slog.String("reason", "TELEGRAM_TRISHNA_TOKEN not set"))
		<-ctx.Done()
		return nil
	}

	switch s.cfg.Transport {
	case TransportBotAPI:
		return s.runBotAPI(ctx)
	case TransportMTProto:
		return s.runMTProto(ctx)
	default:
		err := fmt.Errorf("unsupported %s %q", EnvTelegramTransport, s.cfg.Transport)
		s.recordStopped("disabled (config error)", err)
		s.logger.Warn("telegram disabled by config error", slog.String("error", err.Error()))
		<-ctx.Done()
		return nil
	}
}

func (s *Service) runBotAPI(ctx context.Context) error {
	for ctx.Err() == nil {
		bot, err := gotgbot.NewBot(s.cfg.Token, &gotgbot.BotOpts{
			RequestOpts: s.requestOpts(shortRequestTimeout),
		})
		if err != nil {
			s.recordStopped("failed to start", err)
			s.logger.Warn("telegram failed to start", slog.String("error", err.Error()))
			if !sleepContext(ctx, retryBackoff) {
				break
			}
			continue
		}

		username := bot.Username
		if username == "" {
			username = "<unknown>"
		}
		s.recordRunning("telegram botapi @" + username)
		s.logger.Info("telegram running", slog.String("transport", TransportBotAPI), slog.String("bot", username))

		offset := int64(0)
		if latestOffset, err := s.dropStaleUpdates(ctx, bot); err != nil {
			if ctx.Err() != nil {
				break
			}
			s.logger.Warn("failed to drop stale telegram updates", slog.String("error", err.Error()))
		} else {
			offset = latestOffset
		}

		if err := s.poll(ctx, bot, offset); err != nil && ctx.Err() == nil {
			s.recordStopped("stopped", err)
			s.logger.Warn("telegram stopped", slog.String("error", err.Error()))
			if !sleepContext(ctx, retryBackoff) {
				break
			}
		}
	}

	s.recordStopped("stopped", nil)
	return nil
}

func (s *Service) dropStaleUpdates(ctx context.Context, bot *gotgbot.Bot) (int64, error) {
	updates, err := bot.GetUpdatesWithContext(ctx, &gotgbot.GetUpdatesOpts{
		Offset:         -1,
		Limit:          1,
		Timeout:        0,
		AllowedUpdates: []string{allowedUpdateMessage},
		RequestOpts:    s.requestOpts(shortRequestTimeout),
	})
	if err != nil {
		return 0, err
	}
	if len(updates) == 0 {
		return 0, nil
	}
	return updates[len(updates)-1].UpdateId + 1, nil
}

func (s *Service) poll(ctx context.Context, bot *gotgbot.Bot, offset int64) error {
	for ctx.Err() == nil {
		updates, err := bot.GetUpdatesWithContext(ctx, &gotgbot.GetUpdatesOpts{
			Offset:         offset,
			Limit:          pollLimit,
			Timeout:        pollTimeoutSeconds,
			AllowedUpdates: []string{allowedUpdateMessage},
			RequestOpts:    s.requestOpts(pollRequestTimeout),
		})
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				break
			}
			s.recordStopped("poll error", err)
			s.logger.Warn("telegram poll error", slog.String("error", err.Error()))
			if !sleepContext(ctx, retryBackoff) {
				break
			}
			continue
		}

		s.recordRunning("telegram botapi polling")
		for _, update := range updates {
			offset = update.UpdateId + 1
			if err := s.handleUpdate(ctx, bot, update); err != nil {
				if ctx.Err() != nil {
					break
				}
				s.logger.Warn("telegram update error", slog.String("error", err.Error()))
			}
		}
	}

	return nil
}

func (s *Service) handleUpdate(ctx context.Context, bot *gotgbot.Bot, update gotgbot.Update) error {
	if update.Message == nil {
		return nil
	}

	message := update.Message
	if message.From == nil || !s.isOwner(message.From.Id) {
		return nil
	}

	command, ok := parseCommand(message.Text, bot.Username)
	if !ok {
		return nil
	}

	response, ok := s.response(ctx, command)
	if !ok {
		return nil
	}

	opts := &gotgbot.SendMessageOpts{
		RequestOpts: s.requestOpts(sendRequestTimeout),
	}
	if response.format == messageFormatHTML {
		opts.ParseMode = "HTML"
	}
	_, err := bot.SendMessageWithContext(ctx, message.Chat.Id, response.text, opts)
	return err
}

type messageFormat string

const (
	messageFormatPlain messageFormat = "plain"
	messageFormatHTML  messageFormat = "html"
)

type commandResponse struct {
	text   string
	format messageFormat
}

func (s *Service) response(ctx context.Context, command string) (commandResponse, bool) {
	switch command {
	case ping.CommandName:
		return commandResponse{text: ping.ResponseText(), format: messageFormatPlain}, true
	case "status":
		if s.statusHTML != nil {
			return commandResponse{text: s.statusHTML(ctx), format: messageFormatHTML}, true
		}
		if s.status != nil {
			return commandResponse{text: s.status(ctx), format: messageFormatPlain}, true
		}
		return commandResponse{}, false
	case networkCommandName:
		return commandResponse{text: s.networkReportHTML(ctx), format: messageFormatHTML}, true
	default:
		return commandResponse{}, false
	}
}

func (s *Service) isOwner(userID int64) bool {
	_, ok := s.owners[userID]
	return ok
}

func (s *Service) requestOpts(timeout time.Duration) *gotgbot.RequestOpts {
	return &gotgbot.RequestOpts{
		Timeout: timeout,
		APIURL:  s.cfg.APIBaseURL,
	}
}

func (s *Service) recordRunning(detail string) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
	s.detail = detail
	s.lastOK = &now
	s.lastError = ""
}

func (s *Service) recordStopped(detail string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	s.detail = detail
	if err != nil {
		s.lastError = err.Error()
	}
}

func parseCommand(text, botUsername string) (string, bool) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return "", false
	}

	token := fields[0]
	if !strings.HasPrefix(token, "/") || len(token) == 1 {
		return "", false
	}

	command := strings.TrimPrefix(token, "/")
	if name, target, ok := strings.Cut(command, "@"); ok {
		if botUsername == "" || !strings.EqualFold(target, botUsername) {
			return "", false
		}
		command = name
	}

	if command == "" {
		return "", false
	}
	return strings.ToLower(command), true
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
