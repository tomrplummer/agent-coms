package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.telegram.org"

type Client struct {
	BotToken   string
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(botToken string) *Client {
	return &Client{
		BotToken: botToken,
		BaseURL:  defaultBaseURL,
		HTTPClient: &http.Client{
			Timeout: 70 * time.Second,
		},
	}
}

type APIError struct {
	Code        int
	Description string
}

func (e APIError) Error() string {
	if e.Description == "" {
		return fmt.Sprintf("telegram API error (%d)", e.Code)
	}
	return fmt.Sprintf("telegram API error (%d): %s", e.Code, e.Description)
}

type RetryAfterError struct {
	RetryAfterSec int
	Description   string
}

func (e RetryAfterError) Error() string {
	if e.Description == "" {
		return fmt.Sprintf("telegram rate limited; retry after %ds", e.RetryAfterSec)
	}
	return fmt.Sprintf("%s (retry after %ds)", e.Description, e.RetryAfterSec)
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) (Message, error) {
	var out Message
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	if err := c.call(ctx, "sendMessage", payload, &out); err != nil {
		return Message{}, err
	}
	return out, nil
}

func (c *Client) GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error) {
	payload := map[string]any{}
	if offset > 0 {
		payload["offset"] = offset
	}
	if timeoutSec > 0 {
		payload["timeout"] = timeoutSec
	}
	payload["allowed_updates"] = []string{"message", "edited_message"}

	var updates []Update
	if err := c.call(ctx, "getUpdates", payload, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func (c *Client) call(ctx context.Context, method string, payload any, out any) error {
	if strings.TrimSpace(c.BotToken) == "" {
		return errors.New("bot token is empty")
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		c.BaseURL = defaultBaseURL
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 70 * time.Second}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	endpoint := fmt.Sprintf("%s/bot%s/%s", strings.TrimRight(c.BaseURL, "/"), c.BotToken, method)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(b, &envelope); err != nil {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return fmt.Errorf("decode response: %w", err)
		}
		return APIError{Code: resp.StatusCode, Description: string(b)}
	}

	if !envelope.OK {
		if envelope.ErrorCode == http.StatusTooManyRequests && envelope.Parameters.RetryAfter > 0 {
			return RetryAfterError{RetryAfterSec: envelope.Parameters.RetryAfter, Description: envelope.Description}
		}
		code := envelope.ErrorCode
		if code == 0 {
			code = resp.StatusCode
		}
		return APIError{Code: code, Description: envelope.Description}
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, out); err != nil {
		return fmt.Errorf("decode result: %w", err)
	}
	return nil
}

type apiEnvelope struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

type Update struct {
	UpdateID      int64    `json:"update_id"`
	Message       *Message `json:"message,omitempty"`
	EditedMessage *Message `json:"edited_message,omitempty"`
}

type Message struct {
	MessageID      int64    `json:"message_id"`
	Date           int64    `json:"date"`
	Text           string   `json:"text"`
	From           *User    `json:"from,omitempty"`
	Chat           Chat     `json:"chat"`
	ReplyToMessage *Message `json:"reply_to_message,omitempty"`
}

type Chat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Username string `json:"username,omitempty"`
}

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}
