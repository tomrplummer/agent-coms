---
name: telegram-away
description: Use the coms CLI to send Telegram updates and wait for replies when the user explicitly asks for away-from-keyboard messaging, notifications, Telegram check-ins, or async response handling.
---

# Telegram Away

## When to Use

Use this skill only when the user explicitly asks for Telegram notifications or away-from-keyboard communication, such as:

- "message me on Telegram"
- "notify me when done"
- "I will be away"
- "wait for my response"

Do not use this skill for normal in-terminal collaboration.

## Preconditions

- `COMS_TELEGRAM_BOT_TOKEN` is set.
- `coms init-chat` has been run at least once to store `chat_id`.

## Workflow

1. If asking for user input, generate or reuse a request ID (`rid`).
2. Send concise context and one direct question:

```bash
coms send --rid <rid> --message "<question or status>"
```

3. If a reply is required before continuing, wait with timeout:

```bash
coms wait --timeout-sec 900
```

`wait` uses the pending request automatically. Use `--rid <rid>` only when targeting a specific request.

4. If non-blocking behavior is needed, poll once:

```bash
coms poll
```

`poll` also uses the pending request automatically.

5. On completion/failure, send final status message when user asked for async updates:

```bash
coms send --rid <rid> --message "Task complete: <result>"
```

## Message Conventions

- Keep a single pending question whenever possible.
- The user should reply directly in Telegram; they do not need to type `rid`.
- Keep `rid` in sent prompts for machine correlation.
- Keep messages short and actionable.
- Ask one decision question per message.
- Include key context needed to answer from a phone.

## Error and Retry Guidance

- Exit code `2` from `wait` means timeout; send a follow-up or continue with default behavior if user specified one.
- Exit code `3` means config/auth/input problem; report clear setup steps.
- Exit code `4` means Telegram API problem; include retry context.
