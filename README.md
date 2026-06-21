# Trishna Go

Trishna is a modular personal bot platform written in Go. It runs independent adapters in one process where possible, so one platform can fail without stopping the others. It currently includes Discord and Telegram adapters plus a separate Discord chat bot:

- **Trishna** — Discord slash commands, Telegram owner commands, Mac Mini `/status`, YouTube webhook notifications
- **shawnb** — Ollama AI chat bot with a gitignored `SOUL.md` personality file, DMs and allowed channels, chat logs, reminders, and owner alerts

## Status

### Trishna (`cmd/trishna`)

- Discord gateway startup with `disgo`
- Environment-only config
- Slash command sync
- Module registry
- Discord `/ping` command
- Telegram owner-only `/ping` command through long polling
- `/status` command (bot uptime, version, process stats, Mac Mini CPU/RAM/SSD, services)
- YouTube RSS poller with Discord webhook notifications
- YouTube live chat stream bot with basic `!` commands
- Docker build target

### shawnb (`cmd/shawnb`)

- Separate Discord bot process
- Replies to allowed users in DMs and configured channels
- Local Ollama inference (`gemma4:e2b` by default)
- Personality from gitignored `SOUL.md` (see `SOUL.md.example`)
- Natural-language reminders persisted to JSON and delivered on schedule
- Owner DM alerts when users send urgent or important messages
- Append-only JSONL chat logs under `data/shawnb/chats/`
- `/reset` slash command to clear in-memory history for a conversation

## Configuration

Copy `.env.example` values into your runtime environment:

```sh
DISCORD_TRISHNA_TOKEN=your-trishna-bot-token
DISCORD_GUILD_ID=optional-dev-guild-id
LOG_LEVEL=info
TELEGRAM_TRISHNA_TOKEN=your-telegram-bot-token
TELEGRAM_OWNER_USER_IDS=your-telegram-user-id
TELEGRAM_TRANSPORT=mtproto
TELEGRAM_MTPROTO_APP_ID=your-telegram-api-id
TELEGRAM_MTPROTO_APP_HASH=your-telegram-api-hash
TELEGRAM_MTPROTO_SESSION_PATH=data/telegram/mtproto-session.json
TELEGRAM_MTPROTO_PROXY_ENABLED=true
TELEGRAM_MTPROTO_PROXY_HOST=your-mtproxy-host
TELEGRAM_MTPROTO_PROXY_PORT=443
TELEGRAM_MTPROTO_PROXY_SECRET=your-mtproxy-secret-hex
TELEGRAM_API_BASE_URL=
DISCORD_WEBHOOK_SHNKPLAYS=https://discord.com/api/webhooks/...
YOUTUBE_CHAT_ENABLED=false
YOUTUBE_CLIENT_ID=your-google-oauth-client-id
YOUTUBE_CLIENT_SECRET=your-google-oauth-client-secret
YOUTUBE_TOKEN_PATH=data/youtube-token.json
YOUTUBE_OWNER_CHANNEL_IDS=your-youtube-channel-id
YOUTUBE_LIVE_VIDEO_ID=optional-live-video-id
STREAMBOT_STATE_PATH=data/streambot/state.json
STREAMBOT_RESPONSES_DIR=data/streambot/responses
STATUS_ALLOWED_USER_IDS=

DISCORD_SHAWNB_TOKEN=your-shawnb-bot-token
DISCORD_SHAWNB_GUILD_ID=optional-dev-guild-id
SHAWNB_ALLOWED_USER_IDS=allowed-user-discord-id
SHAWNB_OWNER_USER_ID=owner-discord-user-id
SHAWNB_ALLOWED_CHANNEL_IDS=private-channel-id
OLLAMA_BASE_URL=http://127.0.0.1:11434
OLLAMA_MODEL=gemma4:e2b
SOUL_MD_PATH=./SOUL.md
SHAWNB_CHAT_LOG_DIR=./data/shawnb/chats
SHAWNB_HEARTBEAT_PATH=data/shawnb/heartbeat.json
SHAWNB_HISTORY_LIMIT=20
```

`DISCORD_TRISHNA_TOKEN` enables Trishna's Discord adapter. `DISCORD_TOKEN` still works as a legacy fallback. If Discord config is missing or invalid, the process can still run other enabled adapters such as YouTube chat.

`TELEGRAM_TRISHNA_TOKEN` enables Trishna's Telegram adapter. `TELEGRAM_OWNER_USER_IDS` is required when the Telegram token is set; only those numeric Telegram user IDs can run commands in DMs or groups.

