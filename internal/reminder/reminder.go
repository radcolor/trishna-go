package reminder

import (
	"time"

	"github.com/disgoorg/snowflake/v2"
)

const (
	DefaultPath       = "data/shawnb/reminders.json"
	LocationName      = "Asia/Kolkata"
	minLeadTime       = 30 * time.Second
	maxFutureDuration = 365 * 24 * time.Hour
	maxSendAttempts   = 5
	tickInterval      = 15 * time.Second
	maxDiscordMessage = 2000
)

type Reminder struct {
	ID              string        `json:"id"`
	UserID          snowflake.ID  `json:"user_id"`
	ChannelID       snowflake.ID  `json:"channel_id"`
	Event           string        `json:"event"`
	DueAt           time.Time     `json:"due_at"`
	CreatedAt       time.Time     `json:"created_at"`
	OriginalMessage string        `json:"original_message,omitempty"`
	SendAttempts    int           `json:"send_attempts,omitempty"`
}

type fileData struct {
	Reminders []Reminder `json:"reminders"`
}
