package telegram

import (
	"encoding/base64"
	"encoding/hex"
	"path/filepath"
	"strings"
)

const (
	redactedTelegramSecret  = "[redacted-telegram-secret]"
	redactedTelegramSession = "[redacted-telegram-session]"
)

func (s *Service) redactTelegramSecrets(value string) string {
	if value == "" {
		return value
	}

	replacements := []string{
		s.cfg.Token,
		s.cfg.MTProto.AppHash,
	}
	if len(s.cfg.MTProto.ProxySecret) > 0 {
		hexSecret := hex.EncodeToString(s.cfg.MTProto.ProxySecret)
		replacements = append(replacements,
			hexSecret,
			strings.ToUpper(hexSecret),
			base64.RawURLEncoding.EncodeToString(s.cfg.MTProto.ProxySecret),
			base64.URLEncoding.EncodeToString(s.cfg.MTProto.ProxySecret),
			base64.RawStdEncoding.EncodeToString(s.cfg.MTProto.ProxySecret),
			base64.StdEncoding.EncodeToString(s.cfg.MTProto.ProxySecret),
		)
	}
	for _, secret := range replacements {
		if secret == "" {
			continue
		}
		value = strings.ReplaceAll(value, secret, redactedTelegramSecret)
	}

	if path := strings.TrimSpace(s.cfg.MTProto.SessionPath); path != "" {
		value = strings.ReplaceAll(value, path, redactedTelegramSession)
		if base := filepath.Base(path); base != "." && base != "/" && base != "" {
			value = strings.ReplaceAll(value, base, redactedTelegramSession)
		}
	}

	return value
}
