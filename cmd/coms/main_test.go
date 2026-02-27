package main

import (
	"testing"

	"github.com/tomrplummer/agent-coms/internal/state"
	"github.com/tomrplummer/agent-coms/internal/telegram"
)

func TestResolveSelectionUsesPendingWhenAvailable(t *testing.T) {
	st := state.State{
		Pending: &state.PendingRequest{ChatID: 123, SentMessageID: 10},
	}

	selection := resolveSelection(&st, 123)
	if selection.Pending == nil {
		t.Fatal("pending should not be nil")
	}
	if selection.AllowAnyMessage {
		t.Fatal("allowAnyMessage should be false when pending exists")
	}
}

func TestResolveSelectionAllowsAnyWithoutPending(t *testing.T) {
	st := state.State{}

	selection := resolveSelection(&st, 123)
	if selection.Pending != nil {
		t.Fatal("pending should be nil")
	}
	if !selection.AllowAnyMessage {
		t.Fatal("allowAnyMessage should be true without pending")
	}
}

func TestResolveSelectionIgnoresPendingFromAnotherChat(t *testing.T) {
	st := state.State{
		Pending: &state.PendingRequest{ChatID: 999, SentMessageID: 10},
	}

	selection := resolveSelection(&st, 123)
	if selection.Pending != nil {
		t.Fatal("pending should be nil for a different chat")
	}
	if !selection.AllowAnyMessage {
		t.Fatal("allowAnyMessage should be true when pending is for another chat")
	}
}

func TestApplyUpdatesMatchesPendingFallback(t *testing.T) {
	st := state.State{UpdateOffset: 0}
	pending := &state.PendingRequest{
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

	selection := matchSelection{Pending: pending}
	matched, changed := applyUpdates(&st, updates, 123, selection, 0)
	if matched == nil {
		t.Fatal("expected matched update")
	}
	if !changed {
		t.Fatal("expected state change from offset advance")
	}
}

func TestApplyUpdatesIgnoresReplyToDifferentMessage(t *testing.T) {
	st := state.State{UpdateOffset: 0}
	pending := &state.PendingRequest{
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

	selection := matchSelection{Pending: pending}
	matched, _ := applyUpdates(&st, updates, 123, selection, 0)
	if matched != nil {
		t.Fatal("did not expect match")
	}
}

func TestApplyUpdatesAllowsAnyMessageWithoutPending(t *testing.T) {
	st := state.State{UpdateOffset: 0}
	selection := matchSelection{AllowAnyMessage: true}

	updates := []telegram.Update{{
		UpdateID: 9,
		Message: &telegram.Message{
			MessageID: 13,
			Date:      1700000002,
			Text:      "Yep",
			Chat:      telegram.Chat{ID: 123, Type: "private"},
		},
	}}

	matched, _ := applyUpdates(&st, updates, 123, selection, 0)
	if matched == nil {
		t.Fatal("expected matched update")
	}
}

func TestClearPendingForSelection(t *testing.T) {
	st := state.State{Pending: &state.PendingRequest{ChatID: 123, SentMessageID: 10}}
	selection := matchSelection{Pending: &state.PendingRequest{ChatID: 123, SentMessageID: 10}}

	if !clearPendingForSelection(&st, selection) {
		t.Fatal("expected clearPendingForSelection to return true")
	}
	if st.Pending != nil {
		t.Fatal("pending should be cleared")
	}
}

func TestClearPendingForSelectionSkipsDifferentMessage(t *testing.T) {
	st := state.State{Pending: &state.PendingRequest{ChatID: 123, SentMessageID: 11}}
	selection := matchSelection{Pending: &state.PendingRequest{ChatID: 123, SentMessageID: 10}}

	if clearPendingForSelection(&st, selection) {
		t.Fatal("did not expect pending to be cleared")
	}
	if st.Pending == nil {
		t.Fatal("pending should remain")
	}
}
