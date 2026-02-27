# coms

`coms` is a small Go CLI that lets coding agents ask you questions over Telegram and continue work once you reply.

It is designed for away-from-keyboard workflows in Codex/OpenCode/Cursor where the agent can:

- send a concise decision request,
- wait or poll for your response,
- correlate the reply reliably with a request ID (`rid`).

## What this project includes

- `coms` CLI (`cmd/coms`)
- Telegram API client (`internal/telegram`)
- Request correlation helpers (`internal/correlation`)
- Local offset/pending state (`internal/state`)
- Shared agent skill package (`skills/telegram-away`)

## Requirements

- Go 1.22+
- A Telegram bot token in `COMS_TELEGRAM_BOT_TOKEN`
- A private Telegram chat with your bot

## Quick setup

1. Create a bot with BotFather and copy the token.
2. Set environment variable:

```bash
export COMS_TELEGRAM_BOT_TOKEN="<token>"
```

3. Build and install:

```bash
go build -o ~/.local/bin/coms ./cmd/coms
```

4. Send your bot one DM in Telegram.
5. Initialize chat routing:

```bash
coms init-chat
```

Defaults written by `init-chat`:

- config: `~/.config/coms/config.toml`
- state: `~/.local/state/coms/state.json`

## Environment variables

Required:

- `COMS_TELEGRAM_BOT_TOKEN` - bot token from BotFather

Optional (path overrides):

- `XDG_CONFIG_HOME` - controls where config defaults are resolved
- `XDG_STATE_HOME` - controls where state defaults are resolved

You can copy `.env.example` as a reference for local env setup.

## CLI usage

Send a question (RID auto-generated if omitted):

```bash
coms send --message "Need your decision on deploy window"
```

Wait for a reply (uses pending request automatically):

```bash
coms wait --timeout-sec 900
```

Poll once (non-blocking):

```bash
coms poll
```

Target a specific request explicitly when needed:

```bash
coms wait --rid deploy42 --timeout-sec 900
coms poll --rid deploy42
```

Manual offset advance:

```bash
coms ack --update-id 123456
```

## Reply behavior

- Preferred: reply directly to the Telegram message from the bot.
- You do not need to type `[rid:...]` manually in normal flow.
- `send` stores one pending request in local state (new sends replace old pending).
- Successful `wait`/`poll` clears the matched pending request.
- `wait` timeout does not clear pending, so a late reply can still match later.

## Skill setup for agents

Skill source in this repo:

- `skills/telegram-away/SKILL.md`
- `skills/telegram-away/agents/openai.yaml`

### OpenCode

Install/update the skill package:

```bash
mkdir -p ~/.config/opencode/skills
cp -R skills/telegram-away ~/.config/opencode/skills/
```

Restart OpenCode sessions after updating skills.

### Codex

Install/update the same skill package:

```bash
mkdir -p ~/.codex/skills
cp -R skills/telegram-away ~/.codex/skills/
```

Restart Codex sessions after updating skills.

### Cursor

Install as a personal or project skill.

Personal skill (available everywhere):

```bash
mkdir -p ~/.cursor/skills
cp -R skills/telegram-away ~/.cursor/skills/
```

Project skill (checked into repo for teammates):

```bash
mkdir -p .cursor/skills
cp -R skills/telegram-away .cursor/skills/
```

Restart Cursor chats after updating skills.

## Exit codes

- `0` success
- `2` timeout (`wait` only)
- `3` config/auth/input error
- `4` Telegram API or transport error

## Security notes

- Keep `COMS_TELEGRAM_BOT_TOKEN` secret.
- `coms` matches messages only for configured `chat_id`.
- Avoid group chats for this workflow unless you want shared participants to potentially satisfy replies.
