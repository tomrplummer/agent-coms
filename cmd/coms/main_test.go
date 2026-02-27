package main

import (
	"testing"

	"github.com/tomrplummer/agent-coms/internal/state"
	"github.com/tomrplummer/agent-coms/internal/telegram"
)

func TestResolveRIDSelectionUsesPendingWhenRIDMissing(t *testing.T) {
	st := state.State{
		Pending: &state.PendingRequest{RID: "Deploy42", ChatID: 123},
	}

	selection, err := resolveRIDSelection(&st, 123, "")
	if err != nil {
		t.Fatalf("resolveRIDSelection() error = %v", err)
	}
	if selection.RID != "deploy42" {
		t.Fatalf("selection.RID = %q, want %q", selection.RID, "deploy42")
	}
	if selection.Pending == nil {
		t.Fatal("pending should not be nil")
	}
	if !selection.AllowPlainFallback {
		t.Fatal("allowPlainFallback should be true")
	}
	if selection.AllowAnyRID {
		t.Fatal("allowAnyRID should be false")
	}
}

func TestResolveRIDSelectionAutoRIDWithoutPending(t *testing.T) {
	st := state.State{}
	selection, err := resolveRIDSelection(&st, 123, "")
	if err != nil {
		t.Fatalf("resolveRIDSelection() error = %v", err)
	}
	if !selection.AllowAnyRID {
		t.Fatal("allowAnyRID should be true")
	}
	if selection.RID != "" {
		t.Fatalf("selection.RID = %q, want empty", selection.RID)
	}
}

func TestApplyUpdatesMatchesPendingFallback(t *testing.T) {
	st := state.State{UpdateOffset: 0}
	pending := &state.PendingRequest{
		RID:           "abc123",
		ChatID:        123,
		SentMessageID: 10,
		SentAtUnix:    1700000000,
	}

	updates := []telegram.Update{{
		UpdateID: 7,
		Message: &telegram.Message{
			MessageID: 11,
			Date:      1700000000,
			Text:      "Looks good",
			Chat:      telegram.Chat{ID: 123, Type: "private"},
		},
	}}

	selection := ridSelection{RID: "abc123", Pending: pending, AllowPlainFallback: true}
	matched, changed := applyUpdates(&st, updates, 123, selection, 0)
	if matched == nil {
		t.Fatal("expected matched update")
	}
	if matched.RID != "abc123" {
		t.Fatalf("matched.RID = %q, want %q", matched.RID, "abc123")
	}
	if !changed {
		t.Fatal("expected state change from offset advance")
	}
}

func TestApplyUpdatesIgnoresReplyToDifferentMessage(t *testing.T) {
	st := state.State{UpdateOffset: 0}
	pending := &state.PendingRequest{
		RID:           "abc123",
		ChatID:        123,
		SentMessageID: 10,
		SentAtUnix:    1700000000,
	}

	updates := []telegram.Update{{
		UpdateID: 8,
		Message: &telegram.Message{
			MessageID: 12,
			Date:      1700000001,
			Text:      "Unrelated",
			Chat:      telegram.Chat{ID: 123, Type: "private"},
			ReplyToMessage: &telegram.Message{
				MessageID: 9,
				Text:      "old",
			},
		},
	}}

	selection := ridSelection{RID: "abc123", Pending: pending, AllowPlainFallback: true}
	matched, _ := applyUpdates(&st, updates, 123, selection, 0)
	if matched != nil {
		t.Fatal("did not expect match")
	}
}

func TestApplyUpdatesAutoRIDFromReply(t *testing.T) {
	st := state.State{UpdateOffset: 0}
	selection := ridSelection{AllowAnyRID: true}

	updates := []telegram.Update{{
		UpdateID: 9,
		Message: &telegram.Message{
			MessageID: 13,
			Date:      1700000002,
			Text:      "Yep",
			Chat:      telegram.Chat{ID: 123, Type: "private"},
			ReplyToMessage: &telegram.Message{
				MessageID: 10,
				Text:      "[rid:deploy42] Need a call",
			},
		},
	}}

	matched, _ := applyUpdates(&st, updates, 123, selection, 0)
	if matched == nil {
		t.Fatal("expected matched update")
	}
	if matched.RID != "deploy42" {
		t.Fatalf("matched.RID = %q, want %q", matched.RID, "deploy42")
	}
}

func TestClearPendingForRID(t *testing.T) {
	st := state.State{Pending: &state.PendingRequest{RID: "abc123"}}
	if !clearPendingForRID(&st, "ABC123") {
		t.Fatal("expected clearPendingForRID to return true")
	}
	if st.Pending != nil {
		t.Fatal("pending should be cleared")
	}
}
