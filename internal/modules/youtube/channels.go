package youtube

const (
	EnvWebhookShnkplays = "DISCORD_WEBHOOK_SHNKPLAYS"
)

type Channel struct {
	ID            string
	Name          string
	WebhookEnvKey string
}

var Channels = []Channel{
	{
		ID:            "UCRMARomI-6vCRfOXQOayBPg",
		Name:          "shnk",
		WebhookEnvKey: EnvWebhookShnkplays,
	},
}

func FeedURL(channelID string) string {
	return "https://www.youtube.com/feeds/videos.xml?channel_id=" + channelID
}
