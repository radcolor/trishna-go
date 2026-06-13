# Trishna Go

Trishna is a modular personal bot platform written in Go. It runs two separate Discord bots as independent processes:

- **Trishna** — slash commands, Mac Mini `/status`, YouTube webhook notifications
- **shawnb** — Ollama AI chat bot with a gitignored `SOUL.md` personality file, DMs and allowed channels, chat logs, reminders, and owner alerts

## Status

### Trishna (`cmd/trishna`)

- Discord gateway startup with `disgo`
- Environment-only config
- Slash command sync
- Module registry
- `/ping` command
- `/status` command (bot uptime, version, process stats, Mac Mini CPU/RAM/SSD, services)
- YouTube RSS poller with Discord webhook notifications
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
DISCORD_WEBHOOK_SHNKPLAYS=https://discord.com/api/webhooks/...
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

`DISCORD_TRISHNA_TOKEN` is required for Trishna. `DISCORD_TOKEN` still works as a legacy fallback.

Trishna `/status` includes a **shawnb** section (Discord connection via heartbeat file). shawnb writes `data/shawnb/heartbeat.json` every 10 seconds while connected.

`DISCORD_GUILD_ID` is optional. When set, Trishna slash commands sync to that guild for faster development. When unset, commands sync globally.

`DISCORD_SHAWNB_GUILD_ID` works the same way for shawnb's `/reset` command.

`DISCORD_WEBHOOK_SHNKPLAYS` is optional. When set, Trishna polls the hardcoded shnk YouTube channel every 5 seconds and posts new uploads or live streams to that Discord webhook. The bot does not post these updates itself; Discord receives them through the webhook URL.

`STATUS_ALLOWED_USER_IDS` is optional. When set, only those Discord user IDs can run `/status`. When unset, anyone can use it.

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
tail -f data/shawnb/chats/$(date +%F).jsonl
```

Each line is JSON with `role`, `content`, `user_id`, `channel_id`, `is_dm`, and `ts`.

## YouTube Webhook Setup

1. In Discord, open the target channel's settings.
2. Go to **Integrations → Webhooks → New Webhook**.
3. Name the webhook `shnkplays`, choose the channel, and copy the webhook URL.
4. Set `DISCORD_WEBHOOK_SHNKPLAYS` in your environment.
5. Start Trishna.

On first run, Trishna records the newest feed entry as a baseline and does not post older videos. After that, only new uploads or live streams are sent. State is stored in `data/youtube-state.json`.

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

- `logs/shawnb.log`
- `logs/shawnb.error.log`
- `data/shawnb/chats/YYYY-MM-DD.jsonl`

Useful commands:

```sh
./deploy/macos/status.sh
./deploy/macos/status-shawnb.sh
tail -f logs/trishna.log
tail -f logs/shawnb.log
tail -f data/shawnb/chats/$(date +%F).jsonl
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
  -v "$(pwd)/data:/data" \
  trishna-go
```

## Module Direction

Modules own their commands and interaction handlers. Future service adapters should call shared module logic instead of embedding platform behavior in module internals.
