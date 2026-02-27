package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type State struct {
	UpdateOffset int64           `json:"update_offset"`
	Pending      *PendingRequest `json:"pending,omitempty"`
}

type PendingRequest struct {
	ChatID        int64 `json:"chat_id,omitempty"`
	SentMessageID int64 `json:"sent_message_id,omitempty"`
	SentAtUnix    int64 `json:"sent_at_unix,omitempty"`
}

func Load(path string) (State, error) {
	var st State
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return st, nil
		}
		return st, fmt.Errorf("read state: %w", err)
	}
	if len(b) == 0 {
		return st, nil
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, fmt.Errorf("parse state: %w", err)
	}
	return st, nil
}

func Save(path string, st State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	return nil
}

func AdvanceOffset(st *State, updateID int64) {
	next := updateID + 1
	if next > st.UpdateOffset {
		st.UpdateOffset = next
	}
}

func SetPending(st *State, pending PendingRequest) {
	copy := pending
	st.Pending = &copy
}

func ClearPending(st *State) {
	st.Pending = nil
}
