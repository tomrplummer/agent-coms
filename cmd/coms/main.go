package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tomrplummer/agent-coms/internal/config"
	"github.com/tomrplummer/agent-coms/internal/state"
	"github.com/tomrplummer/agent-coms/internal/telegram"
)

const (
	exitOK          = 0
	exitTimeout     = 2
	exitConfigError = 3
	exitAPIError    = 4
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return exitConfigError
	}

	switch args[0] {
	case "help", "--help", "-h":
		printUsage()
		return exitOK
	case "init-chat":
		return runInitChat(args[1:])
	case "send":
		return runSend(args[1:])
	case "wait":
		return runWait(args[1:])
	case "poll":
		return runPoll(args[1:])
	case "ack":
		return runAck(args[1:])
	default:
		printError(exitConfigError, "invalid_command", fmt.Sprintf("unknown command %q", args[0]), nil)
		return exitConfigError
	}
}

func printUsage() {
	fmt.Println("coms <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init-chat   Detect Telegram chat_id from recent updates and save config")
	fmt.Println("  send        Send message")
	fmt.Println("  wait        Wait for matching reply")
	fmt.Println("  poll        Check once for matching reply")
	fmt.Println("  ack         Advance update offset manually")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  COMS_TELEGRAM_BOT_TOKEN  Telegram bot token")
}

type commandContext struct {
	configPath string
	cfg        config.Config
	statePath  string
	st         state.State
	client     *telegram.Client
}

