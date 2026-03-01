package config_test

import (
	"os"
	"testing"

	"telegram-wa/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	os.Unsetenv("TELEGRAM_BOT_TOKEN")
	os.Unsetenv("WAGRAM_DB_PATH")
	os.Unsetenv("WA_SESSION_DB_PATH")

	cfg := config.Load()

	if cfg.TelegramBotToken != "" {
		t.Errorf("expected empty token, got %q", cfg.TelegramBotToken)
	}
	if cfg.WagramDBPath != "wagram.db" {
		t.Errorf("expected default wagram.db, got %q", cfg.WagramDBPath)
	}
	if cfg.WASessionDBPath != "wa_session.db" {
		t.Errorf("expected default wa_session.db, got %q", cfg.WASessionDBPath)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-token-123")
	os.Setenv("WAGRAM_DB_PATH", "/tmp/test_wagram.db")
	os.Setenv("WA_SESSION_DB_PATH", "/tmp/test_wa.db")
	defer func() {
		os.Unsetenv("TELEGRAM_BOT_TOKEN")
		os.Unsetenv("WAGRAM_DB_PATH")
		os.Unsetenv("WA_SESSION_DB_PATH")
	}()

	cfg := config.Load()

	if cfg.TelegramBotToken != "test-token-123" {
		t.Errorf("expected test-token-123, got %q", cfg.TelegramBotToken)
	}
	if cfg.WagramDBPath != "/tmp/test_wagram.db" {
		t.Errorf("expected /tmp/test_wagram.db, got %q", cfg.WagramDBPath)
	}
	if cfg.WASessionDBPath != "/tmp/test_wa.db" {
		t.Errorf("expected /tmp/test_wa.db, got %q", cfg.WASessionDBPath)
	}
}

func TestLoad_PartialEnv(t *testing.T) {
	os.Setenv("TELEGRAM_BOT_TOKEN", "my-token")
	os.Unsetenv("WAGRAM_DB_PATH")
	os.Unsetenv("WA_SESSION_DB_PATH")
	defer os.Unsetenv("TELEGRAM_BOT_TOKEN")

	cfg := config.Load()

	if cfg.TelegramBotToken != "my-token" {
		t.Errorf("expected my-token, got %q", cfg.TelegramBotToken)
	}
	if cfg.WagramDBPath != "wagram.db" {
		t.Errorf("expected default wagram.db, got %q", cfg.WagramDBPath)
	}
	if cfg.WASessionDBPath != "wa_session.db" {
		t.Errorf("expected default wa_session.db, got %q", cfg.WASessionDBPath)
	}
}
