# AI Install Guide for `coms`

This guide is for AI assistants that need to act like an install wizard for a user.

Goal: get `coms` installed, Telegram wired, and agent skills copied so the user can send/receive Telegram messages from AI workflows.

## Wizard Rules

1. Keep the user moving. Ask one question at a time only when needed.
2. Prefer defaults and concrete commands.
3. Confirm each milestone before continuing.
4. Do not ask the user to manually edit config files unless automation fails.
5. Never ask the user to paste secrets into chat history if avoidable.

## Inputs To Collect First

Ask the user these in order:

1. OS and shell:
   - Linux/macOS + bash/zsh
   - Windows PowerShell
   - Windows cmd.exe
2. Which AI tool(s) they use:
   - OpenCode
   - Codex
   - Cursor
3. Whether Go 1.22+ is already installed.
4. Whether they already created a Telegram bot token.

## Success Criteria

Installation is complete when all are true:

- `coms --help` runs successfully.
- `coms init-chat` succeeds and stores chat routing.
- `coms send` then `coms wait` succeeds with a real Telegram reply.
- Skill folder is installed for each requested AI tool.

## Step-by-Step Wizard Flow

### Step 1: Prerequisites

- Verify Go:

```bash
go version
```

- If missing, have user install Go 1.22+ first, then continue.

### Step 2: Telegram Bot Setup

Walk the user through:

1. Open BotFather in Telegram.
2. Run `/newbot`.
3. Save the token.
4. Open DM with the bot (`https://t.me/<bot_username>`).
5. Send at least one message to the bot.

### Step 3: Set `COMS_TELEGRAM_BOT_TOKEN`

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

```text
setx COMS_TELEGRAM_BOT_TOKEN "<token>"
```

### Step 4: Build/Install `coms`

From the repo root:

Linux/macOS local build:

```bash
go build -o ./bin/coms ./cmd/coms
```

Windows local build:

```powershell
go build -o .\bin\coms.exe .\cmd\coms
```

Or Go install (all platforms):

```bash
go install github.com/tomrplummer/agent-coms/cmd/coms@latest
```

### Step 5: Verify CLI Runs

Repo build binary:

- Linux/macOS: `./bin/coms --help`
- Windows: `.\bin\coms.exe --help`

Go install binary:

- Linux/macOS: `~/go/bin/coms --help`
- Windows: `%USERPROFILE%\go\bin\coms.exe --help`

### Step 6: Initialize Telegram Chat Routing

- Run:

```bash
coms init-chat
```

If using local binary path:

- Linux/macOS: `./bin/coms init-chat`
- Windows: `.\bin\coms.exe init-chat`

### Step 7: End-to-End Test

1. Send test:

```bash
coms send --message "Install wizard test: please reply Hi"
```

2. User replies in Telegram.
3. Wait for response:

```bash
coms wait --timeout-sec 120
```

Expected: JSON status `ok` with received message details.

## Skill Installation

Install the skill for whichever tools the user selected.

### OpenCode

Linux/macOS:

```bash
mkdir -p ~/.config/opencode/skills
cp -R skills/telegram-away ~/.config/opencode/skills/
```

Windows PowerShell:

```powershell
New-Item -ItemType Directory -Force "$HOME\.config\opencode\skills" | Out-Null
Copy-Item -Recurse -Force "skills\telegram-away" "$HOME\.config\opencode\skills\"
```

### Codex

Linux/macOS:

```bash
mkdir -p ~/.codex/skills
cp -R skills/telegram-away ~/.codex/skills/
```

Windows PowerShell:

```powershell
New-Item -ItemType Directory -Force "$HOME\.codex\skills" | Out-Null
Copy-Item -Recurse -Force "skills\telegram-away" "$HOME\.codex\skills\"
```

### Cursor

Personal skill (Linux/macOS):

```bash
mkdir -p ~/.cursor/skills
cp -R skills/telegram-away ~/.cursor/skills/
```

Personal skill (Windows PowerShell):

```powershell
New-Item -ItemType Directory -Force "$HOME\.cursor\skills" | Out-Null
Copy-Item -Recurse -Force "skills\telegram-away" "$HOME\.cursor\skills\"
```

Project skill (Linux/macOS):

```bash
mkdir -p .cursor/skills
cp -R skills/telegram-away .cursor/skills/
```

Project skill (Windows PowerShell):

```powershell
New-Item -ItemType Directory -Force ".cursor\skills" | Out-Null
Copy-Item -Recurse -Force "skills\telegram-away" ".cursor\skills\"
```

After copying skills: restart AI sessions/chats.

## Troubleshooting Playbook

### `missing_bot_token`

- Token env var is not set in current shell.
- Re-set `COMS_TELEGRAM_BOT_TOKEN` and retry.

### `chat_not_found` from `init-chat`

- User has not sent a DM to the bot yet, or wrong bot token.
- Have user send a fresh DM to the bot and rerun `coms init-chat`.

### `telegram_api_error` or transport failure

- Verify token is correct.
- Verify internet access.
- Retry after short delay.

### `wait` times out

- User did not reply in time.
- Ask user to reply in Telegram, then rerun `coms wait --timeout-sec 120`.

### Command not found

- Use absolute binary path or add binary directory to `PATH`.

## Suggested AI Prompt Pattern

When acting as wizard, use this message pattern repeatedly:

1. Tell user current step and why.
2. Provide exact command(s) for their OS/shell.
3. Ask for result/output.
4. Branch based on output.

Example:

```text
Step 3/7: set your Telegram token so coms can authenticate.
Run this in PowerShell:
$env:COMS_TELEGRAM_BOT_TOKEN = "<token>"
Then run:
go run ./cmd/coms --help
Paste the output and I will move to chat initialization.
```

## Final Handoff Checklist

Before declaring done, confirm:

- `coms --help` works.
- `init-chat` succeeded.
- One send/wait exchange succeeded.
- Skills installed for requested tools.
- User knows how to re-run update commands after future pulls.
