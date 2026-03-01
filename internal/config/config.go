/*
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *  Configuration — Environment Variable Loader
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *
 *  Reads app settings from environment variables:
 *    TELEGRAM_BOT_TOKEN  – Telegram Bot API token (required)
 *    WAGRAM_DB_PATH      – SQLite file for chat mappings (default: wagram.db)
 *    WA_SESSION_DB_PATH  – SQLite file for WA sessions  (default: wa_session.db)
 */
package config

import "os"

/**
 * Config holds all application settings.
 */
type Config struct {
	TelegramBotToken string
	WagramDBPath     string
	WASessionDBPath  string
}

/**
 * Load reads configuration from environment variables
 * and applies sensible defaults where needed.
 */
func Load() Config {
	cfg := Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		WagramDBPath:     getEnvOrDefault("WAGRAM_DB_PATH", "wagram.db"),
		WASessionDBPath:  getEnvOrDefault("WA_SESSION_DB_PATH", "wa_session.db"),
	}
	return cfg
}

/**
 * getEnvOrDefault returns the environment variable value,
 * or the fallback if the variable is empty/unset.
 */
func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
