# enjoyable-stats

A CLI tool that downloads a CS2 demo file, parses it for custom game events, and posts insights to a Discord channel — but only for players who have linked their Steam ID via a Discord bot slash command.

## Prerequisites

- Go 1.26.4

## Configuration

Create a `.env` file in the project root (or set the variables in your environment):

```
DISCORD_TOKEN=your_bot_token_here
DISCORD_CHANNEL_ID=your_channel_id_here
```

- `DISCORD_TOKEN` — required. Your Discord bot token (from the Discord Developer Portal).
- `DISCORD_CHANNEL_ID` — default channel for posting insights. Can be overridden per run with the `-channel` flag.

A missing `.env` is not fatal; if the environment variables are already set, the file is not needed. A missing `DISCORD_TOKEN` is always fatal.

## Commands

### `bot` — run the persistent Discord bot

Starts a long-lived Discord session, registers the `/link-steam` and `/unlink-steam` slash commands in every guild the bot is in, and listens for interactions until `SIGINT`/`SIGTERM`.

```bash
# During development
go run . bot

# After building
./enjoyable-stats bot
```

### `analyse` — download, parse, and post insights (one-shot)

Downloads a demo, parses it for tracked players' highlights, saves the insights to the local database, and posts them to Discord.

```bash
# From a URL
go run . analyse -demo <url> [-channel <channel-id>]

# From a local file (.dem or .dem.bz2)
go run . analyse -file demos/match.dem [-channel <channel-id>]
go run . analyse -file demos/match.dem.bz2
```

| Flag | Required | Description |
|------|----------|-------------|
| `-demo` | one of `-demo` / `-file` | URL of the `.dem` or `.dem.bz2` demo file |
| `-file` | one of `-demo` / `-file` | Local path to a `.dem` or `.dem.bz2` file (skips download) |
| `-channel` | no | Discord channel ID (overrides `DISCORD_CHANNEL_ID` from config) |

Both `.dem` and `.dem.bz2` URLs and local files are supported. `.gz` is not handled in Phase 1.

## Building

```bash
go build -o enjoyable-stats .
```

## Discord Slash Commands

Once the bot is running, the following slash commands are available in any guild:

| Command | Description |
|---------|-------------|
| `/link-steam <steam-id>` | Links your 17-digit SteamID64 to your Discord account so your highlights are tracked and @mentioned in insights. |
| `/unlink-steam` | Removes your Steam ID link. You will no longer appear in demo insights. |

## Running Tests

```bash
go test ./...
```
