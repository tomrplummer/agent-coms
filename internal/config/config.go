package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultPollIntervalSec   = 2
	defaultTimeoutSec        = 900
	defaultConfigRelativeDir = "coms/config.toml"
	defaultStateRelativeDir  = "coms/state.json"
)

type Config struct {
	Telegram TelegramConfig
	State    StateConfig
}

type TelegramConfig struct {
	ChatID            int64
	Username          string
	PollIntervalSec   int
	DefaultTimeoutSec int
}

type StateConfig struct {
	OffsetStore string
}

func DefaultConfig() Config {
	return Config{
		Telegram: TelegramConfig{
			PollIntervalSec:   defaultPollIntervalSec,
			DefaultTimeoutSec: defaultTimeoutSec,
		},
		State: StateConfig{
			OffsetStore: DefaultStatePath(),
		},
	}
}

func DefaultConfigPath() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, defaultConfigRelativeDir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".config/" + defaultConfigRelativeDir
	}
	return filepath.Join(home, ".config", defaultConfigRelativeDir)
}

func DefaultStatePath() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); xdg != "" {
		return filepath.Join(xdg, defaultStateRelativeDir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".local/state/" + defaultStateRelativeDir
	}
	return filepath.Join(home, ".local", "state", defaultStateRelativeDir)
}

func ResolvePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path is empty")
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve abs path: %w", err)
	}
	return filepath.Clean(abs), nil
}

func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	resolvedPath, err := ResolvePath(path)
	if err != nil {
		return cfg, err
	}

	b, err := os.ReadFile(resolvedPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			cfg.State.OffsetStore = DefaultStatePath()
			if resolved, resolveErr := ResolvePath(cfg.State.OffsetStore); resolveErr == nil {
				cfg.State.OffsetStore = resolved
			}
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := parseTOML(b, &cfg); err != nil {
		return cfg, err
	}

	cfg.applyDefaults()
	cfg.State.OffsetStore, err = ResolvePath(cfg.State.OffsetStore)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	resolvedPath, err := ResolvePath(path)
	if err != nil {
		return err
	}
	cfg.applyDefaults()
	cfg.State.OffsetStore, err = ResolvePath(cfg.State.OffsetStore)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	content := renderTOML(cfg)
	tmp := resolvedPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := os.Rename(tmp, resolvedPath); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func (c *Config) applyDefaults() {
	if c.Telegram.PollIntervalSec <= 0 {
		c.Telegram.PollIntervalSec = defaultPollIntervalSec
	}
	if c.Telegram.DefaultTimeoutSec <= 0 {
		c.Telegram.DefaultTimeoutSec = defaultTimeoutSec
	}
	if strings.TrimSpace(c.State.OffsetStore) == "" {
		c.State.OffsetStore = DefaultStatePath()
	}
}

func parseTOML(b []byte, cfg *Config) error {
	scanner := bufio.NewScanner(strings.NewReader(string(b)))
	section := ""
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(stripComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("config parse error on line %d", lineNo)
		}
		key := strings.TrimSpace(parts[0])
		raw := strings.TrimSpace(parts[1])

		if err := assignValue(cfg, section, key, raw); err != nil {
			return fmt.Errorf("config parse error on line %d: %w", lineNo, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan config: %w", err)
	}
	return nil
}

func stripComment(line string) string {
	inQuote := false
	for i := 0; i < len(line); i++ {
		if line[i] == '"' {
			if i == 0 || line[i-1] != '\\' {
				inQuote = !inQuote
			}
			continue
		}
		if line[i] == '#' && !inQuote {
			return line[:i]
		}
	}
	return line
}

func assignValue(cfg *Config, section, key, raw string) error {
	path := key
	if section != "" {
		path = section + "." + key
	}

	switch path {
	case "telegram.chat_id":
		v, err := parseInt64(raw)
		if err != nil {
			return err
		}
		cfg.Telegram.ChatID = v
	case "telegram.username":
		v, err := parseString(raw)
		if err != nil {
			return err
		}
		cfg.Telegram.Username = v
	case "telegram.poll_interval_sec":
		v, err := parseInt(raw)
		if err != nil {
			return err
		}
		cfg.Telegram.PollIntervalSec = v
	case "telegram.default_timeout_sec":
		v, err := parseInt(raw)
		if err != nil {
			return err
		}
		cfg.Telegram.DefaultTimeoutSec = v
	case "state.offset_store":
		v, err := parseString(raw)
		if err != nil {
			return err
		}
		cfg.State.OffsetStore = v
	default:
		// Ignore unknown keys so config can evolve without breaking older clients.
	}
	return nil
}

func parseInt(raw string) (int, error) {
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("expected int, got %q", raw)
	}
	return v, nil
}

func parseInt64(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "\"") {
		unquoted, err := parseString(raw)
		if err != nil {
			return 0, err
		}
		raw = unquoted
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("expected int64, got %q", raw)
	}
	return v, nil
}

func parseString(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "\"") || !strings.HasSuffix(raw, "\"") {
		return "", fmt.Errorf("expected quoted string, got %q", raw)
	}
	v, err := strconv.Unquote(raw)
	if err != nil {
		return "", fmt.Errorf("parse string: %w", err)
	}
	return v, nil
}

func renderTOML(cfg Config) string {
	cfg.applyDefaults()
	var b strings.Builder
	b.WriteString("[telegram]\n")
	if cfg.Telegram.ChatID != 0 {
		b.WriteString(fmt.Sprintf("chat_id = %d\n", cfg.Telegram.ChatID))
	}
	if cfg.Telegram.Username != "" {
		b.WriteString(fmt.Sprintf("username = %q\n", cfg.Telegram.Username))
	}
	b.WriteString(fmt.Sprintf("poll_interval_sec = %d\n", cfg.Telegram.PollIntervalSec))
	b.WriteString(fmt.Sprintf("default_timeout_sec = %d\n", cfg.Telegram.DefaultTimeoutSec))
	b.WriteString("\n")
	b.WriteString("[state]\n")
	b.WriteString(fmt.Sprintf("offset_store = %q\n", cfg.State.OffsetStore))
	return b.String()
}
