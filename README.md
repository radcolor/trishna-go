# Trishna Go

Trishna is a modular personal bot written in Go. V0 is Discord-first, with room for future services and modules such as feed notifications and assistant providers.

## Status

Current skeleton includes:

- Discord gateway startup with `disgo`
- Environment-only config
- Slash command sync
- Module registry
- `/ping` command
- YouTube RSS poller with Discord webhook notifications
- Docker build target

## Configuration

Copy `.env.example` values into your runtime environment:

```sh
DISCORD_TOKEN=your-bot-token
DISCORD_GUILD_ID=optional-dev-guild-id
LOG_LEVEL=info
DISCORD_WEBHOOK_SHNKPLAYS=https://discord.com/api/webhooks/...
```

`DISCORD_GUILD_ID` is optional. When set, slash commands sync to that guild for faster development. When unset, commands sync globally.

`DISCORD_WEBHOOK_SHNKPLAYS` is optional. When set, Trishna polls the hardcoded shnk YouTube channel every 5 seconds and posts new uploads or live streams to that Discord webhook. The bot does not post these updates itself; Discord receives them through the webhook URL.

## YouTube Webhook Setup

1. In Discord, open the target channel's settings.
2. Go to **Integrations → Webhooks → New Webhook**.
3. Name the webhook `shnkplays`, choose the channel, and copy the webhook URL.
4. Set `DISCORD_WEBHOOK_SHNKPLAYS` in your environment.
5. Start the bot.

On first run, Trishna records the newest feed entry as a baseline and does not post older videos. After that, only new uploads or live streams are sent. State is stored in `data/youtube-state.json`.

## Run

```sh
go run ./cmd/trishna
```

## Test

```sh
go test ./...
```

## Docker

```sh
docker build -t trishna-go .
docker run --rm \
  -e DISCORD_TOKEN="$DISCORD_TOKEN" \
  -e DISCORD_GUILD_ID="$DISCORD_GUILD_ID" \
  -e DISCORD_WEBHOOK_SHNKPLAYS="$DISCORD_WEBHOOK_SHNKPLAYS" \
  -v "$(pwd)/data:/data" \
  trishna-go
```

## Module Direction

Modules own their commands and interaction handlers. Future service adapters should call shared module logic instead of embedding platform behavior in module internals.
