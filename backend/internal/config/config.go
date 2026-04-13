package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
// Required fields: WorkDir, JWTSecret, AdminUsername, AdminPasswordHash.
// On missing required field the server panics with a clear message.
type Config struct {
	Port                 string
	WorkerCount          int
	WorkDir              string
	ClaudeBin            string
	ClaudeDefaultModel   string
	ClaudePermissionMode string
	JobTimeoutMin        int
	MaxCostPerJobUSD     float64
	JobLogRetentionDays  int
	DBPath               string
	JWTSecret            string
	AdminUsername        string
	AdminPasswordHash    string
	CORSOrigin           string
}

// Load reads .env file (optional), environment variables, and returns a validated Config.
// Priority: env vars > .env file > hardcoded defaults.
func Load() (*Config, error) {
	// .env is optional — silently ignore if missing
	godotenv.Load()

	cfg := &Config{
		Port:                 getEnv("PORT", "8888"),
		WorkerCount:          getEnvInt("WORKER_COUNT", 2),
		WorkDir:              getEnv("WORK_DIR", ""),
		ClaudeBin:            getEnv("CLAUDE_BIN", "/usr/local/bin/claude"),
		ClaudeDefaultModel:   getEnv("CLAUDE_DEFAULT_MODEL", "claude-sonnet-4-6"),
		ClaudePermissionMode: getEnv("CLAUDE_PERMISSION_MODE", "bypassPermissions"),
		JobTimeoutMin:        getEnvInt("JOB_TIMEOUT_MIN", 30),
		MaxCostPerJobUSD:     getEnvFloat("MAX_COST_PER_JOB_USD", 1.0),
		JobLogRetentionDays:  getEnvInt("JOB_LOG_RETENTION_DAYS", 14),
		DBPath:               getEnv("DB_PATH", "./claudemote.db"),
		JWTSecret:            getEnv("JWT_SECRET", ""),
		AdminUsername:        getEnv("ADMIN_USERNAME", ""),
		AdminPasswordHash:    getEnv("ADMIN_PASSWORD_HASH", ""),
		CORSOrigin:           getEnv("CORS_ORIGIN", "http://localhost:8088"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validate checks that all required configuration fields are present.
func (c *Config) validate() error {
	required := []struct {
		name  string
		value string
	}{
		{"WORK_DIR", c.WorkDir},
		{"JWT_SECRET", c.JWTSecret},
		{"ADMIN_USERNAME", c.AdminUsername},
		{"ADMIN_PASSWORD_HASH", c.AdminPasswordHash},
	}

	for _, r := range required {
		if r.value == "" {
			return fmt.Errorf("required environment variable %s is not set", r.name)
		}
	}

	// Enforce a minimum secret length to prevent brute-forceable HS256 tokens.
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 bytes (got %d)", len(c.JWTSecret))
	}

	return nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return fallback
}
