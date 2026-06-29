package config

import (
	"fmt"
	"os"
	"time"
)

const (
	DefaultHTTPAddr        = ":8083"
	DefaultServiceVersion  = "dev"
	DefaultEnvironment     = "local"
	DefaultShutdownTimeout = 10 * time.Second
)

type Config struct {
	HTTPAddr        string
	ServiceVersion  string
	Environment     string
	DatabaseURL     string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:        stringValue("KNOWLEDGE_HTTP_ADDR", DefaultHTTPAddr),
		ServiceVersion:  stringValue("KNOWLEDGE_SERVICE_VERSION", DefaultServiceVersion),
		Environment:     stringValue("KNOWLEDGE_ENV", DefaultEnvironment),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		ShutdownTimeout: DefaultShutdownTimeout,
	}

	if raw := os.Getenv("KNOWLEDGE_SHUTDOWN_TIMEOUT"); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("KNOWLEDGE_SHUTDOWN_TIMEOUT must be a positive duration")
		}
		cfg.ShutdownTimeout = value
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func stringValue(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
