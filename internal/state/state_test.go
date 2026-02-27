package state

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	temp := t.TempDir()
	path := filepath.Join(temp, "state.json")

	in := State{
		UpdateOffset: 42,
		Pending: &PendingRequest{
			ChatID:        123,
			SentMessageID: 77,
			SentAtUnix:    1700000000,
		},
	}
	if err := Save(path, in); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if out.UpdateOffset != in.UpdateOffset {
		t.Fatalf("UpdateOffset = %d, want %d", out.UpdateOffset, in.UpdateOffset)
	}
	if out.Pending == nil {
		t.Fatal("Pending should not be nil")
	}
	if out.Pending.ChatID != in.Pending.ChatID {
		t.Fatalf("Pending.ChatID = %d, want %d", out.Pending.ChatID, in.Pending.ChatID)
	}
	if out.Pending.SentMessageID != in.Pending.SentMessageID {
		t.Fatalf("Pending.SentMessageID = %d, want %d", out.Pending.SentMessageID, in.Pending.SentMessageID)
	}
	if out.Pending.SentAtUnix != in.Pending.SentAtUnix {
		t.Fatalf("Pending.SentAtUnix = %d, want %d", out.Pending.SentAtUnix, in.Pending.SentAtUnix)
	}
}

func TestAdvanceOffset(t *testing.T) {
	st := State{UpdateOffset: 10}
	AdvanceOffset(&st, 5)
	if st.UpdateOffset != 10 {
		t.Fatalf("UpdateOffset regressed to %d", st.UpdateOffset)
	}
	AdvanceOffset(&st, 12)
	if st.UpdateOffset != 13 {
		t.Fatalf("UpdateOffset = %d, want 13", st.UpdateOffset)
	}
}

func TestSetAndClearPending(t *testing.T) {
	st := State{}
	SetPending(&st, PendingRequest{SentMessageID: 10})
	if st.Pending == nil {
		t.Fatal("Pending should not be nil")
	}
	ClearPending(&st)
	if st.Pending != nil {
		t.Fatal("Pending should be nil")
	}
}
