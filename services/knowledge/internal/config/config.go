package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultHTTPAddr           = ":8000"
	DefaultServiceVersion     = "0.3.0"
	DefaultEnvironment        = "local"
	DefaultStorageBackend     = "memory"
	DefaultDatabaseURL        = ""
	DefaultFileServiceBaseURL = ""
	DefaultEmbeddingProvider  = "local_hashing"
	DefaultEmbeddingModel     = "local_hashing"
	DefaultEmbeddingDimension = 384
	DefaultQdrantURL          = ""
	DefaultQdrantAPIKey       = ""
	DefaultQdrantCollection   = "knowledge_chunks"
	DefaultShutdownTimeout    = 10 * time.Second
)

type Config struct {
	HTTPAddr           string
	ServiceVersion     string
	Environment        string
	StorageBackend     string
	DatabaseURL        string
	FileServiceBaseURL string
	EmbeddingProvider  string
	EmbeddingModel     string
	EmbeddingDimension int
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
		StorageBackend:     stringValue("KNOWLEDGE_STORAGE_BACKEND", DefaultStorageBackend),
		DatabaseURL:        stringValue("DATABASE_URL", DefaultDatabaseURL),
		FileServiceBaseURL: stringValue("FILE_SERVICE_BASE_URL", DefaultFileServiceBaseURL),
		EmbeddingProvider:  stringValue("EMBEDDING_PROVIDER", DefaultEmbeddingProvider),
		EmbeddingModel:     stringValue("EMBEDDING_MODEL", DefaultEmbeddingModel),
		EmbeddingDimension: DefaultEmbeddingDimension,
		QdrantURL:          strings.TrimRight(stringValue("QDRANT_URL", DefaultQdrantURL), "/"),
		QdrantAPIKey:       stringValue("QDRANT_API_KEY", DefaultQdrantAPIKey),
		QdrantCollection:   stringValue("QDRANT_COLLECTION", DefaultQdrantCollection),
		ShutdownTimeout:    DefaultShutdownTimeout,
	}

	if raw := os.Getenv("EMBEDDING_DIMENSION"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("EMBEDDING_DIMENSION must be a positive integer")
		}
		cfg.EmbeddingDimension = value
	}

	if raw := os.Getenv("KNOWLEDGE_SHUTDOWN_TIMEOUT"); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("KNOWLEDGE_SHUTDOWN_TIMEOUT must be a positive duration")
		}
		cfg.ShutdownTimeout = value
	}

	if strings.TrimSpace(cfg.HTTPAddr) == "" {
		return Config{}, fmt.Errorf("KNOWLEDGE_HTTP_ADDR must not be empty")
	}
	if strings.TrimSpace(cfg.QdrantCollection) == "" {
		return Config{}, fmt.Errorf("QDRANT_COLLECTION must not be empty")
	}
	switch cfg.StorageBackend {
	case "memory":
	case "postgres":
		if strings.TrimSpace(cfg.DatabaseURL) == "" {
			return Config{}, fmt.Errorf("DATABASE_URL must not be empty when KNOWLEDGE_STORAGE_BACKEND=postgres")
		}
	default:
		return Config{}, fmt.Errorf("KNOWLEDGE_STORAGE_BACKEND=%q is not implemented; supported values: memory, postgres", cfg.StorageBackend)
	}

	return cfg, nil
}

func stringValue(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
