package config_test

import (
	"strings"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/file/internal/config"
)

func TestLoadAcceptsLocalStorageBackend(t *testing.T) {
	t.Setenv("FILE_STORAGE_BACKEND", "local")
	t.Setenv("FILE_LOCAL_STORAGE_DIR", t.TempDir())

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.StorageBackend != "local" || cfg.LocalStorageDir == "" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadAcceptsMinIOStorageBackend(t *testing.T) {
	t.Setenv("FILE_STORAGE_BACKEND", "minio")
	t.Setenv("FILE_MINIO_ENDPOINT", "minio:9000")
	t.Setenv("FILE_MINIO_ACCESS_KEY", "file-access")
	t.Setenv("FILE_MINIO_SECRET_KEY", "file-secret")
	t.Setenv("FILE_MINIO_BUCKET", "file-objects")
	t.Setenv("FILE_MINIO_USE_SSL", "true")
	t.Setenv("FILE_MINIO_REGION", "us-east-1")
	t.Setenv("FILE_MINIO_TIMEOUT", "3s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.StorageBackend != "minio" {
		t.Fatalf("StorageBackend = %q", cfg.StorageBackend)
	}
	if cfg.MinIOEndpoint != "minio:9000" || cfg.MinIOBucket != "file-objects" || !cfg.MinIOUseSSL || cfg.MinIORegion != "us-east-1" {
		t.Fatalf("minio config endpoint=%q bucket=%q useSSL=%t region=%q", cfg.MinIOEndpoint, cfg.MinIOBucket, cfg.MinIOUseSSL, cfg.MinIORegion)
	}
	if cfg.MinIOTimeout.String() != "3s" {
		t.Fatalf("MinIOTimeout = %s", cfg.MinIOTimeout)
	}
}

func TestLoadRejectsMinIOMissingRequiredConfig(t *testing.T) {
	t.Setenv("FILE_STORAGE_BACKEND", "minio")
	t.Setenv("FILE_MINIO_ENDPOINT", "minio:9000")
	t.Setenv("FILE_MINIO_ACCESS_KEY", "file-access")
	t.Setenv("FILE_MINIO_SECRET_KEY", "file-secret")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if got := err.Error(); got == "" || strings.Contains(got, "file-secret") {
		t.Fatalf("Load() error = %q", got)
	}
}

func TestLoadRejectsUnsupportedStorageBackend(t *testing.T) {
	t.Setenv("FILE_STORAGE_BACKEND", "unsupported")

	if _, err := config.Load(); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