func runInitChat(args []string) int {
	fs := flag.NewFlagSet("init-chat", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var configPath string
	fs.StringVar(&configPath, "config", "", "path to config.toml")
	if err := fs.Parse(args); err != nil {
		printError(exitConfigError, "invalid_flags", err.Error(), nil)
		return exitConfigError
	}

	ctx, code := loadContext(configPath, true)
	if code != exitOK {
		return code
	}

	nowCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	updates, err := ctx.client.GetUpdates(nowCtx, ctx.st.UpdateOffset, 1)
	if err != nil {
		return handleTelegramError(err)
	}

	var latest *telegram.Message
	var latestUpdateID int64 = -1
	for _, update := range updates {
		msg := messageFromUpdate(update)
		if msg == nil {
			continue
		}
		if msg.Chat.Type != "private" {
			continue
		}
		if update.UpdateID >= latestUpdateID {
			latest = msg
			latestUpdateID = update.UpdateID
		}
	}

	if latest == nil {
		extra := map[string]any{
			"hint": "send a message to your bot first, then run init-chat again",
		}
		printError(exitConfigError, "chat_not_found", "no private chat updates found", extra)
		return exitConfigError
	}

	ctx.cfg.Telegram.ChatID = latest.Chat.ID
	if latest.From != nil && latest.From.Username != "" {
		ctx.cfg.Telegram.Username = latest.From.Username
	} else if latest.Chat.Username != "" {
		ctx.cfg.Telegram.Username = latest.Chat.Username
	}

	if err := config.Save(ctx.configPath, ctx.cfg); err != nil {
		printError(exitConfigError, "save_config_failed", err.Error(), nil)
		return exitConfigError
	}

	if latestUpdateID >= 0 {
		state.AdvanceOffset(&ctx.st, latestUpdateID)
		if err := state.Save(ctx.statePath, ctx.st); err != nil {
			printError(exitConfigError, "save_state_failed", err.Error(), nil)
			return exitConfigError
		}
	}

	printJSON(map[string]any{
		"status":      "ok",
		"chat_id":     ctx.cfg.Telegram.ChatID,
		"username":    ctx.cfg.Telegram.Username,
		"config_path": ctx.configPath,
		"state_path":  ctx.statePath,
	})
	return exitOK
}

func runSend(args []string) int {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var configPath string
	var messageText string
	var tag string
	fs.StringVar(&configPath, "config", "", "path to config.toml")
	fs.StringVar(&messageText, "message", "", "message text")
	fs.StringVar(&tag, "tag", "", "optional tag")
	if err := fs.Parse(args); err != nil {
		printError(exitConfigError, "invalid_flags", err.Error(), nil)
		return exitConfigError
	}
	messageText = strings.TrimSpace(messageText)
	if messageText == "" {
		printError(exitConfigError, "missing_message", "--message is required", nil)
		return exitConfigError
	}

	ctx, code := loadContext(configPath, false)
	if code != exitOK {
		return code
	}

	outbound := composeOutboundMessage(messageText, tag)
	msg, err := ctx.client.SendMessage(context.Background(), ctx.cfg.Telegram.ChatID, outbound)
	if err != nil {
		return handleTelegramError(err)
	}

	state.SetPending(&ctx.st, state.PendingRequest{
		ChatID:        ctx.cfg.Telegram.ChatID,
		SentMessageID: msg.MessageID,
		SentAtUnix:    msg.Date,
	})
	if err := state.Save(ctx.statePath, ctx.st); err != nil {
		printError(exitConfigError, "save_state_failed", err.Error(), nil)
		return exitConfigError
	}

	printJSON(map[string]any{
		"status":       "ok",
		"chat_id":      ctx.cfg.Telegram.ChatID,
		"message_id":   msg.MessageID,
		"sent_at_unix": msg.Date,
	})
	return exitOK
}

func runWait(args []string) int {
	fs := flag.NewFlagSet("wait", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var configPath string
	var timeoutSec int
	fs.StringVar(&configPath, "config", "", "path to config.toml")
	fs.IntVar(&timeoutSec, "timeout-sec", 0, "timeout in seconds")
	if err := fs.Parse(args); err != nil {
		printError(exitConfigError, "invalid_flags", err.Error(), nil)
		return exitConfigError
	}

	ctx, code := loadContext(configPath, false)
	if code != exitOK {
		return code
	}

	selection := resolveSelection(&ctx.st, ctx.cfg.Telegram.ChatID)

	if timeoutSec <= 0 {
		timeoutSec = ctx.cfg.Telegram.DefaultTimeoutSec
	}
	if timeoutSec <= 0 {
		timeoutSec = 900
	}

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			printJSON(map[string]any{
				"status":      "timeout",
				"timeout_sec": timeoutSec,
			})
			return exitTimeout
		}

		pollSeconds := int(remaining.Seconds())
		if pollSeconds > 30 {
			pollSeconds = 30
		}
		if pollSeconds < 1 {
			pollSeconds = 1
		}

		updates, err := ctx.client.GetUpdates(context.Background(), ctx.st.UpdateOffset, pollSeconds)
		if err != nil {
			if retryErr, ok := err.(telegram.RetryAfterError); ok {
				sleepFor := time.Duration(retryErr.RetryAfterSec) * time.Second
				if sleepFor > remaining {
					sleepFor = remaining
				}
				time.Sleep(sleepFor)
				continue
			}
			return handleTelegramError(err)
		}

		matched, changed := applyUpdates(&ctx.st, updates, ctx.cfg.Telegram.ChatID, selection, 0)
		if matched != nil && clearPendingForSelection(&ctx.st, selection) {
			changed = true
		}
		if changed {
			if err := state.Save(ctx.statePath, ctx.st); err != nil {
				printError(exitConfigError, "save_state_failed", err.Error(), nil)
				return exitConfigError
			}
		}
		if matched != nil {
			printJSON(map[string]any{
				"status":        "ok",
				"update_id":     matched.UpdateID,
				"message_id":    matched.Message.MessageID,
				"message_text":  matched.Message.Text,
				"sender":        senderLabel(matched.Message),
				"received_unix": matched.Message.Date,
			})
			return exitOK
		}
	}
}

