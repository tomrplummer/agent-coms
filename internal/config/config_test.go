package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultPathsUseXDG(t *testing.T) {
	temp := t.TempDir()
	cfgHome := filepath.Join(temp, "xdg-config")
	stateHome := filepath.Join(temp, "xdg-state")
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("XDG_STATE_HOME", stateHome)

	if got, want := DefaultConfigPath(), filepath.Join(cfgHome, "coms", "config.toml"); got != want {
		t.Fatalf("DefaultConfigPath() = %q, want %q", got, want)
	}
	if got, want := DefaultStatePath(), filepath.Join(stateHome, "coms", "state.json"); got != want {
		t.Fatalf("DefaultStatePath() = %q, want %q", got, want)
	}
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(temp, "state-home"))
	cfgPath := filepath.Join(temp, "missing.toml")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Telegram.PollIntervalSec != 2 {
		t.Fatalf("PollIntervalSec = %d, want 2", cfg.Telegram.PollIntervalSec)
	}
	if cfg.Telegram.DefaultTimeoutSec != 900 {
		t.Fatalf("DefaultTimeoutSec = %d, want 900", cfg.Telegram.DefaultTimeoutSec)
	}
	if cfg.State.OffsetStore == "" {
		t.Fatal("OffsetStore should not be empty")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	temp := t.TempDir()
	cfgPath := filepath.Join(temp, "config.toml")
	statePath := filepath.Join(temp, "state.json")

	in := Config{
		Telegram: TelegramConfig{
			ChatID:            123456789,
			Username:          "tom",
			PollIntervalSec:   5,
			DefaultTimeoutSec: 120,
		},
		State: StateConfig{OffsetStore: statePath},
	}

	if err := Save(cfgPath, in); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	out, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if out.Telegram.ChatID != in.Telegram.ChatID {
		t.Fatalf("ChatID = %d, want %d", out.Telegram.ChatID, in.Telegram.ChatID)
	}
	if out.Telegram.Username != in.Telegram.Username {
		t.Fatalf("Username = %q, want %q", out.Telegram.Username, in.Telegram.Username)
	}
	if out.Telegram.PollIntervalSec != in.Telegram.PollIntervalSec {
		t.Fatalf("PollIntervalSec = %d, want %d", out.Telegram.PollIntervalSec, in.Telegram.PollIntervalSec)
	}
	if out.Telegram.DefaultTimeoutSec != in.Telegram.DefaultTimeoutSec {
		t.Fatalf("DefaultTimeoutSec = %d, want %d", out.Telegram.DefaultTimeoutSec, in.Telegram.DefaultTimeoutSec)
	}
	if out.State.OffsetStore != statePath {
		t.Fatalf("OffsetStore = %q, want %q", out.State.OffsetStore, statePath)
	}
}
