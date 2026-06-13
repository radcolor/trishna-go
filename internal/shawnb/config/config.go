package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/disgoorg/snowflake/v2"

	"github.com/radcolor/trishna-go/internal/config"
)

const (
	EnvDiscordShawnbToken      = "DISCORD_SHAWNB_TOKEN"
	EnvDiscordShawnbGuildID    = "DISCORD_SHAWNB_GUILD_ID"
	EnvShawnbAllowedUserIDs    = "SHAWNB_ALLOWED_USER_IDS"
	EnvShawnbOwnerUserID       = "SHAWNB_OWNER_USER_ID"
	EnvShawnbAllowedChannelIDs = "SHAWNB_ALLOWED_CHANNEL_IDS"
	EnvOllamaBaseURL           = "OLLAMA_BASE_URL"
	EnvOllamaModel             = "OLLAMA_MODEL"
	EnvSoulMDPath              = "SOUL_MD_PATH"
	EnvShawnbChatLogDir        = "SHAWNB_CHAT_LOG_DIR"
	EnvShawnbHistoryLimit      = "SHAWNB_HISTORY_LIMIT"
	EnvShawnbHeartbeatPath     = "SHAWNB_HEARTBEAT_PATH"
	EnvShawnbRemindersPath     = "SHAWNB_REMINDERS_PATH"
	EnvLogLevel                = "LOG_LEVEL"
)

const (
	defaultOllamaBaseURL     = "http://127.0.0.1:11434"
	defaultOllamaModel       = "gemma4:e2b"
	defaultSoulMDPath        = "./SOUL.md"
	defaultShawnbChatLogDir   = "./data/shawnb/chats"
	defaultShawnbHeartbeatPath = "data/shawnb/heartbeat.json"
	defaultShawnbRemindersPath = "data/shawnb/reminders.json"
	defaultShawnbHistoryLimit = 20
)

type Config struct {
	DiscordToken         string
	DiscordGuildID       snowflake.ID
	HasDiscordGuildID    bool
	LogLevel             slog.Level
	AllowedUserIDs       []snowflake.ID
	OwnerUserID          snowflake.ID
	HasOwnerUserID       bool
	AllowedChannelIDs    []snowflake.ID
	OllamaBaseURL        string
	OllamaModel          string
	SoulMDPath           string
	ChatLogDir           string
	HeartbeatPath        string
	RemindersPath        string
	HistoryLimit         int
}

type LookupEnv func(key string) (string, bool)

func LoadFromEnv() (Config, error) {
	return LoadFromLookupEnv(sysLookupEnv)
}

func LoadFromLookupEnv(lookup LookupEnv) (Config, error) {
	token, ok := lookupTrimmed(lookup, EnvDiscordShawnbToken)
	if !ok {
		return Config{}, errors.New("DISCORD_SHAWNB_TOKEN is required")
	}

	cfg := Config{
		DiscordToken:  token,
		LogLevel:      slog.LevelInfo,
		OllamaBaseURL: defaultOllamaBaseURL,
		OllamaModel:   defaultOllamaModel,
		SoulMDPath:    defaultSoulMDPath,
		ChatLogDir:    defaultShawnbChatLogDir,
		HeartbeatPath: defaultShawnbHeartbeatPath,
		RemindersPath: defaultShawnbRemindersPath,
		HistoryLimit:  defaultShawnbHistoryLimit,
	}

	if rawGuildID, ok := lookupTrimmed(lookup, EnvDiscordShawnbGuildID); ok {
		guildID, err := snowflake.Parse(rawGuildID)
		if err != nil {
			return Config{}, fmt.Errorf("parse DISCORD_SHAWNB_GUILD_ID: %w", err)
		}
		cfg.DiscordGuildID = guildID
		cfg.HasDiscordGuildID = true
	}

	if rawLevel, ok := lookupTrimmed(lookup, EnvLogLevel); ok {
		level, err := parseLogLevel(rawLevel)
		if err != nil {
			return Config{}, err
		}
		cfg.LogLevel = level
	}

	allowedUsers, err := parseIDList(lookupTrimmedValue(lookup, EnvShawnbAllowedUserIDs))
	if err != nil {
		return Config{}, fmt.Errorf("parse SHAWNB_ALLOWED_USER_IDS: %w", err)
	}
	if len(allowedUsers) == 0 {
		return Config{}, errors.New("SHAWNB_ALLOWED_USER_IDS is required")
	}
	cfg.AllowedUserIDs = allowedUsers

	if rawOwner, ok := lookupTrimmed(lookup, EnvShawnbOwnerUserID); ok {
		ownerID, err := snowflake.Parse(rawOwner)
		if err != nil {
			return Config{}, fmt.Errorf("parse SHAWNB_OWNER_USER_ID: %w", err)
		}
		cfg.OwnerUserID = ownerID
		cfg.HasOwnerUserID = true
	} else {
		return Config{}, errors.New("SHAWNB_OWNER_USER_ID is required")
	}

	allowedChannels, err := parseIDList(lookupTrimmedValue(lookup, EnvShawnbAllowedChannelIDs))
	if err != nil {
		return Config{}, fmt.Errorf("parse SHAWNB_ALLOWED_CHANNEL_IDS: %w", err)
	}
	cfg.AllowedChannelIDs = allowedChannels

	if rawBaseURL, ok := lookupTrimmed(lookup, EnvOllamaBaseURL); ok {
		cfg.OllamaBaseURL = rawBaseURL
	}
	if rawModel, ok := lookupTrimmed(lookup, EnvOllamaModel); ok {
		cfg.OllamaModel = rawModel
	}
	if rawSoul, ok := lookupTrimmed(lookup, EnvSoulMDPath); ok {
		cfg.SoulMDPath = rawSoul
	}
	if rawLogDir, ok := lookupTrimmed(lookup, EnvShawnbChatLogDir); ok {
		cfg.ChatLogDir = rawLogDir
	}
	if rawHeartbeat, ok := lookupTrimmed(lookup, EnvShawnbHeartbeatPath); ok {
		cfg.HeartbeatPath = rawHeartbeat
	}
	if rawReminders, ok := lookupTrimmed(lookup, EnvShawnbRemindersPath); ok {
		cfg.RemindersPath = rawReminders
	}
	if rawLimit, ok := lookupTrimmed(lookup, EnvShawnbHistoryLimit); ok {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit < 1 {
			return Config{}, fmt.Errorf("parse SHAWNB_HISTORY_LIMIT: %q", rawLimit)
		}
		cfg.HistoryLimit = limit
	}

	return cfg, nil
}

func (c Config) BotConfig() config.Config {
	return config.Config{
		DiscordToken:      c.DiscordToken,
		DiscordGuildID:    c.DiscordGuildID,
		HasDiscordGuildID: c.HasDiscordGuildID,
		LogLevel:          c.LogLevel,
	}
}

func lookupTrimmedValue(lookup LookupEnv, key string) string {
	value, ok := lookupTrimmed(lookup, key)
	if !ok {
		return ""
	}
	return value
}

func lookupTrimmed(lookup LookupEnv, key string) (string, bool) {
	value, ok := lookup(key)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

func parseLogLevel(value string) (slog.Level, error) {
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

func parseIDList(raw string) ([]snowflake.ID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	var ids []snowflake.ID
	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := snowflake.Parse(part)
		if err != nil {
			return nil, fmt.Errorf("parse entry %q: %w", part, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
