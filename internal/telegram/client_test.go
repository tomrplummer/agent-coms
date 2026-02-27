package telegram

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func makeResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestSendMessage(t *testing.T) {
	client := NewClient("TOKEN")
	client.BaseURL = "https://example.test"
	client.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/botTOKEN/sendMessage" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			return makeResponse(http.StatusOK, `{
				"ok": true,
				"result": {
					"message_id": 99,
					"date": 1700000000,
					"text": "hello",
					"chat": {"id": 123, "type": "private"}
				}
			}`), nil
		}),
	}

	msg, err := client.SendMessage(context.Background(), 123, "hello")
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if msg.MessageID != 99 {
		t.Fatalf("MessageID = %d, want 99", msg.MessageID)
	}
}

func TestGetUpdates(t *testing.T) {
	client := NewClient("TOKEN")
	client.BaseURL = "https://example.test"
	client.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/botTOKEN/getUpdates" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			return makeResponse(http.StatusOK, `{
				"ok": true,
				"result": [{
					"update_id": 7,
					"message": {
						"message_id": 1,
						"date": 1700000001,
						"text": "[rid:abc] reply",
						"chat": {"id": 123, "type": "private"}
					}
				}]
			}`), nil
		}),
	}

	updates, err := client.GetUpdates(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("GetUpdates() error = %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("len(updates) = %d, want 1", len(updates))
	}
	if updates[0].UpdateID != 7 {
		t.Fatalf("UpdateID = %d, want 7", updates[0].UpdateID)
	}
}

func TestRetryAfterError(t *testing.T) {
	client := NewClient("TOKEN")
	client.BaseURL = "https://example.test"
	client.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return makeResponse(http.StatusTooManyRequests, `{
				"ok": false,
				"error_code": 429,
				"description": "Too Many Requests",
				"parameters": {"retry_after": 5}
			}`), nil
		}),
	}

	_, err := client.GetUpdates(context.Background(), 0, 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(RetryAfterError); !ok {
		t.Fatalf("expected RetryAfterError, got %T", err)
	}
}
