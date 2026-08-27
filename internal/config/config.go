package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL    string
	MaxConns       int32
	MinConns       int32
	HTTPAddress    string
	AppEnv         string
	FrontendURL    string
	JWTSecret      string
	JWTExpiration  time.Duration
	GoogleClientID string
	SMTPHost       string
	SMTPPort       string
	SMTPUsername   string
	SMTPPassword   string
	SMTPFrom       string
}

func Load() (Config, error) {
	maxConns, err := envInt32("DB_MAX_CONNS", 20)
	if err != nil {
		return Config{}, err
	}
	minConns, err := envInt32("DB_MIN_CONNS", 2)
	if err != nil {
		return Config{}, err
	}
	if minConns > maxConns {
		return Config{}, fmt.Errorf("DB_MIN_CONNS must not exceed DB_MAX_CONNS")
	}
	jwtExpiration, err := time.ParseDuration(env("JWT_EXPIRATION", "24h"))
	if err != nil {
		return Config{}, fmt.Errorf("JWT_EXPIRATION must be a valid duration")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientID == "" {
		return Config{}, fmt.Errorf("GOOGLE_CLIENT_ID is required")
	}
	databaseURL, err := LoadDatabaseURL()
	if err != nil {
		return Config{}, err
	}
	port := env("APP_PORT", "8080")
	httpAddress := os.Getenv("HTTP_ADDRESS")
	if httpAddress == "" {
		httpAddress = ":" + port
	}
	return Config{
		DatabaseURL: databaseURL, MaxConns: maxConns, MinConns: minConns,
		HTTPAddress: httpAddress, AppEnv: env("APP_ENV", "development"), FrontendURL: env("FRONTEND_URL", "http://localhost:3000"), JWTSecret: jwtSecret, JWTExpiration: jwtExpiration, GoogleClientID: googleClientID,
		SMTPHost: os.Getenv("SMTP_HOST"), SMTPPort: env("SMTP_PORT", "1025"), SMTPUsername: os.Getenv("SMTP_USERNAME"), SMTPPassword: os.Getenv("SMTP_PASSWORD"), SMTPFrom: env("SMTP_FROM", "social-fund@example.test"),
	}, nil
}

func LoadDatabaseURL() (string, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		return databaseURL, nil
	}
	return legacyDatabaseURL()
}

func legacyDatabaseURL() (string, error) {
	required := []string{"DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD"}
	for _, key := range required {
		if os.Getenv(key) == "" {
			return "", fmt.Errorf("DATABASE_URL is required")
		}
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME"), env("DB_SSLMODE", "disable")), nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func envInt32(key string, fallback int32) (int32, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
	return int32(parsed), nil
}

const DatabasePingTimeout = 5 * time.Second
