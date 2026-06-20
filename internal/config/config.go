package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/disgoorg/snowflake/v2"
)

const (
	EnvDiscordTrishnaToken  = "DISCORD_TRISHNA_TOKEN"
	EnvDiscordToken         = "DISCORD_TOKEN" // legacy fallback for Trishna
	EnvDiscordGuildID       = "DISCORD_GUILD_ID"
	EnvLogLevel             = "LOG_LEVEL"
	EnvStatusAllowedUserIDs = "STATUS_ALLOWED_USER_IDS"
	EnvShawnbHeartbeatPath  = "SHAWNB_HEARTBEAT_PATH"
	EnvOllamaBaseURL        = "OLLAMA_BASE_URL"
	EnvOllamaModel          = "OLLAMA_MODEL"
)

type Config struct {
	DiscordToken      string
	DiscordGuildID    snowflake.ID
	HasDiscordGuildID bool
	LogLevel          slog.Level
}

type LookupEnv func(key string) (string, bool)

func LoadFromEnv() (Config, error) {
	return LoadFromLookupEnv(sysLookupEnv)
}

func LoadFromLookupEnv(lookup LookupEnv) (Config, error) {
	token, ok := lookupTrimmed(lookup, EnvDiscordTrishnaToken)
	if !ok {
		token, ok = lookupTrimmed(lookup, EnvDiscordToken)
	}

	cfg := Config{
		LogLevel: slog.LevelInfo,
	}
	if ok {
		cfg.DiscordToken = token
	}

	if rawGuildID, ok := lookupTrimmed(lookup, EnvDiscordGuildID); ok {
		guildID, err := snowflake.Parse(rawGuildID)
		if err != nil {
			return Config{}, fmt.Errorf("parse DISCORD_GUILD_ID: %w", err)
		}
		cfg.DiscordGuildID = guildID
		cfg.HasDiscordGuildID = true
	}

	if rawLevel, ok := lookupTrimmed(lookup, EnvLogLevel); ok {
		level, err := ParseLogLevel(rawLevel)
		if err != nil {
			return Config{}, err
		}
		cfg.LogLevel = level
	}

	return cfg, nil
}

func ParseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported LOG_LEVEL %q", value)
	}
}

func lookupTrimmed(lookup LookupEnv, key string) (string, bool) {
	value, ok := lookup(key)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}