`TELEGRAM_TRANSPORT=mtproto` is the default. It uses Telegram MTProto through MTProxy by default, so `TELEGRAM_MTPROTO_APP_ID`, `TELEGRAM_MTPROTO_APP_HASH`, `TELEGRAM_MTPROTO_PROXY_HOST`, `TELEGRAM_MTPROTO_PROXY_PORT`, and `TELEGRAM_MTPROTO_PROXY_SECRET` are required. The MTProto session is stored at `TELEGRAM_MTPROTO_SESSION_PATH` with `0600` file permissions.

Set `TELEGRAM_MTPROTO_PROXY_ENABLED=false` only when you intentionally want direct MTProto traffic, such as routing Telegram through WireGuard outside Trishna. Set `TELEGRAM_TRANSPORT=botapi` to use the old cloud/local Bot API path; `TELEGRAM_API_BASE_URL` can point to a local Bot API server such as `http://127.0.0.1:8081`.

Trishna `/status` includes a **shawnb** section (Discord connection via heartbeat file). shawnb writes `data/shawnb/heartbeat.json` every 10 seconds while connected.

`DISCORD_GUILD_ID` is optional. When set, Trishna slash commands sync to that guild for faster development. When unset, commands sync globally.

`DISCORD_SHAWNB_GUILD_ID` works the same way for shawnb's `/reset` command.

`DISCORD_WEBHOOK_SHNKPLAYS` is optional. When set, Trishna polls the hardcoded shnk YouTube channel every 5 seconds and posts new uploads or live streams to that Discord webhook. The bot does not post these updates itself; Discord receives them through the webhook URL.

`YOUTUBE_CHAT_ENABLED=true` enables the YouTube stream bot. Run `go run ./cmd/trishna auth youtube` once after setting `YOUTUBE_CLIENT_ID`, `YOUTUBE_CLIENT_SECRET`, and `YOUTUBE_TOKEN_PATH`; the command prints a Google consent URL and saves the OAuth token after the browser callback. The OAuth client must allow a loopback redirect URI for desktop apps.

`YOUTUBE_OWNER_CHANNEL_IDS` is a comma-separated list of YouTube channel IDs allowed to run owner-only stream commands such as `!setgame`. Trishna also uses these channel IDs to auto-detect the active public live stream via `/channel/{id}/live` when OAuth's owned-broadcast lookup cannot see the stream.

`YOUTUBE_LIVE_VIDEO_ID` is optional. Set it only when auto-detection cannot find the stream, such as some unlisted/private broadcasts.

Trishna auto-detects the current stream game from the broadcast title and tags. Supported game buckets are `sky`, `valorant`, and `generic`; owner-only `!setgame` can override detection during a stream.

`STREAMBOT_RESPONSES_DIR` points at editable text replies. These optional files override built-in fallback replies:

- `socials.txt`
- `valorant.txt`
- `sky.txt`

`STATUS_ALLOWED_USER_IDS` is required when the Discord adapter is enabled. Only those Discord user IDs can run `/status`.

`SHAWNB_ALLOWED_USER_IDS` is required for shawnb. Only those users can chat with the bot.

`SHAWNB_OWNER_USER_ID` is required for shawnb. That Discord user receives DM alerts when an allowed user sends something urgent (needs owner, contact request, emotional, security, etc.).

`SHAWNB_ALLOWED_CHANNEL_IDS` is optional. When set, shawnb also replies in those guild channels (in addition to DMs). Enable **Message Content Intent** and **Direct Messages** in the shawnb Discord application settings.

## shawnb setup

1. Create a second Discord application/bot for shawnb.
2. Enable **Message Content Intent** and **Direct Messages** under Bot settings.
3. Copy `SOUL.md.example` to `SOUL.md` and customize personality (keep `SOUL.md` gitignored).
4. Pull an Ollama model: `ollama pull gemma4:e2b`
5. Set shawnb env vars in `.env` and run `./cmd/shawnb` or install the launchd service.

Read chat logs later:

```sh
tail -f "$HOME/Library/Application Support/trishna-go/data/shawnb/chats/$(date +%F).jsonl"
```

Each line is JSON with `role`, `content`, `user_id`, `channel_id`, `is_dm`, and `ts`.

## YouTube Webhook Setup

1. In Discord, open the target channel's settings.
2. Go to **Integrations → Webhooks → New Webhook**.
3. Name the webhook `shnkplays`, choose the channel, and copy the webhook URL.
4. Set `DISCORD_WEBHOOK_SHNKPLAYS` in your environment.
5. Start Trishna.

