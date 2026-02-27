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

1. Send concise context and one direct question:

```bash
coms send --message "<question or status>"
```

2. Immediately wait for reply before doing more work:

```bash
coms wait --timeout-sec 900
```

`wait` uses the pending message automatically.

This is mandatory behavior: **every `coms send` must be followed by `coms wait`** before continuing.

Do not continue execution after sending a Telegram message until `wait` returns `status: ok`, unless the user explicitly instructs fire-and-forget notifications.

3. Use non-blocking polling only when user explicitly asks not to block:

```bash
coms poll
```

`poll` also uses the pending message automatically.

4. On completion/failure, send final status message when user asked for async updates:

```bash
coms send --message "Task complete: <result>"
```

## Message Conventions

- Keep a single pending question whenever possible.
- The user should reply directly in Telegram.
- Do not add request IDs in messages.
- Keep messages short and actionable.
- Ask one decision question per message.
- Include key context needed to answer from a phone.
- For decision points, do not "notify and continue"; block on `wait`.

## Execution Behavior

- If the user gives a clear instruction that does not require extra permission, execute it immediately.
- Do not ask permission-style prompts such as "should I do that now?" for normal safe actions.
- Ask only when truly required (ambiguous requirement, destructive/irreversible action, missing secret/credential).
- When the user asked for Telegram check-ins and a decision is needed, send then wait; do not continue without the reply.
- If multiple questions are needed, use repeated pairs: `send -> wait`, `send -> wait`.

## Error and Retry Guidance

- Exit code `2` from `wait` means timeout; send a follow-up or continue with default behavior if user specified one.
- Exit code `3` means config/auth/input problem; report clear setup steps.
- Exit code `4` means Telegram API problem; include retry context.

## Anti-Patterns (Do Not Do)

- Sending a decision question and then proceeding without `wait`.
- Sending any Telegram message and then proceeding without `wait`.
- Using `poll` as a replacement for `wait` unless user explicitly requested non-blocking behavior.
- Asking multiple decisions in one Telegram message.
- Asking "should I proceed" for straightforward requested work.
