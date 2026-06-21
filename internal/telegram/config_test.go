package telegram

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

const validTLSMTProxySecretHex = "ee852380f362a09343efb4690c4e17862e676f6f676c652e636f6d"

func TestLoadConfigFromLookupEnvDisabled(t *testing.T) {
	cfg, err := LoadConfigFromLookupEnv(mapLookup(nil))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Token != "" {
		t.Fatalf("token = %q", cfg.Token)
	}
	if len(cfg.OwnerUserIDs) != 0 {
		t.Fatalf("owners = %v", cfg.OwnerUserIDs)
	}
	if cfg.Transport != TransportMTProto {
		t.Fatalf("transport = %q", cfg.Transport)
	}
	if !cfg.MTProto.ProxyEnabled {
		t.Fatal("expected mtproxy enabled by default")
	}
	if cfg.MTProto.SessionPath != DefaultMTProtoSessionPath {
		t.Fatalf("session path = %q", cfg.MTProto.SessionPath)
	}
}

func TestLoadConfigFromLookupEnvBotAPI(t *testing.T) {
	cfg, err := LoadConfigFromLookupEnv(mapLookup(map[string]string{
		EnvTelegramTrishnaToken: " token ",
		EnvTelegramOwnerUserIDs: " 123, 456 ",
		EnvTelegramAPIBaseURL:   " http://127.0.0.1:8081/ ",
		EnvTelegramTransport:    TransportBotAPI,
	}))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Transport != TransportBotAPI {
		t.Fatalf("transport = %q", cfg.Transport)
	}
	if cfg.Token != "token" {
		t.Fatalf("token = %q", cfg.Token)
	}
	if cfg.APIBaseURL != "http://127.0.0.1:8081" {
		t.Fatalf("api base url = %q", cfg.APIBaseURL)
	}
	if len(cfg.OwnerUserIDs) != 2 || cfg.OwnerUserIDs[0] != 123 || cfg.OwnerUserIDs[1] != 456 {
		t.Fatalf("owners = %v", cfg.OwnerUserIDs)
	}
}

func TestLoadConfigFromLookupEnvMTProto(t *testing.T) {
	cfg, err := LoadConfigFromLookupEnv(mapLookup(map[string]string{
		EnvTelegramTrishnaToken:       " token ",
		EnvTelegramOwnerUserIDs:       " 123, 456 ",
		EnvTelegramMTProtoAppID:       "12345",
		EnvTelegramMTProtoAppHash:     "apphash",
		EnvTelegramMTProtoSessionPath: "data/custom-session.json",
		EnvTelegramMTProxyHost:        "proxy.example.com",
		EnvTelegramMTProxyPort:        "443",
		EnvTelegramMTProxySecret:      "0123456789abcdef0123456789abcdef",
	}))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Transport != TransportMTProto {
		t.Fatalf("transport = %q", cfg.Transport)
	}
	if cfg.MTProto.AppID != 12345 {
		t.Fatalf("app id = %d", cfg.MTProto.AppID)
	}
	if cfg.MTProto.AppHash != "apphash" {
		t.Fatalf("app hash = %q", cfg.MTProto.AppHash)
	}
	if cfg.MTProto.SessionPath != "data/custom-session.json" {
		t.Fatalf("session path = %q", cfg.MTProto.SessionPath)
	}
	if !cfg.MTProto.ProxyEnabled {
		t.Fatal("expected proxy enabled")
	}
	if cfg.MTProto.ProxyHost != "proxy.example.com" || cfg.MTProto.ProxyPort != 443 {
		t.Fatalf("proxy = %s:%d", cfg.MTProto.ProxyHost, cfg.MTProto.ProxyPort)
	}
	if len(cfg.MTProto.ProxySecret) == 0 {
		t.Fatal("expected proxy secret")
	}
}

