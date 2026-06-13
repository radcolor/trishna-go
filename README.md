# Trishna Go

Trishna is a modular personal bot written in Go. V0 is Discord-first, with room for future services and modules such as feed notifications and assistant providers.

## Status

Current skeleton includes:

- Discord gateway startup with `disgo`
- Environment-only config
- Slash command sync
- Module registry
- `/ping` command
- Docker build target

## Configuration

Copy `.env.example` values into your runtime environment:

```sh
DISCORD_TOKEN=your-bot-token
DISCORD_GUILD_ID=optional-dev-guild-id
LOG_LEVEL=info
```

`DISCORD_GUILD_ID` is optional. When set, slash commands sync to that guild for faster development. When unset, commands sync globally.

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
docker run --rm -e DISCORD_TOKEN="$DISCORD_TOKEN" -e DISCORD_GUILD_ID="$DISCORD_GUILD_ID" trishna-go
```

## Module Direction

Modules own their commands and interaction handlers. Future service adapters should call shared module logic instead of embedding platform behavior in module internals.
