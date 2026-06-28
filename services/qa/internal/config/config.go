package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr               string
	MaxContextMessages int
	MaxContextChars    int
	ShutdownTimeout    time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Addr:               ":" + getEnv("PORT", "8080"),
		MaxContextMessages: 20,
		MaxContextChars:    8000,
		ShutdownTimeout:    10 * time.Second,
	}

	var err error
	if cfg.MaxContextMessages, err = getPositiveInt("MAX_CONTEXT_MESSAGES", cfg.MaxContextMessages); err != nil {
		return Config{}, err
	}
	if cfg.MaxContextChars, err = getPositiveInt("MAX_CONTEXT_CHARS", cfg.MaxContextChars); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getPositiveInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}

	return parsed, nil
}