func TestLoadConfigFromLookupEnvMTProtoBase64URLSecret(t *testing.T) {
	rawSecret, err := hex.DecodeString(validTLSMTProxySecretHex)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	cfg, err := LoadConfigFromLookupEnv(mapLookup(map[string]string{
		EnvTelegramTrishnaToken:   "token",
		EnvTelegramOwnerUserIDs:   "123",
		EnvTelegramMTProtoAppID:   "12345",
		EnvTelegramMTProtoAppHash: "apphash",
		EnvTelegramMTProxyHost:    "proxy.example.com",
		EnvTelegramMTProxyPort:    "443",
		EnvTelegramMTProxySecret:  base64.RawURLEncoding.EncodeToString(rawSecret),
	}))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got := hex.EncodeToString(cfg.MTProto.ProxySecret); got != validTLSMTProxySecretHex {
		t.Fatalf("proxy secret = %q", got)
	}
}

func TestLoadConfigFromLookupEnvMTProtoDirect(t *testing.T) {
	cfg, err := LoadConfigFromLookupEnv(mapLookup(map[string]string{
		EnvTelegramTrishnaToken:   "token",
		EnvTelegramOwnerUserIDs:   "123",
		EnvTelegramMTProtoAppID:   "12345",
		EnvTelegramMTProtoAppHash: "apphash",
		EnvTelegramMTProxyEnabled: "false",
	}))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.MTProto.ProxyEnabled {
		t.Fatal("expected proxy disabled")
	}
}

func TestLoadConfigFromLookupEnvMTProtoFailsClosedWithoutProxy(t *testing.T) {
	_, err := LoadConfigFromLookupEnv(mapLookup(map[string]string{
		EnvTelegramTrishnaToken:   "token",
		EnvTelegramOwnerUserIDs:   "123",
		EnvTelegramMTProtoAppID:   "12345",
		EnvTelegramMTProtoAppHash: "apphash",
	}))
	if err == nil {
		t.Fatal("expected missing proxy error")
	}
}

func TestLoadConfigFromLookupEnvRequiresOwnersWhenEnabled(t *testing.T) {
	_, err := LoadConfigFromLookupEnv(mapLookup(map[string]string{
		EnvTelegramTrishnaToken: "token",
	}))
	if err == nil {
		t.Fatal("expected missing owners error")
	}
}

func TestLoadConfigFromLookupEnvRejectsInvalidOwner(t *testing.T) {
	_, err := LoadConfigFromLookupEnv(mapLookup(map[string]string{
		EnvTelegramTrishnaToken: "token",
		EnvTelegramOwnerUserIDs: "123, nope",
		EnvTelegramTransport:    TransportBotAPI,
	}))
	if err == nil {
		t.Fatal("expected invalid owner error")
	}
}

func TestLoadConfigFromLookupEnvRejectsInvalidTransport(t *testing.T) {
	_, err := LoadConfigFromLookupEnv(mapLookup(map[string]string{
		EnvTelegramTransport: "bogus",
	}))
	if err == nil {
		t.Fatal("expected invalid transport error")
	}
}

func TestLoadConfigFromLookupEnvRejectsInvalidProxyPort(t *testing.T) {
	_, err := LoadConfigFromLookupEnv(mapLookup(map[string]string{
		EnvTelegramTrishnaToken:   "token",
		EnvTelegramOwnerUserIDs:   "123",
		EnvTelegramMTProtoAppID:   "12345",
		EnvTelegramMTProtoAppHash: "apphash",
		EnvTelegramMTProxyHost:    "proxy.example.com",
		EnvTelegramMTProxyPort:    "70000",
		EnvTelegramMTProxySecret:  "0123456789abcdef0123456789abcdef",
	}))
	if err == nil {
		t.Fatal("expected invalid proxy port error")
	}
}

func TestLoadConfigFromLookupEnvRejectsInvalidProxySecret(t *testing.T) {
	_, err := LoadConfigFromLookupEnv(mapLookup(map[string]string{
		EnvTelegramTrishnaToken:   "token",
		EnvTelegramOwnerUserIDs:   "123",
		EnvTelegramMTProtoAppID:   "12345",
		EnvTelegramMTProtoAppHash: "apphash",
		EnvTelegramMTProxyHost:    "proxy.example.com",
		EnvTelegramMTProxyPort:    "443",
		EnvTelegramMTProxySecret:  "not-hex",
	}))
	if err == nil {
		t.Fatal("expected invalid proxy secret error")
	}
}

func mapLookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
