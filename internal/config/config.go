package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration loaded from environment variables.
// One instance is created at startup and passed explicitly — no global state.
type Config struct {
	// Server
	Port string
	Env  string

	// Database
	DatabaseURL string

	// JWT (HS256, stateless — no DB storage for refresh tokens)
	JWTSecret string

	// AES-256 encryption for PII fields (phone, Aadhaar, PAN)
	// Must be a 64-character hex string (= 32 bytes)
	EncryptionKey string

	// Google Maps (geocoding on property creation)
	GoogleMapsAPIKey string

	// OpenAI (async personality embedding generation)
	OpenAIAPIKey string

	// Razorpay (wired in Module 8)
	RazorpayKeyID     string
	RazorpayKeySecret string

	// Email / SMTP (wired in Module 10)
	SMTPHost  string
	SMTPPort  int
	SMTPUser  string
	SMTPPass  string
	EmailFrom string
}

// Load reads environment variables and returns a populated Config.
// Returns an error if any required variable is missing or invalid.
func Load() (*Config, error) {
	smtpPort, err := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	if err != nil {
		return nil, fmt.Errorf("config: SMTP_PORT must be a number: %w", err)
	}

	cfg := &Config{
		Port:              getEnv("PORT", "8080"),
		Env:               getEnv("ENV", "development"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		EncryptionKey:     os.Getenv("ENCRYPTION_KEY"),
		GoogleMapsAPIKey:  os.Getenv("GOOGLE_MAPS_API_KEY"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		RazorpayKeyID:     os.Getenv("RAZORPAY_KEY_ID"),
		RazorpayKeySecret: os.Getenv("RAZORPAY_KEY_SECRET"),
		SMTPHost:          os.Getenv("SMTP_HOST"),
		SMTPPort:          smtpPort,
		SMTPUser:          os.Getenv("SMTP_USER"),
		SMTPPass:          os.Getenv("SMTP_PASS"),
		EmailFrom:         getEnv("EMAIL_FROM", "noreply@bachelorpad.in"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// IsProduction returns true when running in production mode.
func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

// validate checks that all required config values are present and valid.
func (c *Config) validate() error {
	required := map[string]string{
		"DATABASE_URL":   c.DatabaseURL,
		"JWT_SECRET":     c.JWTSecret,
		"ENCRYPTION_KEY": c.EncryptionKey,
	}
	for key, val := range required {
		if val == "" {
			return fmt.Errorf("config: required environment variable %q is not set", key)
		}
	}
	if len(c.EncryptionKey) != 64 {
		return fmt.Errorf("config: ENCRYPTION_KEY must be 64 hex characters (32 bytes), got %d", len(c.EncryptionKey))
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
