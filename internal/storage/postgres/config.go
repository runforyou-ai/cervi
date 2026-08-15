package postgres

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
	defaultStartupTimeout  = time.Minute
)

// Config contains the PostgreSQL connection and pool settings used by Bun.
type Config struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	StartupTimeout  time.Duration
}

// ConfigFromEnv loads server-side PostgreSQL settings from environment variables.
func ConfigFromEnv() (Config, error) {
	config := Config{
		DSN:             strings.TrimSpace(os.Getenv("DATABASE_URL")),
		MaxOpenConns:    defaultMaxOpenConns,
		MaxIdleConns:    defaultMaxIdleConns,
		ConnMaxLifetime: defaultConnMaxLifetime,
		ConnMaxIdleTime: defaultConnMaxIdleTime,
		StartupTimeout:  defaultStartupTimeout,
	}

	if config.DSN == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required for server builds")
	}

	var err error
	if config.MaxOpenConns, err = intFromEnv("POSTGRES_MAX_OPEN_CONNS", config.MaxOpenConns); err != nil {
		return Config{}, err
	}
	if config.MaxIdleConns, err = intFromEnv("POSTGRES_MAX_IDLE_CONNS", config.MaxIdleConns); err != nil {
		return Config{}, err
	}
	if config.ConnMaxLifetime, err = durationFromEnv("POSTGRES_CONN_MAX_LIFETIME", config.ConnMaxLifetime); err != nil {
		return Config{}, err
	}
	if config.ConnMaxIdleTime, err = durationFromEnv("POSTGRES_CONN_MAX_IDLE_TIME", config.ConnMaxIdleTime); err != nil {
		return Config{}, err
	}
	if config.StartupTimeout, err = durationFromEnv("POSTGRES_STARTUP_TIMEOUT", config.StartupTimeout); err != nil {
		return Config{}, err
	}
	if config.MaxIdleConns > config.MaxOpenConns {
		return Config{}, fmt.Errorf("POSTGRES_MAX_IDLE_CONNS must not exceed POSTGRES_MAX_OPEN_CONNS")
	}

	return config, nil
}

func intFromEnv(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration (for example 30s or 5m)", name)
	}
	return parsed, nil
}
