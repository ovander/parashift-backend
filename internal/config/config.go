package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration from environment variables.
type Config struct {
	Env                 string
	Port                int
	LogLevel            string
	DatabaseURL         string
	AllowedOrigins      []string
	AppBaseURL          string
	AutoMigrate         bool
	RunMigrations       bool // RUN_MIGRATIONS: run file-based SQL migrations on startup (independent of AutoMigrate)
	MaxRequestBodyBytes int64
	MetricsEnabled      bool
	Socrate             SocrateConfig
	DBPool              DBPoolConfig
	JWKS                JWKSConfig
	AI                  AIConfig
	Sentry              SentryConfig
	Tracing             TracingConfig
}

// TracingConfig holds OpenTelemetry tracing configuration (OBS-3).
// Leave Endpoint empty (or unset) to disable tracing entirely.
type TracingConfig struct {
	Endpoint    string  // OTLP/HTTP collector address, host:port (no scheme)
	Insecure    bool    // send over plaintext (dev / in-cluster without TLS)
	SampleRatio float64 // head sampling ratio 0..1 (1 = sample everything)
}

// AIConfig holds configuration for the AI gateway (Phase 3).
type AIConfig struct {
	AnthropicAPIKey string
	Model           string
	MaxTokens       int
	TimeoutSec      int
}

// SocrateConfig holds Socrate authentication configuration.
type SocrateConfig struct {
	BaseURL      string
	AdminURL     string
	ClientID     string
	ClientSecret string
	AppID        string
	RedirectURL  string
}

// DBPoolConfig holds database connection pool configuration.
type DBPoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int
}

// JWKSConfig holds JWT key set configuration.
type JWKSConfig struct {
	URL    string
	Issuer string
}

// SentryConfig holds Sentry error monitoring configuration.
// Leave DSN empty (or unset) to disable monitoring silently.
type SentryConfig struct {
	DSN string
}

// Load creates a Config from environment variables.
func Load() *Config {
	return &Config{
		Env:                 getEnv("ENV", "development"),
		Port:                getEnvInt("PORT", 4000),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
		DatabaseURL:         getEnv("DATABASE_URL", ""),
		AllowedOrigins:      parseCSV(getEnv("ALLOWED_ORIGINS", "http://localhost:3000")),
		AppBaseURL:          getEnv("APP_BASE_URL", "http://localhost:4000"),
		AutoMigrate:         getEnvBool("AUTO_MIGRATE", false),    // safe default: GORM schema sync; keep false in production
		RunMigrations:       getEnvBool("RUN_MIGRATIONS", false),  // safe default: explicit opt-in per deploy
		MaxRequestBodyBytes: int64(getEnvInt("MAX_REQUEST_BODY_BYTES", 10485760)), // 10MB
		MetricsEnabled:      getEnvBool("METRICS_ENABLED", false),
		Socrate: SocrateConfig{
			BaseURL:      getEnv("SOCRATE_BASE_URL", ""),
			AdminURL:     getEnv("SOCRATE_ADMIN_URL", ""),
			ClientID:     getEnv("SOCRATE_CLIENT_ID", ""),
			ClientSecret: getEnv("SOCRATE_CLIENT_SECRET", ""),
			AppID:        getEnv("SOCRATE_APP_ID", ""),
			RedirectURL:  getEnv("SOCRATE_REDIRECT_URL", ""),
		},
		DBPool: DBPoolConfig{
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvInt("DB_CONN_MAX_LIFETIME", 300),
		},
		JWKS: JWKSConfig{
			URL:    getEnv("SOCRATE_JWKS_URL", ""),
			Issuer: getEnv("SOCRATE_ISSUER", ""),
		},
		AI: AIConfig{
			AnthropicAPIKey: getEnv("ANTHROPIC_API_KEY", ""),
			Model:           getEnv("AI_MODEL", "claude-sonnet-4-6"),
			MaxTokens:       getEnvInt("AI_MAX_TOKENS", 2000),
			TimeoutSec:      getEnvInt("AI_TIMEOUT_SEC", 30),
		},
		Sentry: SentryConfig{
			DSN: getEnv("SENTRY_DSN", ""),
		},
		Tracing: TracingConfig{
			Endpoint:    getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			Insecure:    getEnvBool("OTEL_EXPORTER_OTLP_INSECURE", false),
			SampleRatio: getEnvFloat("OTEL_TRACES_SAMPLER_ARG", 1.0),
		},
	}
}

// Validate checks that all required configuration values are set.
// The server refuses to start if this returns an error.
func (c *Config) Validate() error {
	var errs []string

	required := []struct {
		name  string
		value string
	}{
		{"DATABASE_URL", c.DatabaseURL},
		{"SOCRATE_JWKS_URL / SOCRATE_JWKS_URL", c.JWKS.URL},
		{"SOCRATE_CLIENT_ID", c.Socrate.ClientID},
		{"SOCRATE_CLIENT_SECRET", c.Socrate.ClientSecret},
		{"SOCRATE_BASE_URL", c.Socrate.BaseURL},
	}
	for _, r := range required {
		if strings.TrimSpace(r.value) == "" {
			errs = append(errs, r.name+" is required")
		}
	}

	// Warn loudly when running production without issuer validation.
	if c.Env == "production" && strings.TrimSpace(c.JWKS.Issuer) == "" {
		errs = append(errs, "SOCRATE_ISSUER must be set in production (prevents accepting tokens from foreign issuers)")
	}

	// AutoMigrate in production is dangerous: it can silently alter constraints.
	if c.Env == "production" && c.AutoMigrate {
		errs = append(errs, "AUTO_MIGRATE must be false in production (use versioned SQL migrations)")
	}

	// Database must use TLS in production. sslmode=disable transmits credentials
	// and query results in plaintext; even on a private network this is unacceptable.
	if c.Env == "production" && strings.Contains(c.DatabaseURL, "sslmode=disable") {
		errs = append(errs, "DATABASE_URL must not use sslmode=disable in production (use sslmode=require or sslmode=verify-full)")
	}

	// CORS wildcard or localhost origins in production expose the API to cross-origin
	// requests from any website. Require at least one HTTPS origin.
	if c.Env == "production" {
		hasHTTPSOrigin := false
		for _, o := range c.AllowedOrigins {
			if strings.HasPrefix(o, "https://") {
				hasHTTPSOrigin = true
				break
			}
		}
		if !hasHTTPSOrigin {
			errs = append(errs, "ALLOWED_ORIGINS must contain at least one https:// origin in production")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// getEnv retrieves an environment variable or returns a fallback value.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// getEnvInt retrieves an environment variable as an integer or returns a fallback value.
func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return fallback
}

// getEnvBool retrieves an environment variable as a boolean or returns a fallback value.
func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		switch strings.ToLower(value) {
		case "true", "yes", "1":
			return true
		case "false", "no", "0":
			return false
		}
	}
	return fallback
}

// getEnvFloat retrieves an environment variable as a float64 or returns a fallback value.
func getEnvFloat(key string, fallback float64) float64 {
	if value, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return fallback
}

// parseCSV parses a comma-separated string into a slice of strings.
func parseCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	var result []string
	for _, item := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
