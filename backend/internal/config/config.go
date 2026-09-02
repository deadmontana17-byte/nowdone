// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the api, bot and worker binaries.
type Config struct {
	Env             string
	DBDSN           string
	JWTSecret       string
	TelegramToken   string
	BotUsername     string
	SiteURL         string
	OpenAIAPIKey    string
	S3AccessKeyID   string
	S3SecretKey     string
	S3BucketName    string
	S3Endpoint      string
	S3Region        string
	S3PublicBaseURL string
	// S3PresignExpiry is how long a presigned upload URL stays valid. Kept
	// short (default 15m) so a leaked URL is only briefly useful.
	S3PresignExpiry time.Duration
	CookieDomain    string
	APIPort         string
	MigrationsPath  string
	JWTTokenTTL     time.Duration
	AuthCodeTTL     time.Duration
	ResetCodeTTL    time.Duration
}

// Load reads configuration from environment variables. All secrets must come
// from the environment, never hardcoded, per project rules.
func Load() (*Config, error) {
	cfg := &Config{
		Env:           getEnv("ENV", "production"),
		DBDSN:         os.Getenv("DB_DSN"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		BotUsername:   os.Getenv("BOT_USERNAME"),
		SiteURL:       os.Getenv("SITE_URL"),
		OpenAIAPIKey:  os.Getenv("OPENAI_API_KEY"),
		S3AccessKeyID: os.Getenv("S3_ACCESS_KEY_ID"),
		S3SecretKey:   os.Getenv("S3_SECRET_ACCESS_KEY"),
		S3BucketName:  os.Getenv("S3_BUCKET_NAME"),
		S3Endpoint:    os.Getenv("S3_ENDPOINT"),
		S3Region:      getEnv("S3_REGION", "us-east-1"),
		// Optional. Base URL for building public attachment links (e.g. a
		// website/virtual-hosted endpoint). When empty, links fall back to
		// "{S3_ENDPOINT}/{S3_BUCKET_NAME}".
		S3PublicBaseURL: os.Getenv("S3_PUBLIC_BASE_URL"),
		// Minutes; defaults to 15 when unset or not a positive integer.
		S3PresignExpiry: getEnvMinutes("S3_PRESIGN_EXPIRY", 15),
		CookieDomain:    getEnv("COOKIE_DOMAIN", ""),
		APIPort:         getEnv("API_PORT", "8080"),
		MigrationsPath:  getEnv("MIGRATIONS_PATH", "migrations"),
		JWTTokenTTL:     30 * 24 * time.Hour,
		AuthCodeTTL:     5 * time.Minute,
		ResetCodeTTL:    5 * time.Minute,
	}

	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvMinutes reads an integer count of minutes from the environment and
// returns it as a Duration. Falls back to fallbackMinutes when the variable is
// missing or not a positive integer.
func getEnvMinutes(key string, fallbackMinutes int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Minute
		}
	}
	return time.Duration(fallbackMinutes) * time.Minute
}
