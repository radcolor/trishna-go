package config

import (
	"log/slog"
	"testing"
)

func TestLoadFromLookupEnvAllowsMissingDiscordToken(t *testing.T) {
	cfg, err := LoadFromLookupEnv(mapLookup(nil))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DiscordToken != "" {
		t.Fatalf("token = %q", cfg.DiscordToken)
	}
}

func TestLoadFromLookupEnvDefaults(t *testing.T) {
	cfg, err := LoadFromLookupEnv(mapLookup(map[string]string{
		EnvDiscordTrishnaToken: " token ",
	}))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DiscordToken != "token" {
		t.Fatalf("token = %q", cfg.DiscordToken)
	}
	if cfg.HasDiscordGuildID {
		t.Fatal("expected no guild id")
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Fatalf("log level = %v", cfg.LogLevel)
	}
}

func TestLoadFromLookupEnvLegacyDiscordTokenFallback(t *testing.T) {
	cfg, err := LoadFromLookupEnv(mapLookup(map[string]string{
		EnvDiscordToken: "legacy-token",
	}))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DiscordToken != "legacy-token" {
		t.Fatalf("token = %q", cfg.DiscordToken)
	}
}

func TestLoadFromLookupEnvOptionalGuildAndLogLevel(t *testing.T) {
	cfg, err := LoadFromLookupEnv(mapLookup(map[string]string{
		EnvDiscordTrishnaToken: "token",
		EnvDiscordGuildID:      "123456789",
		EnvLogLevel:            "debug",
	}))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.HasDiscordGuildID {
		t.Fatal("expected guild id")
	}
	if cfg.DiscordGuildID.String() != "123456789" {
		t.Fatalf("guild id = %s", cfg.DiscordGuildID)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Fatalf("log level = %v", cfg.LogLevel)
	}
}

func TestLoadFromLookupEnvRejectsInvalidGuild(t *testing.T) {
	_, err := LoadFromLookupEnv(mapLookup(map[string]string{
		EnvDiscordTrishnaToken: "token",
		EnvDiscordGuildID:      "not-a-snowflake",
	}))
	if err == nil {
		t.Fatal("expected invalid guild id error")
	}
}

func mapLookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
