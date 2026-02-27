# coms

`coms` is a small Go CLI that lets coding agents ask you questions over Telegram and continue work once you reply.

It is designed for away-from-keyboard workflows in Codex/OpenCode/Cursor where the agent can:

- send a concise decision request,
- wait or poll for your response,
- correlate the reply reliably with a request ID (`rid`).

## Platform support

- Linux
- macOS
- Windows (PowerShell or cmd.exe)
- Command name stays `coms` (the Windows binary file is `coms.exe`)

## What this project includes

- `coms` CLI (`cmd/coms`)
- Telegram API client (`internal/telegram`)
- Request correlation helpers (`internal/correlation`)
- Local offset/pending state (`internal/state`)
- Shared agent skill package (`skills/telegram-away`)

## Requirements

- Go 1.22+
- A Telegram account
- A Telegram bot token in `COMS_TELEGRAM_BOT_TOKEN`
- A private Telegram DM with your bot

## Telegram setup help

1. In Telegram, open **BotFather** and run `/newbot`.
2. Save the bot token BotFather gives you.
3. Open a private DM with your new bot (`https://t.me/<your_bot_username>`).
4. Send at least one message to the bot (for example: `hello`).
5. Keep this workflow in a private DM, not a group chat.

If `coms init-chat` says `chat_not_found`:

- confirm you sent a DM to the same bot token,
- send one more message to the bot,
- run `init-chat` again.

## Quick setup

1. Set bot token environment variable.

Linux/macOS (bash/zsh):

```bash
export COMS_TELEGRAM_BOT_TOKEN="<token>"
```

Windows PowerShell:

```powershell
$env:COMS_TELEGRAM_BOT_TOKEN = "<token>"
```

Windows cmd.exe:

```bat
set COMS_TELEGRAM_BOT_TOKEN=<token>
```

Optional persistence across new shells:

- PowerShell or cmd.exe: `setx COMS_TELEGRAM_BOT_TOKEN "<token>"`

2. Build the CLI from this repo.

Linux/macOS:

```bash
go build -o ./bin/coms ./cmd/coms
```

Windows PowerShell or cmd.exe:

```powershell
go build -o .\bin\coms.exe .\cmd\coms
```

3. Initialize chat routing (detects and stores your `chat_id`).

Linux/macOS:

```bash
./bin/coms init-chat
```

Windows:

```powershell
.\bin\coms.exe init-chat
```

4. Send and receive one test message.

Linux/macOS:

```bash
./bin/coms send --message "Test from coms"
./bin/coms wait --timeout-sec 120
```

Windows:

```powershell
.\bin\coms.exe send --message "Test from coms"
.\bin\coms.exe wait --timeout-sec 120
```

## Alternative install (Go toolchain)

You can also install directly with:

```bash
go install github.com/tomrplummer/agent-coms/cmd/coms@latest
```

Then ensure your Go bin directory is on `PATH`.

- Linux/macOS default: `~/go/bin/coms`
- Windows default: `%USERPROFILE%\go\bin\coms.exe`

## Default file locations

Defaults written by `init-chat`:

| OS | Config path | State path |
| --- | --- | --- |
| Linux/macOS | `~/.config/coms/config.toml` | `~/.local/state/coms/state.json` |
| Windows | `%USERPROFILE%\\.config\\coms\\config.toml` | `%USERPROFILE%\\.local\\state\\coms\\state.json` |

## Environment variables

Required:

- `COMS_TELEGRAM_BOT_TOKEN` - bot token from BotFather

Optional (path overrides, all platforms):

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

Linux/macOS:

```bash
mkdir -p ~/.config/opencode/skills
cp -R skills/telegram-away ~/.config/opencode/skills/
```

Windows (PowerShell):

```powershell
New-Item -ItemType Directory -Force "$HOME\\.config\\opencode\\skills" | Out-Null
Copy-Item -Recurse -Force "skills\\telegram-away" "$HOME\\.config\\opencode\\skills\\"
```

Restart OpenCode sessions after updating skills.

### Codex

Linux/macOS:

```bash
mkdir -p ~/.codex/skills
cp -R skills/telegram-away ~/.codex/skills/
```

Windows (PowerShell):

```powershell
New-Item -ItemType Directory -Force "$HOME\\.codex\\skills" | Out-Null
Copy-Item -Recurse -Force "skills\\telegram-away" "$HOME\\.codex\\skills\\"
```

Restart Codex sessions after updating skills.

### Cursor

Install as a personal or project skill.

Personal skill (available everywhere) - Linux/macOS:

```bash
mkdir -p ~/.cursor/skills
cp -R skills/telegram-away ~/.cursor/skills/
```

Personal skill (available everywhere) - Windows (PowerShell):

```powershell
New-Item -ItemType Directory -Force "$HOME\\.cursor\\skills" | Out-Null
Copy-Item -Recurse -Force "skills\\telegram-away" "$HOME\\.cursor\\skills\\"
```

Project skill (checked into repo for teammates) - Linux/macOS:

```bash
mkdir -p .cursor/skills
cp -R skills/telegram-away .cursor/skills/
```

Project skill (checked into repo for teammates) - Windows (PowerShell):

```powershell
New-Item -ItemType Directory -Force ".cursor\\skills" | Out-Null
Copy-Item -Recurse -Force "skills\\telegram-away" ".cursor\\skills\\"
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
