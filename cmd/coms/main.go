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

	"coms/internal/config"
	"coms/internal/correlation"
	"coms/internal/state"
	"coms/internal/telegram"
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
	fmt.Println("  send        Send message with request id")
	fmt.Println("  wait        Wait for matching reply (request id or pending request)")
	fmt.Println("  poll        Check once for matching reply (request id or pending request)")
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
	var rid string
	var tag string
	fs.StringVar(&configPath, "config", "", "path to config.toml")
	fs.StringVar(&messageText, "message", "", "message text")
	fs.StringVar(&rid, "rid", "", "request id")
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

	rid = correlation.NormalizeRID(rid)
	if rid == "" {
		generated, err := correlation.GenerateRID()
		if err != nil {
			printError(exitConfigError, "rid_generation_failed", err.Error(), nil)
			return exitConfigError
		}
		rid = generated
	}
	if !correlation.IsValidRID(rid) {
		printError(exitConfigError, "invalid_rid", "rid must be 3-64 chars [a-z0-9_-]", nil)
		return exitConfigError
	}

	outbound := composeOutboundMessage(messageText, rid, tag)
	msg, err := ctx.client.SendMessage(context.Background(), ctx.cfg.Telegram.ChatID, outbound)
	if err != nil {
		return handleTelegramError(err)
	}

	state.SetPending(&ctx.st, state.PendingRequest{
		RID:           rid,
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
		"rid":          rid,
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
	var rid string
	var timeoutSec int
	fs.StringVar(&configPath, "config", "", "path to config.toml")
	fs.StringVar(&rid, "rid", "", "request id (optional if pending request exists)")
	fs.IntVar(&timeoutSec, "timeout-sec", 0, "timeout in seconds")
	if err := fs.Parse(args); err != nil {
		printError(exitConfigError, "invalid_flags", err.Error(), nil)
		return exitConfigError
	}

	ctx, code := loadContext(configPath, false)
	if code != exitOK {
		return code
	}

	selection, err := resolveRIDSelection(&ctx.st, ctx.cfg.Telegram.ChatID, rid)
	if err != nil {
		printError(exitConfigError, "invalid_rid", err.Error(), nil)
		return exitConfigError
	}
	rid = selection.RID

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
				"rid":         rid,
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
		matchRID := rid
		if matched != nil && matched.RID != "" {
			matchRID = matched.RID
		}
		if matchRID != "" && clearPendingForRID(&ctx.st, matchRID) {
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
				"rid":           matchRID,
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
	var rid string
	var sinceRaw string
	fs.StringVar(&configPath, "config", "", "path to config.toml")
	fs.StringVar(&rid, "rid", "", "request id (optional if pending request exists)")
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

	selection, err := resolveRIDSelection(&ctx.st, ctx.cfg.Telegram.ChatID, rid)
	if err != nil {
		printError(exitConfigError, "invalid_rid", err.Error(), nil)
		return exitConfigError
	}
	rid = selection.RID

	updates, err := ctx.client.GetUpdates(context.Background(), ctx.st.UpdateOffset, 0)
	if err != nil {
		return handleTelegramError(err)
	}

	matched, changed := applyUpdates(&ctx.st, updates, ctx.cfg.Telegram.ChatID, selection, since)
	matchRID := rid
	if matched != nil && matched.RID != "" {
		matchRID = matched.RID
	}
	if matchRID != "" && clearPendingForRID(&ctx.st, matchRID) {
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
			"rid":    matchRID,
		})
		return exitOK
	}

	printJSON(map[string]any{
		"status":        "ok",
		"rid":           matchRID,
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
	RID      string
}

type ridSelection struct {
	RID                string
	Pending            *state.PendingRequest
	AllowPlainFallback bool
	AllowAnyRID        bool
}

func applyUpdates(st *state.State, updates []telegram.Update, chatID int64, selection ridSelection, since int64) (*matchedUpdate, bool) {
	before := st.UpdateOffset
	targetRID := correlation.NormalizeRID(selection.RID)

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
		if targetRID != "" && messageMatchesRID(msg, targetRID) {
			return &matchedUpdate{UpdateID: update.UpdateID, Message: msg, RID: targetRID}, st.UpdateOffset != before
		}
		if selection.AllowAnyRID {
			if extractedRID, ok := extractRIDFromMessage(msg); ok {
				return &matchedUpdate{UpdateID: update.UpdateID, Message: msg, RID: extractedRID}, st.UpdateOffset != before
			}
		}
		if selection.AllowPlainFallback && selection.Pending != nil && messageMatchesPendingFallback(msg, selection.Pending) {
			fallbackRID := targetRID
			if fallbackRID == "" {
				fallbackRID = correlation.NormalizeRID(selection.Pending.RID)
			}
			return &matchedUpdate{UpdateID: update.UpdateID, Message: msg, RID: fallbackRID}, st.UpdateOffset != before
		}
	}

	return nil, st.UpdateOffset != before
}

func resolveRIDSelection(st *state.State, chatID int64, ridRaw string) (ridSelection, error) {
	rid := correlation.NormalizeRID(ridRaw)
	if rid != "" {
		if !correlation.IsValidRID(rid) {
			return ridSelection{}, errors.New("--rid must match [a-z0-9_-]")
		}
		pending := pendingForRID(st, chatID, rid)
		return ridSelection{RID: rid, Pending: pending}, nil
	}

	pending := currentPendingForChat(st, chatID)
	if pending == nil {
		return ridSelection{AllowAnyRID: true}, nil
	}
	rid = correlation.NormalizeRID(pending.RID)
	if !correlation.IsValidRID(rid) {
		return ridSelection{}, errors.New("pending request RID is invalid; send a new message")
	}
	return ridSelection{RID: rid, Pending: pending, AllowPlainFallback: true}, nil
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

func pendingForRID(st *state.State, chatID int64, rid string) *state.PendingRequest {
	pending := currentPendingForChat(st, chatID)
	if pending == nil {
		return nil
	}
	if correlation.NormalizeRID(pending.RID) != rid {
		return nil
	}
	return pending
}

func clearPendingForRID(st *state.State, rid string) bool {
	if st == nil || st.Pending == nil {
		return false
	}
	if correlation.NormalizeRID(st.Pending.RID) != correlation.NormalizeRID(rid) {
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

func messageMatchesRID(msg *telegram.Message, rid string) bool {
	if msg == nil {
		return false
	}
	if correlation.TextContainsRID(msg.Text, rid) {
		return true
	}
	if msg.ReplyToMessage != nil && correlation.TextContainsRID(msg.ReplyToMessage.Text, rid) {
		return true
	}
	return false
}

func extractRIDFromMessage(msg *telegram.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	if rid, ok := correlation.ExtractRID(msg.Text); ok {
		rid = correlation.NormalizeRID(rid)
		if correlation.IsValidRID(rid) {
			return rid, true
		}
	}
	if msg.ReplyToMessage != nil {
		if rid, ok := correlation.ExtractRID(msg.ReplyToMessage.Text); ok {
			rid = correlation.NormalizeRID(rid)
			if correlation.IsValidRID(rid) {
				return rid, true
			}
		}
	}
	return "", false
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

func composeOutboundMessage(messageText, rid, tag string) string {
	parts := []string{fmt.Sprintf("[rid:%s]", rid)}
	if strings.TrimSpace(tag) != "" {
		parts = append(parts, fmt.Sprintf("[tag:%s]", strings.TrimSpace(tag)))
	}
	parts = append(parts, strings.TrimSpace(messageText))
	return strings.Join(parts, " ")
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