func runPoll(args []string) int {
	fs := flag.NewFlagSet("poll", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var configPath string
	var sinceRaw string
	fs.StringVar(&configPath, "config", "", "path to config.toml")
	fs.StringVar(&sinceRaw, "since", "", "unix timestamp or RFC3339")
	if err := fs.Parse(args); err != nil {
		printError(exitConfigError, "invalid_flags", err.Error(), nil)
		return exitConfigError
	}

	since, err := parseSince(sinceRaw)
	if err != nil {
		printError(exitConfigError, "invalid_since", err.Error(), nil)
		return exitConfigError
	}

	ctx, code := loadContext(configPath, false)
	if code != exitOK {
		return code
	}

	selection := resolveSelection(&ctx.st, ctx.cfg.Telegram.ChatID)

	updates, err := ctx.client.GetUpdates(context.Background(), ctx.st.UpdateOffset, 0)
	if err != nil {
		return handleTelegramError(err)
	}

	matched, changed := applyUpdates(&ctx.st, updates, ctx.cfg.Telegram.ChatID, selection, since)
	if matched != nil && clearPendingForSelection(&ctx.st, selection) {
		changed = true
	}
	if changed {
		if err := state.Save(ctx.statePath, ctx.st); err != nil {
			printError(exitConfigError, "save_state_failed", err.Error(), nil)
			return exitConfigError
		}
	}

	if matched == nil {
		printJSON(map[string]any{
			"status": "no_match",
		})
		return exitOK
	}

	printJSON(map[string]any{
		"status":        "ok",
		"update_id":     matched.UpdateID,
		"message_id":    matched.Message.MessageID,
		"message_text":  matched.Message.Text,
		"sender":        senderLabel(matched.Message),
		"received_unix": matched.Message.Date,
	})
	return exitOK
}

func runAck(args []string) int {
	fs := flag.NewFlagSet("ack", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var configPath string
	var updateID int64
	fs.StringVar(&configPath, "config", "", "path to config.toml")
	fs.Int64Var(&updateID, "update-id", -1, "update id to acknowledge")
	if err := fs.Parse(args); err != nil {
		printError(exitConfigError, "invalid_flags", err.Error(), nil)
		return exitConfigError
	}
	if updateID < 0 {
		printError(exitConfigError, "invalid_update_id", "--update-id must be >= 0", nil)
		return exitConfigError
	}

	ctx, code := loadContext(configPath, false)
	if code != exitOK {
		return code
	}

	before := ctx.st.UpdateOffset
	state.AdvanceOffset(&ctx.st, updateID)
	if ctx.st.UpdateOffset != before {
		if err := state.Save(ctx.statePath, ctx.st); err != nil {
			printError(exitConfigError, "save_state_failed", err.Error(), nil)
			return exitConfigError
		}
	}

	printJSON(map[string]any{
		"status":         "ok",
		"acknowledged":   updateID,
		"current_offset": ctx.st.UpdateOffset,
	})
	return exitOK
}

func loadContext(configPathArg string, allowMissingChat bool) (*commandContext, int) {
	resolvedConfigPath := config.DefaultConfigPath()
	if strings.TrimSpace(configPathArg) != "" {
		var err error
		resolvedConfigPath, err = config.ResolvePath(configPathArg)
		if err != nil {
			printError(exitConfigError, "invalid_config_path", err.Error(), nil)
			return nil, exitConfigError
		}
	}

	cfg, err := config.Load(resolvedConfigPath)
	if err != nil {
		printError(exitConfigError, "load_config_failed", err.Error(), nil)
		return nil, exitConfigError
	}

	if !allowMissingChat && cfg.Telegram.ChatID == 0 {
		printError(exitConfigError, "missing_chat_id", "chat_id is not set; run init-chat first", nil)
		return nil, exitConfigError
	}

	token := strings.TrimSpace(os.Getenv("COMS_TELEGRAM_BOT_TOKEN"))
	if token == "" {
		printError(exitConfigError, "missing_bot_token", "COMS_TELEGRAM_BOT_TOKEN is not set", nil)
		return nil, exitConfigError
	}

	statePath, err := config.ResolvePath(cfg.State.OffsetStore)
	if err != nil {
		printError(exitConfigError, "invalid_state_path", err.Error(), nil)
		return nil, exitConfigError
	}
	st, err := state.Load(statePath)
	if err != nil {
		printError(exitConfigError, "load_state_failed", err.Error(), nil)
		return nil, exitConfigError
	}

	return &commandContext{
		configPath: resolvedConfigPath,
		cfg:        cfg,
		statePath:  statePath,
		st:         st,
		client:     telegram.NewClient(token),
	}, exitOK
}

type matchedUpdate struct {
	UpdateID int64
	Message  *telegram.Message
}

type matchSelection struct {
	Pending         *state.PendingRequest
	AllowAnyMessage bool
}

func applyUpdates(st *state.State, updates []telegram.Update, chatID int64, selection matchSelection, since int64) (*matchedUpdate, bool) {
	before := st.UpdateOffset

	for _, update := range updates {
		state.AdvanceOffset(st, update.UpdateID)
		msg := messageFromUpdate(update)
		if msg == nil {
			continue
		}
		if msg.Chat.ID != chatID {
			continue
		}
		if since > 0 && msg.Date < since {
			continue
		}
		if selection.Pending != nil {
			if messageMatchesPendingFallback(msg, selection.Pending) {
				return &matchedUpdate{UpdateID: update.UpdateID, Message: msg}, st.UpdateOffset != before
			}
			continue
		}
		if selection.AllowAnyMessage && strings.TrimSpace(msg.Text) != "" {
			return &matchedUpdate{UpdateID: update.UpdateID, Message: msg}, st.UpdateOffset != before
		}
	}

	return nil, st.UpdateOffset != before
}

func resolveSelection(st *state.State, chatID int64) matchSelection {
	pending := currentPendingForChat(st, chatID)
	if pending == nil {
		return matchSelection{AllowAnyMessage: true}
	}
	return matchSelection{Pending: pending}
}

func currentPendingForChat(st *state.State, chatID int64) *state.PendingRequest {
	if st == nil || st.Pending == nil {
		return nil
	}
	pending := st.Pending
	if pending.ChatID != 0 && pending.ChatID != chatID {
		return nil
	}
	return pending
}

func clearPendingForSelection(st *state.State, selection matchSelection) bool {
	if st == nil || st.Pending == nil || selection.Pending == nil {
		return false
	}
	if selection.Pending.ChatID != 0 && st.Pending.ChatID != selection.Pending.ChatID {
		return false
	}
	if selection.Pending.SentMessageID > 0 && st.Pending.SentMessageID != selection.Pending.SentMessageID {
		return false
	}
	state.ClearPending(st)
	return true
}

func messageFromUpdate(update telegram.Update) *telegram.Message {
	if update.Message != nil {
		return update.Message
	}
	if update.EditedMessage != nil {
		return update.EditedMessage
	}
	return nil
}

func messageMatchesPendingFallback(msg *telegram.Message, pending *state.PendingRequest) bool {
	if msg == nil || pending == nil {
		return false
	}
	if pending.ChatID != 0 && msg.Chat.ID != pending.ChatID {
		return false
	}
	if strings.TrimSpace(msg.Text) == "" {
		return false
	}
	if pending.SentMessageID > 0 && msg.MessageID <= pending.SentMessageID {
		return false
	}
	if pending.SentAtUnix > 0 && msg.Date < pending.SentAtUnix {
		return false
	}
	if msg.ReplyToMessage != nil {
		if pending.SentMessageID > 0 && msg.ReplyToMessage.MessageID == pending.SentMessageID {
			return true
		}
		return false
	}
	return true
}

func composeOutboundMessage(messageText, tag string) string {
	messageText = strings.TrimSpace(messageText)
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return messageText
	}
	return fmt.Sprintf("[tag:%s] %s", tag, messageText)
}

func senderLabel(msg *telegram.Message) string {
	if msg == nil || msg.From == nil {
		return "unknown"
	}
	if msg.From.Username != "" {
		return "@" + msg.From.Username
	}
	name := strings.TrimSpace(strings.TrimSpace(msg.From.FirstName + " " + msg.From.LastName))
	if name != "" {
		return name
	}
	return strconv.FormatInt(msg.From.ID, 10)
}

func parseSince(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	if unixTs, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return unixTs, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return 0, errors.New("--since must be unix timestamp or RFC3339")
	}
	return t.Unix(), nil
}

func handleTelegramError(err error) int {
	var retryErr telegram.RetryAfterError
	if errors.As(err, &retryErr) {
		printError(exitAPIError, "rate_limited", retryErr.Error(), map[string]any{"retry_after_sec": retryErr.RetryAfterSec})
		return exitAPIError
	}
	var apiErr telegram.APIError
	if errors.As(err, &apiErr) {
		printError(exitAPIError, "telegram_api_error", apiErr.Error(), map[string]any{"code": apiErr.Code})
		return exitAPIError
	}
	printError(exitAPIError, "telegram_request_failed", err.Error(), nil)
	return exitAPIError
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func printError(exitCode int, status string, message string, extra map[string]any) {
	payload := map[string]any{
		"status":    status,
		"error":     message,
		"exit_code": exitCode,
	}
	for k, v := range extra {
		payload[k] = v
	}
	printJSON(payload)
}
