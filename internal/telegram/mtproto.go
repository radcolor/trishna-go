package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gotd/td/session"
	gotdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/telegram/message"
	messagehtml "github.com/gotd/td/telegram/message/html"
	"github.com/gotd/td/tg"
)

func (s *Service) runMTProto(ctx context.Context) error {
	failures := 0
	for ctx.Err() == nil {
		started := time.Now()
		if err := s.runMTProtoOnce(ctx); err != nil {
			if ctx.Err() != nil {
				break
			}
			if s.hasOKSince(started) {
				failures = 0
			}
			failures++
			if !s.retryOrGiveUp(ctx, failures, "telegram mtproto stopped", err) {
				break
			}
			continue
		}
	}

	s.recordStopped("stopped", nil)
	return nil
}

func (s *Service) runMTProtoOnce(ctx context.Context) error {
	if err := ensureSecureSessionPath(s.cfg.MTProto.SessionPath); err != nil {
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
	if !s.isAllowedMTProtoChat(textMessage) {
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

func messageChatID(msg tg.NotEmptyMessage) (int64, bool) {
	peer := msg.GetPeerID()
	switch p := peer.(type) {
	case *tg.PeerUser:
		if p.UserID <= 0 {
			return 0, false
		}
		return p.UserID, true
	case *tg.PeerChat:
		if p.ChatID <= 0 {
			return 0, false
		}
		return -p.ChatID, true
	case *tg.PeerChannel:
		if p.ChannelID <= 0 {
			return 0, false
		}
		return -1000000000000 - p.ChannelID, true
	default:
		return 0, false
	}
}

func (s *Service) isAllowedMTProtoChat(msg tg.NotEmptyMessage) bool {
	if len(s.chats) == 0 {
		return true
	}
	chatID, ok := messageChatID(msg)
	return ok && s.isAllowedChat(chatID)
}

func ensureSecureSessionPath(path string) error {
	if path == "" {
		return fmt.Errorf("mtproto session path is required")
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create mtproto session dir: %w", err)
		}
		dirInfo, err := os.Lstat(dir)
		if err != nil {
			return fmt.Errorf("stat mtproto session dir: %w", err)
		}
		if dirInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("mtproto session dir must not be a symlink")
		}
		if !dirInfo.IsDir() {
			return fmt.Errorf("mtproto session dir is not a directory")
		}
		if dirInfo.Mode().Perm() != 0o700 {
			if err := os.Chmod(dir, 0o700); err != nil {
				return fmt.Errorf("chmod mtproto session dir: %w", err)
			}
		}
	}

	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat mtproto session path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("mtproto session path must not be a symlink")
	}
	if info.IsDir() {
		return fmt.Errorf("mtproto session path is a directory")
	}
	if info.Mode().Perm() != 0o600 {
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("chmod mtproto session path: %w", err)
		}
	}
	return nil
}
