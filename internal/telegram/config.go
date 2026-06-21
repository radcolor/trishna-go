package telegram

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/gotd/td/mtproxy"
)

const (
	EnvTelegramTrishnaToken       = "TELEGRAM_TRISHNA_TOKEN"
	EnvTelegramOwnerUserIDs       = "TELEGRAM_OWNER_USER_IDS"
	EnvTelegramAPIBaseURL         = "TELEGRAM_API_BASE_URL"
	EnvTelegramTransport          = "TELEGRAM_TRANSPORT"
	EnvTelegramAllowedChatIDs     = "TELEGRAM_ALLOWED_CHAT_IDS"
	EnvTelegramTGNetRevealIPs     = "TELEGRAM_TGNET_REVEAL_IPS"
	EnvTelegramMTProtoAppID       = "TELEGRAM_MTPROTO_APP_ID"
	EnvTelegramMTProtoAppHash     = "TELEGRAM_MTPROTO_APP_HASH"
	EnvTelegramMTProtoSessionPath = "TELEGRAM_MTPROTO_SESSION_PATH"
	EnvTelegramMTProxyEnabled     = "TELEGRAM_MTPROTO_PROXY_ENABLED"
	EnvTelegramMTProxyHost        = "TELEGRAM_MTPROTO_PROXY_HOST"
	EnvTelegramMTProxyPort        = "TELEGRAM_MTPROTO_PROXY_PORT"
	EnvTelegramMTProxySecret      = "TELEGRAM_MTPROTO_PROXY_SECRET"

	TransportMTProto = "mtproto"
	TransportBotAPI  = "botapi"

	DefaultMTProtoSessionPath = "data/telegram/mtproto-session.json"
)

type Config struct {
	Token          string
	OwnerUserIDs   []int64
	AllowedChatIDs []int64
	APIBaseURL     string
	Transport      string
	TGNetRevealIPs bool
	MTProto        MTProtoConfig
}

type MTProtoConfig struct {
	AppID        int
	AppHash      string
	SessionPath  string
	ProxyEnabled bool
	ProxyHost    string
	ProxyPort    int
	ProxySecret  []byte
}

type LookupEnv func(key string) (string, bool)

func LoadConfigFromEnv() (Config, error) {
	return LoadConfigFromLookupEnv(sysLookupEnv)
}

func LoadConfigFromLookupEnv(lookup LookupEnv) (Config, error) {
	token, _ := lookupTrimmed(lookup, EnvTelegramTrishnaToken)
	cfg := Config{
		Token:     token,
		Transport: TransportMTProto,
		MTProto: MTProtoConfig{
			SessionPath:  DefaultMTProtoSessionPath,
			ProxyEnabled: true,
		},
	}

	apiBaseURL, _ := lookupTrimmed(lookup, EnvTelegramAPIBaseURL)
	cfg.APIBaseURL = strings.TrimRight(apiBaseURL, "/")

	if rawTransport, ok := lookupTrimmed(lookup, EnvTelegramTransport); ok {
		transport, err := parseTransport(rawTransport)
		if err != nil {
			return Config{}, err
		}
		cfg.Transport = transport
	}

	if rawSessionPath, ok := lookupTrimmed(lookup, EnvTelegramMTProtoSessionPath); ok {
		cfg.MTProto.SessionPath = rawSessionPath
	}

	if rawProxyEnabled, ok := lookupTrimmed(lookup, EnvTelegramMTProxyEnabled); ok {
		enabled, err := parseBool(rawProxyEnabled)
		if err != nil {
			return Config{}, fmt.Errorf("parse %s: %w", EnvTelegramMTProxyEnabled, err)
		}
		cfg.MTProto.ProxyEnabled = enabled
	}

	if rawRevealIPs, ok := lookupTrimmed(lookup, EnvTelegramTGNetRevealIPs); ok {
		revealIPs, err := parseBool(rawRevealIPs)
		if err != nil {
			return Config{}, fmt.Errorf("parse %s: %w", EnvTelegramTGNetRevealIPs, err)
		}
		cfg.TGNetRevealIPs = revealIPs
	}

	if cfg.Token == "" {
		return cfg, nil
	}

	owners, err := parseOwnerUserIDs(lookupTrimmedValue(lookup, EnvTelegramOwnerUserIDs))
	if err != nil {
		return Config{}, err
	}
	if len(owners) == 0 {
		return Config{}, fmt.Errorf("%s is required when %s is set", EnvTelegramOwnerUserIDs, EnvTelegramTrishnaToken)
	}
	cfg.OwnerUserIDs = owners

	allowedChats, err := parseAllowedChatIDs(lookupTrimmedValue(lookup, EnvTelegramAllowedChatIDs))
	if err != nil {
		return Config{}, err
	}
	cfg.AllowedChatIDs = allowedChats

	switch cfg.Transport {
	case TransportMTProto:
		mtproto, err := loadMTProtoConfig(lookup, cfg.MTProto)
		if err != nil {
			return Config{}, err
		}
		cfg.MTProto = mtproto
	case TransportBotAPI:
		if err := validateBotAPIBaseURL(cfg.APIBaseURL); err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}

func loadMTProtoConfig(lookup LookupEnv, cfg MTProtoConfig) (MTProtoConfig, error) {
	appID, err := parsePositiveInt(lookupTrimmedValue(lookup, EnvTelegramMTProtoAppID))
	if err != nil {
		return MTProtoConfig{}, fmt.Errorf("parse %s: %w", EnvTelegramMTProtoAppID, err)
	}
	cfg.AppID = appID

	appHash, _ := lookupTrimmed(lookup, EnvTelegramMTProtoAppHash)
	if appHash == "" {
		return MTProtoConfig{}, fmt.Errorf("%s is required when %s=%s", EnvTelegramMTProtoAppHash, EnvTelegramTransport, TransportMTProto)
	}
	cfg.AppHash = appHash

	if cfg.SessionPath == "" {
		return MTProtoConfig{}, fmt.Errorf("%s is required when %s=%s", EnvTelegramMTProtoSessionPath, EnvTelegramTransport, TransportMTProto)
	}

	if !cfg.ProxyEnabled {
		return cfg, nil
	}

	proxyHost, _ := lookupTrimmed(lookup, EnvTelegramMTProxyHost)
	if proxyHost == "" {
		return MTProtoConfig{}, fmt.Errorf("%s is required when %s=true", EnvTelegramMTProxyHost, EnvTelegramMTProxyEnabled)
	}
	cfg.ProxyHost = proxyHost

	proxyPort, err := parsePort(lookupTrimmedValue(lookup, EnvTelegramMTProxyPort))
	if err != nil {
		return MTProtoConfig{}, fmt.Errorf("parse %s: %w", EnvTelegramMTProxyPort, err)
	}
	cfg.ProxyPort = proxyPort

	proxySecret, err := parseMTProxySecret(lookupTrimmedValue(lookup, EnvTelegramMTProxySecret))
	if err != nil {
		return MTProtoConfig{}, fmt.Errorf("parse %s: %w", EnvTelegramMTProxySecret, err)
	}
	if err := validateMTProxySecretStrength(proxySecret); err != nil {
		return MTProtoConfig{}, fmt.Errorf("parse %s: %w", EnvTelegramMTProxySecret, err)
	}
	cfg.ProxySecret = proxySecret

	return cfg, nil
}

func parseTransport(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", TransportMTProto:
		return TransportMTProto, nil
	case TransportBotAPI:
		return TransportBotAPI, nil
	default:
		return "", fmt.Errorf("unsupported %s %q", EnvTelegramTransport, value)
	}
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "y", "on":
		return true, nil
	case "false", "0", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", value)
	}
}

func parsePositiveInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("value is required")
	}
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid positive integer %q", value)
	}
	return id, nil
}

func parsePort(value string) (int, error) {
	port, err := parsePositiveInt(value)
	if err != nil {
		return 0, err
	}
	if port > 65535 {
		return 0, fmt.Errorf("port out of range %q", value)
	}
	return port, nil
}

func parseMTProxySecret(value string) ([]byte, error) {
	value = strings.Trim(strings.TrimSpace(value), `"'`)
	if value == "" {
		return nil, fmt.Errorf("value is required")
	}

	if secret, ok := mtProxySecretFromURL(value); ok {
		value = secret
	}

	if secret, ok := decodeMTProxySecretHex(value); ok {
		return secret, nil
	}

	if secret, ok := decodeMTProxySecretBase64(value); ok {
		return secret, nil
	}

	return nil, fmt.Errorf("unsupported secret encoding")
}

func validateMTProxySecretStrength(raw []byte) error {
	secret, err := mtproxy.ParseSecret(raw)
	if err != nil {
		return err
	}
	if secret.Type == mtproxy.Simple {
		return fmt.Errorf("simple MTProxy secrets are not allowed; use secured or TLS MTProxy secret")
	}
	return nil
}

func validateBotAPIBaseURL(value string) error {
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("parse %s: %w", EnvTelegramAPIBaseURL, err)
	}
	if !strings.EqualFold(parsed.Scheme, "http") {
		return nil
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("%s plain HTTP URL must include a host", EnvTelegramAPIBaseURL)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		return fmt.Errorf("%s plain HTTP is only allowed for loopback or private local addresses", EnvTelegramAPIBaseURL)
	}
	return nil
}

func mtProxySecretFromURL(value string) (string, bool) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", false
	}
	secret := parsed.Query().Get("secret")
	if secret == "" {
		return "", false
	}
	return secret, true
}

func decodeMTProxySecretHex(value string) ([]byte, bool) {
	normalized := strings.TrimPrefix(strings.TrimSpace(value), "0x")
	if normalized == "" || len(normalized)%2 != 0 {
		return nil, false
	}

	secret, err := hex.DecodeString(normalized)
	if err != nil {
		return nil, false
	}
	if _, err := mtproxy.ParseSecret(secret); err != nil {
		return nil, false
	}
	return secret, true
}

func decodeMTProxySecretBase64(value string) ([]byte, bool) {
	value = strings.TrimSpace(value)
	encodings := []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	}
	for _, encoding := range encodings {
		secret, err := encoding.DecodeString(value)
		if err != nil {
			continue
		}
		if _, err := mtproxy.ParseSecret(secret); err != nil {
			continue
		}
		return secret, true
	}
	return nil, false
}

func parseOwnerUserIDs(value string) ([]int64, error) {
	return parseTelegramIDs(value, EnvTelegramOwnerUserIDs, false)
}

func parseAllowedChatIDs(value string) ([]int64, error) {
	return parseTelegramIDs(value, EnvTelegramAllowedChatIDs, true)
}

func parseTelegramIDs(value, envName string, allowNegative bool) ([]int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		raw := strings.TrimSpace(part)
		if raw == "" {
			continue
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id == 0 || (!allowNegative && id < 0) {
			return nil, fmt.Errorf("parse %s: invalid Telegram ID %q", envName, raw)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func lookupTrimmedValue(lookup LookupEnv, key string) string {
	value, _ := lookupTrimmed(lookup, key)
	return value
}

func lookupTrimmed(lookup LookupEnv, key string) (string, bool) {
	value, ok := lookup(key)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}
