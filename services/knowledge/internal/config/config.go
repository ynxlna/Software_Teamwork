package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultHTTPAddr           = ":8083"
	DefaultServiceVersion     = "dev"
	DefaultEnvironment        = "local"
	DefaultMaxUploadBytes     = int64(32 << 20)
	DefaultOCRServiceTimeout  = 30 * time.Second
	DefaultEmbeddingProvider  = "local_hashing"
	DefaultEmbeddingModel     = "local_hashing"
	DefaultEmbeddingDimension = 384
	DefaultQdrantCollection   = "knowledge_chunks"
	DefaultShutdownTimeout    = 10 * time.Second
)

type Config struct {
	HTTPAddr           string
	ServiceVersion     string
	Environment        string
	DatabaseURL        string
	FileServiceURL     string
	RedisAddr          string
	ServiceToken       string
	MaxUploadBytes     int64
	OCRServiceBaseURL  string
	OCRServiceToken    string
	OCRServiceTimeout  time.Duration
	EmbeddingProvider  string
	EmbeddingModel     string
	EmbeddingDimension int
	AIGatewayBaseURL   string
	AIGatewayToken     string
	AIGatewayProfileID string
	QdrantURL          string
	QdrantAPIKey       string
	QdrantCollection   string
	ShutdownTimeout    time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:           stringValue("KNOWLEDGE_HTTP_ADDR", DefaultHTTPAddr),
		ServiceVersion:     stringValue("KNOWLEDGE_SERVICE_VERSION", DefaultServiceVersion),
		Environment:        stringValue("KNOWLEDGE_ENV", DefaultEnvironment),
		DatabaseURL:        strings.TrimSpace(os.Getenv("DATABASE_URL")),
		FileServiceURL:     trimTrailingSlash(os.Getenv("FILE_SERVICE_BASE_URL")),
		RedisAddr:          strings.TrimSpace(os.Getenv("KNOWLEDGE_REDIS_ADDR")),
		ServiceToken:       strings.TrimSpace(os.Getenv("KNOWLEDGE_SERVICE_TOKEN")),
		MaxUploadBytes:     DefaultMaxUploadBytes,
		OCRServiceBaseURL:  trimTrailingSlash(os.Getenv("OCR_SERVICE_BASE_URL")),
		OCRServiceToken:    strings.TrimSpace(os.Getenv("OCR_SERVICE_TOKEN")),
		OCRServiceTimeout:  DefaultOCRServiceTimeout,
		EmbeddingProvider:  stringValue("EMBEDDING_PROVIDER", DefaultEmbeddingProvider),
		EmbeddingModel:     stringValue("EMBEDDING_MODEL", DefaultEmbeddingModel),
		EmbeddingDimension: DefaultEmbeddingDimension,
		AIGatewayBaseURL:   trimTrailingSlash(os.Getenv("AI_GATEWAY_BASE_URL")),
		AIGatewayToken:     strings.TrimSpace(os.Getenv("AI_GATEWAY_SERVICE_TOKEN")),
		AIGatewayProfileID: strings.TrimSpace(os.Getenv("AI_GATEWAY_EMBEDDING_PROFILE_ID")),
		QdrantURL:          trimTrailingSlash(os.Getenv("QDRANT_URL")),
		QdrantAPIKey:       strings.TrimSpace(os.Getenv("QDRANT_API_KEY")),
		QdrantCollection:   stringValue("QDRANT_COLLECTION", DefaultQdrantCollection),
		ShutdownTimeout:    DefaultShutdownTimeout,
	}

	if raw := os.Getenv("KNOWLEDGE_MAX_UPLOAD_BYTES"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("KNOWLEDGE_MAX_UPLOAD_BYTES must be a positive integer")
		}
		cfg.MaxUploadBytes = value
	}

	if raw := os.Getenv("KNOWLEDGE_SHUTDOWN_TIMEOUT"); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("KNOWLEDGE_SHUTDOWN_TIMEOUT must be a positive duration")
		}
		cfg.ShutdownTimeout = value
	}
	if raw := os.Getenv("OCR_SERVICE_TIMEOUT"); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("OCR_SERVICE_TIMEOUT must be a positive duration")
		}
		cfg.OCRServiceTimeout = value
	}
	if raw := os.Getenv("EMBEDDING_DIMENSION"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("EMBEDDING_DIMENSION must be a positive integer")
		}
		cfg.EmbeddingDimension = value
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if err := validateHTTPURL("FILE_SERVICE_BASE_URL", cfg.FileServiceURL); err != nil {
		return Config{}, err
	}
	if cfg.RedisAddr == "" {
		return Config{}, fmt.Errorf("KNOWLEDGE_REDIS_ADDR is required")
	}
	if cfg.ServiceToken == "" {
		return Config{}, fmt.Errorf("KNOWLEDGE_SERVICE_TOKEN is required")
	}
	for name, value := range map[string]string{
		"AI_GATEWAY_BASE_URL": cfg.AIGatewayBaseURL,
		"OCR_SERVICE_BASE_URL": cfg.OCRServiceBaseURL,
		"QDRANT_URL":           cfg.QdrantURL,
	} {
		if err := validateOptionalHTTPURL(name, value); err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}

func stringValue(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func trimTrailingSlash(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func validateHTTPURL(name string, value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s must be an absolute http(s) URL", name)
	}
	if parsed.User != nil {
		return fmt.Errorf("%s must not contain credentials", name)
	}
	return nil
}

func validateOptionalHTTPURL(name string, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return validateHTTPURL(name, value)
}
