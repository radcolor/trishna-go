package config

import (
	"log/slog"
	"strings"
	"testing"
)

func TestLoadFromLookupEnvRequiresTokenAndAllowlist(t *testing.T) {
	_, err := LoadFromLookupEnv(mapLookup(nil))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "DISCORD_SHAWNB_TOKEN") {
		t.Fatalf("expected token error, got %v", err)
	}

	_, err = LoadFromLookupEnv(mapLookup(map[string]string{
		EnvDiscordShawnbToken: "token",
	}))
	if err == nil {
		t.Fatal("expected allowlist error")
	}
	if !strings.Contains(err.Error(), "SHAWNB_ALLOWED_USER_IDS") {
		t.Fatalf("expected allowlist error, got %v", err)
	}

	_, err = LoadFromLookupEnv(mapLookup(map[string]string{
		EnvDiscordShawnbToken:   "token",
		EnvShawnbAllowedUserIDs: "123456789",
	}))
	if err == nil {
		t.Fatal("expected owner error")
	}
	if !strings.Contains(err.Error(), "SHAWNB_OWNER_USER_ID") {
		t.Fatalf("expected owner error, got %v", err)
	}
}

func TestLoadFromLookupEnvDefaults(t *testing.T) {
	cfg, err := LoadFromLookupEnv(mapLookup(map[string]string{
		EnvDiscordShawnbToken:   " token ",
		EnvShawnbAllowedUserIDs: "123456789",
		EnvShawnbOwnerUserID:    "987654321",
	}))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DiscordToken != "token" {
		t.Fatalf("token = %q", cfg.DiscordToken)
	}
	if cfg.OllamaBaseURL != defaultOllamaBaseURL {
		t.Fatalf("ollama base url = %q", cfg.OllamaBaseURL)
	}
	if cfg.OllamaModel != defaultOllamaModel {
		t.Fatalf("ollama model = %q", cfg.OllamaModel)
	}
	if cfg.HistoryLimit != defaultShawnbHistoryLimit {
		t.Fatalf("history limit = %d", cfg.HistoryLimit)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Fatalf("log level = %v", cfg.LogLevel)
	}
	if !cfg.HasOwnerUserID || cfg.OwnerUserID.String() != "987654321" {
		t.Fatalf("owner user id = %v has=%v", cfg.OwnerUserID, cfg.HasOwnerUserID)
	}
}

func mapLookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