On first run, Trishna records the newest feed entry as a baseline and does not post older videos. After that, only new uploads or live streams are sent. State is stored in `data/youtube-state.json`.

## YouTube Stream Bot Setup

1. Enable YouTube Data API v3 in Google Cloud.
2. Create an OAuth client for a desktop app.
3. Set `YOUTUBE_CLIENT_ID`, `YOUTUBE_CLIENT_SECRET`, `YOUTUBE_TOKEN_PATH`, and `YOUTUBE_OWNER_CHANNEL_IDS`.
4. Run:

```sh
go run ./cmd/trishna auth youtube
```

5. Open the printed URL, approve access, and wait for the terminal to save the token.
6. Set `YOUTUBE_CHAT_ENABLED=true` and start Trishna.

Supported live chat commands:

- `!commands`
- `!game`
- `!specs`
- `!crosshair`
- `!isekai`
- `!valorant`
- `!sky`
- `!generic`
- `!socials`
- `!ping` (owner only)
- `!status` (owner only)
- `!uptime` (owner only)
- `!setgame valorant`, `!setgame sky`, or `!setgame generic` (owner only)

When the current game is `valorant`, `sky`, or `generic`, Trishna posts a compact welcome/promo message on connect and every 30 minutes. This keeps YouTube API usage low: one chat poll per YouTube's returned interval plus at most two scheduled promo messages per hour.

## Run

Trishna:

```sh
go run ./cmd/trishna
```

shawnb (requires Ollama running and `SOUL.md`):

```sh
go run ./cmd/shawnb
```

## Deploy on macOS (Mac Mini)

Use the included `launchd` services to auto-start on login, restart on crash, and write logs.

1. Copy `.env.example` to `.env` and fill in secrets.
2. Copy `SOUL.md.example` to `SOUL.md` for shawnb.
3. Install Trishna:

```sh
chmod +x deploy/macos/*.sh
./deploy/macos/install.sh
```

4. Install shawnb:

```sh
./deploy/macos/install-shawnb.sh
```

Trishna writes:

- `logs/trishna.log`
- `logs/trishna.error.log`

shawnb writes:

- `$HOME/Library/Application Support/trishna-go/logs/shawnb.log`
- `$HOME/Library/Application Support/trishna-go/logs/shawnb.error.log`
- `$HOME/Library/Application Support/trishna-go/data/shawnb/chats/YYYY-MM-DD.jsonl`

Useful commands:

```sh
./deploy/macos/status.sh
./deploy/macos/status-shawnb.sh
tail -f logs/trishna.log
tail -f "$HOME/Library/Application Support/trishna-go/logs/shawnb.log"
tail -f "$HOME/Library/Application Support/trishna-go/data/shawnb/chats/$(date +%F).jsonl"
./deploy/macos/restart.sh
./deploy/macos/restart-shawnb.sh
./deploy/macos/restart-all.sh
./deploy/macos/uninstall.sh
./deploy/macos/uninstall-shawnb.sh
```

Both services are user launch agents (`com.radcolor.trishna`, `com.radcolor.shawnb`), so they start when your Mac Mini user logs in. Enable automatic login in **System Settings → Users & Groups** if you want them running after every reboot without manual sign-in.

`install.sh` and `install-shawnb.sh` read `.env` and inject values into the launch agents. Keep `.env` gitignored and re-run the install or restart script after changing secrets.

## Test

```sh
go test ./...
```

## Docker

Docker currently builds Trishna only:

```sh
docker build -t trishna-go .
docker run --rm \
  -e DISCORD_TRISHNA_TOKEN="$DISCORD_TRISHNA_TOKEN" \
  -e DISCORD_GUILD_ID="$DISCORD_GUILD_ID" \
  -e DISCORD_WEBHOOK_SHNKPLAYS="$DISCORD_WEBHOOK_SHNKPLAYS" \
  -e YOUTUBE_CHAT_ENABLED="$YOUTUBE_CHAT_ENABLED" \
  -e YOUTUBE_CLIENT_ID="$YOUTUBE_CLIENT_ID" \
  -e YOUTUBE_CLIENT_SECRET="$YOUTUBE_CLIENT_SECRET" \
  -e YOUTUBE_TOKEN_PATH="/data/youtube-token.json" \
  -e YOUTUBE_OWNER_CHANNEL_IDS="$YOUTUBE_OWNER_CHANNEL_IDS" \
  -v "$(pwd)/data:/data" \
  trishna-go
```

## Module Direction

Modules own their commands and interaction handlers. Future service adapters should call shared module logic instead of embedding platform behavior in module internals.
