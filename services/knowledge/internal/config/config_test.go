package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadValidatesUploadDependencies(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://knowledge:knowledge@localhost:5432/knowledge?sslmode=disable")
	t.Setenv("FILE_SERVICE_BASE_URL", "http://localhost:8082")
	t.Setenv("KNOWLEDGE_REDIS_ADDR", "localhost:6379")
	t.Setenv("KNOWLEDGE_SERVICE_TOKEN", "knowledge-token")
	t.Setenv("KNOWLEDGE_MAX_UPLOAD_BYTES", "1024")
	t.Setenv("KNOWLEDGE_SHUTDOWN_TIMEOUT", "7s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.FileServiceURL != "http://localhost:8082" || cfg.RedisAddr != "localhost:6379" {
		t.Fatalf("dependency config = %+v", cfg)
	}
	if cfg.MaxUploadBytes != 1024 {
		t.Fatalf("MaxUploadBytes = %d", cfg.MaxUploadBytes)
	}
	if cfg.ShutdownTimeout != 7*time.Second {
		t.Fatalf("ShutdownTimeout = %s", cfg.ShutdownTimeout)
	}
}

func TestLoadRuntimeAdapters(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://knowledge:knowledge@localhost:5432/knowledge?sslmode=disable")
	t.Setenv("FILE_SERVICE_BASE_URL", "http://localhost:8082/")
	t.Setenv("KNOWLEDGE_REDIS_ADDR", "localhost:6379")
	t.Setenv("KNOWLEDGE_SERVICE_TOKEN", "knowledge-token")
	t.Setenv("OCR_SERVICE_BASE_URL", "https://ocr.internal/")
	t.Setenv("OCR_SERVICE_TOKEN", "ocr-token")
	t.Setenv("OCR_SERVICE_TIMEOUT", "45s")
	t.Setenv("EMBEDDING_PROVIDER", "ai_gateway")
	t.Setenv("EMBEDDING_MODEL", "text-embedding-3-small")
	t.Setenv("EMBEDDING_DIMENSION", "1536")
	t.Setenv("AI_GATEWAY_BASE_URL", "https://ai.internal/")
	t.Setenv("AI_GATEWAY_SERVICE_TOKEN", "ai-token")
	t.Setenv("AI_GATEWAY_EMBEDDING_PROFILE_ID", "profile_1")
	t.Setenv("QDRANT_URL", "http://qdrant.local:6333/")
	t.Setenv("QDRANT_API_KEY", "qdrant-key")
	t.Setenv("QDRANT_COLLECTION", "kb_chunks")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.FileServiceURL != "http://localhost:8082" ||
		cfg.OCRServiceBaseURL != "https://ocr.internal" ||
		cfg.AIGatewayBaseURL != "https://ai.internal" ||
		cfg.QdrantURL != "http://qdrant.local:6333" {
		t.Fatalf("trimmed urls = %+v", cfg)
	}
	if cfg.OCRServiceToken != "ocr-token" || cfg.OCRServiceTimeout != 45*time.Second {
		t.Fatalf("ocr config = %+v", cfg)
	}
	if cfg.EmbeddingProvider != "ai_gateway" ||
		cfg.EmbeddingModel != "text-embedding-3-small" ||
		cfg.EmbeddingDimension != 1536 ||
		cfg.AIGatewayToken != "ai-token" ||
		cfg.AIGatewayProfileID != "profile_1" {
		t.Fatalf("embedding config = %+v", cfg)
	}
	if cfg.QdrantAPIKey != "qdrant-key" || cfg.QdrantCollection != "kb_chunks" {
		t.Fatalf("qdrant config = %+v", cfg)
	}
}

func TestLoadRejectsMissingFileServiceURL(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://knowledge:knowledge@localhost:5432/knowledge?sslmode=disable")
	t.Setenv("KNOWLEDGE_REDIS_ADDR", "localhost:6379")
	t.Setenv("KNOWLEDGE_SERVICE_TOKEN", "knowledge-token")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil")
	}
	if !strings.Contains(err.Error(), "FILE_SERVICE_BASE_URL") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRejectsMissingServiceToken(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://knowledge:knowledge@localhost:5432/knowledge?sslmode=disable")
	t.Setenv("FILE_SERVICE_BASE_URL", "http://localhost:8082")
	t.Setenv("KNOWLEDGE_REDIS_ADDR", "localhost:6379")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil")
	}
	if !strings.Contains(err.Error(), "KNOWLEDGE_SERVICE_TOKEN") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRejectsInvalidAdapterConfig(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  string
		want string
	}{
		{name: "ocr timeout", key: "OCR_SERVICE_TIMEOUT", val: "0s", want: "OCR_SERVICE_TIMEOUT"},
		{name: "embedding dimension", key: "EMBEDDING_DIMENSION", val: "0", want: "EMBEDDING_DIMENSION"},
		{name: "ocr url", key: "OCR_SERVICE_BASE_URL", val: "ftp://ocr.internal", want: "OCR_SERVICE_BASE_URL"},
		{name: "qdrant url credentials", key: "QDRANT_URL", val: "http://user:pass@qdrant.local", want: "QDRANT_URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("DATABASE_URL", "postgres://knowledge:knowledge@localhost:5432/knowledge?sslmode=disable")
			t.Setenv("FILE_SERVICE_BASE_URL", "http://localhost:8082")
			t.Setenv("KNOWLEDGE_REDIS_ADDR", "localhost:6379")
			t.Setenv("KNOWLEDGE_SERVICE_TOKEN", "knowledge-token")
			t.Setenv(tt.key, tt.val)

			_, err := Load()
			if err == nil {
				t.Fatal("Load() error = nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DATABASE_URL",
		"FILE_SERVICE_BASE_URL",
		"KNOWLEDGE_REDIS_ADDR",
		"KNOWLEDGE_SERVICE_TOKEN",
		"KNOWLEDGE_MAX_UPLOAD_BYTES",
		"KNOWLEDGE_HTTP_ADDR",
		"KNOWLEDGE_SERVICE_VERSION",
		"KNOWLEDGE_ENV",
		"KNOWLEDGE_SHUTDOWN_TIMEOUT",
		"OCR_SERVICE_BASE_URL",
		"OCR_SERVICE_TOKEN",
		"OCR_SERVICE_TIMEOUT",
		"EMBEDDING_PROVIDER",
		"EMBEDDING_MODEL",
		"EMBEDDING_DIMENSION",
		"AI_GATEWAY_BASE_URL",
		"AI_GATEWAY_SERVICE_TOKEN",
		"AI_GATEWAY_EMBEDDING_PROFILE_ID",
		"QDRANT_URL",
		"QDRANT_API_KEY",
		"QDRANT_COLLECTION",
	} {
		t.Setenv(key, "")
	}
}
