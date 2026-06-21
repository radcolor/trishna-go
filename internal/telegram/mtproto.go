package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gotd/td/session"
	gotdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/telegram/message"
	messagehtml "github.com/gotd/td/telegram/message/html"
	"github.com/gotd/td/tg"
)

func (s *Service) runMTProto(ctx context.Context) error {
	for ctx.Err() == nil {
		if err := s.runMTProtoOnce(ctx); err != nil {
			if ctx.Err() != nil {
				break
			}
			s.recordStopped("mtproto stopped", err)
			s.logger.Warn("telegram mtproto stopped", slog.String("error", err.Error()))
			if !sleepContext(ctx, retryBackoff) {
				break
			}
			continue
		}
	}

	s.recordStopped("stopped", nil)
	return nil
}

func (s *Service) runMTProtoOnce(ctx context.Context) error {
	if err := ensureSessionDir(s.cfg.MTProto.SessionPath); err != nil {
		return err
	}

	dispatcher := tg.NewUpdateDispatcher()
	opts := gotdtelegram.Options{
		SessionStorage: &session.FileStorage{Path: s.cfg.MTProto.SessionPath},
		UpdateHandler:  dispatcher,
	}

	if s.cfg.MTProto.ProxyEnabled {
		resolver, err := s.mtproxyResolver()
		if err != nil {
			return err
		}
		opts.Resolver = resolver
	}

	client := gotdtelegram.NewClient(s.cfg.MTProto.AppID, s.cfg.MTProto.AppHash, opts)
	sender := message.NewSender(client.API())
	botUsername := ""

	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		return s.handleMTProtoMessage(ctx, sender, botUsername, e, update)
	})
	dispatcher.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewChannelMessage) error {
		return s.handleMTProtoMessage(ctx, sender, botUsername, e, update)
	})

	return client.Run(ctx, func(ctx context.Context) error {
		self, err := s.ensureMTProtoAuth(ctx, client)
		if err != nil {
			return err
		}
		botUsername = self.Username
		if botUsername == "" {
			botUsername = "<unknown>"
		}

		detail := "telegram mtproto direct @" + botUsername
		if s.cfg.MTProto.ProxyEnabled {
			detail = "telegram mtproto via mtproxy @" + botUsername
		}
		s.recordRunning(detail)
		s.logger.Info("telegram running",
			slog.String("transport", TransportMTProto),
			slog.Bool("mtproxy", s.cfg.MTProto.ProxyEnabled),
			slog.String("bot", botUsername),
		)

		<-ctx.Done()
		return nil
	})
}

func (s *Service) ensureMTProtoAuth(ctx context.Context, client *gotdtelegram.Client) (*tg.User, error) {
	status, err := client.Auth().Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("get auth status: %w", err)
	}
	if status.Authorized && status.User != nil {
		return status.User, nil
	}

	authorization, err := client.Auth().Bot(ctx, s.cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("bot auth: %w", err)
	}
	user, ok := authorization.User.AsNotEmpty()
	if !ok {
		return nil, fmt.Errorf("bot auth returned empty user")
	}
	return user, nil
}

func (s *Service) mtproxyResolver() (dcs.Resolver, error) {
	addr := net.JoinHostPort(s.cfg.MTProto.ProxyHost, strconv.Itoa(s.cfg.MTProto.ProxyPort))
	resolver, err := dcs.MTProxy(addr, s.cfg.MTProto.ProxySecret, dcs.MTProxyOptions{})
	if err != nil {
		return nil, fmt.Errorf("create mtproxy resolver: %w", err)
	}
	return resolver, nil
}

type mtprotoMessageUpdate interface {
	GetMessage() tg.MessageClass
}

func (s *Service) handleMTProtoMessage(ctx context.Context, sender *message.Sender, botUsername string, e tg.Entities, update mtprotoMessageUpdate) error {
	msg, ok := update.GetMessage().AsNotEmpty()
	if !ok || msg.GetOut() {
		return nil
	}

	textMessage, ok := msg.(*tg.Message)
	if !ok {
		return nil
	}

	userID, ok := messageSenderUserID(textMessage)
	if !ok || !s.isOwner(userID) {
		return nil
	}

	command, ok := parseCommand(textMessage.Message, botUsername)
	if !ok {
		return nil
	}

	response, ok := s.response(ctx, command)
	if !ok {
		return nil
	}

	builder := sender.Answer(e, update)
	if response.format == messageFormatHTML {
		_, err := builder.StyledText(ctx, messagehtml.String(nil, response.text))
		return err
	}

	_, err := builder.Text(ctx, response.text)
	return err
}

func messageSenderUserID(msg tg.NotEmptyMessage) (int64, bool) {
	fromID, ok := msg.GetFromID()
	if !ok {
		fromID = msg.GetPeerID()
	}
	peerUser, ok := fromID.(*tg.PeerUser)
	if !ok || peerUser.UserID <= 0 {
		return 0, false
	}
	return peerUser.UserID, true
}

func ensureSessionDir(path string) error {
	if path == "" {
		return fmt.Errorf("mtproto session path is required")
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create mtproto session dir: %w", err)
	}
	return nil
}
