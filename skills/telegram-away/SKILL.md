---
name: telegram-away
description: Use the coms CLI to send Telegram updates and wait for replies, including continuous listen mode until the user sends stop.
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

Use one of these two modes based on the user's request.

### One-Shot Mode (Default)

1. Send concise context and one direct question:

```bash
coms send --message "<question or status>"
```

2. Immediately wait for reply before doing more work. Use a per-call timeout and keep retrying on timeout so waiting is effectively indefinite:

```bash
while true; do
  coms wait --timeout-sec "${COMS_WAIT_TIMEOUT_SEC:-60}"
  rc=$?
  if [ "$rc" -eq 0 ]; then
    break
  fi
  if [ "$rc" -ne 2 ]; then
    exit "$rc"
  fi
done
```

3. Use non-blocking polling only when the user explicitly asks not to block:

```bash
coms poll
```

`wait` and `poll` use the pending message automatically.

Mandatory rule: **every `coms send` must be followed by `coms wait`** before continuing.

Do not continue execution after sending a Telegram message until `wait` returns `status: ok`, unless the user explicitly asks for fire-and-forget notifications.

### Listen Mode (Continuous Until Stop)

Use this when the user explicitly asks to "listen", "enter listen mode", "keep waiting", or "stay in loop until stop".

1. Announce mode and stop keyword:

```bash
coms send --message "Listening mode enabled. Send instructions anytime. Reply 'stop' to exit."
```

2. Immediately begin waiting with a per-call timeout. On timeout, keep waiting (do not exit listen mode):

```bash
while true; do
  coms wait --timeout-sec "${COMS_WAIT_TIMEOUT_SEC:-60}"
  rc=$?
  if [ "$rc" -eq 0 ]; then
    break
  fi
  if [ "$rc" -ne 2 ]; then
    exit "$rc"
  fi
done
```

3. For each `wait` result:

- If `status: ok`, treat `message_text` as the next instruction.
- If instruction is exactly `stop` or `/stop` (case-insensitive, surrounding whitespace ignored), send confirmation and exit listen mode:

```bash
coms send --message "Listening mode stopped. Returning to normal workflow."
```

- Otherwise, execute the instruction immediately (subject to normal safety rules), then send one concise result plus the next listening prompt:

```bash
coms send --message "Done: <result>. Waiting for next instruction. Reply 'stop' to exit."
```

- If `wait` exits with code `2` (`status: timeout`), stay in listen mode and run `wait` again immediately. Do not exit listen mode on timeout.

## Message Conventions

- Keep a single pending question whenever possible.
- The user should reply directly in Telegram.
- Do not add request IDs in messages.
- Keep messages short and actionable.
- Ask one decision question per message.
- Include key context needed to answer from a phone.
- For decision points, do not "notify and continue"; block on `wait`.
- In listen mode responses, end with a clear waiting cue (for example: "Waiting for next instruction. Reply 'stop' to exit.").

## Execution Behavior

- If the user gives a clear instruction that does not require extra permission, execute it immediately.
- Do not ask permission-style prompts such as "should I do that now?" for normal safe actions.
- Ask only when truly required (ambiguous requirement, destructive/irreversible action, missing secret/credential).
- In listen mode, run the repeated loop: `wait -> execute -> send result -> wait` until `stop`.
- If multiple questions are needed, use repeated pairs: `send -> wait`, `send -> wait`.

## Error and Retry Guidance

- Exit code `2` from `wait` means timeout.
  - One-shot mode: send a follow-up or continue with default behavior if the user specified one.
  - Listen mode: continue waiting and keep the loop active.
- Exit code `3` means config/auth/input problem; report clear setup steps.
- Exit code `4` means Telegram API problem; include retry context.

## Anti-Patterns (Do Not Do)

- Sending a decision question and then proceeding without `wait`.
- Sending any Telegram message and then proceeding without `wait`.
- Using `poll` as a replacement for `wait` unless user explicitly requested non-blocking behavior.
- Treating `--timeout-sec` as a total session timeout.
- Exiting listen mode on first timeout.
- Asking multiple decisions in one Telegram message.
- Asking "should I proceed" for straightforward requested work.
