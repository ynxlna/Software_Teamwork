package service

import (
	"context"
	"strings"
)

type StatusConfig struct {
	Version            string
	Environment        string
	StorageBackend     string
	EmbeddingProvider  string
	EmbeddingModel     string
	EmbeddingDimension int
	QdrantCollection   string
}

type StatusService struct {
	cfg StatusConfig
}

type HealthStatus struct {
	Service string
	Status  string
}

type ReadyStatus struct {
	Service            string
	Status             string
	Version            string
	Environment        string
	StorageBackend     string
	EmbeddingProvider  string
	EmbeddingModel     string
	EmbeddingDimension int
	QdrantCollection   string
}

func NewStatusService(cfg StatusConfig) *StatusService {
	return &StatusService{cfg: cfg}
}

func (s *StatusService) Health(_ context.Context) HealthStatus {
	return HealthStatus{
		Service: "knowledge",
		Status:  "ok",
	}
}

func (s *StatusService) Ready(_ context.Context) (ReadyStatus, error) {
	fields := map[string]string{}
	if strings.TrimSpace(s.cfg.StorageBackend) == "" {
		fields["storageBackend"] = "is required"
	}
	if strings.TrimSpace(s.cfg.EmbeddingProvider) == "" {
		fields["embeddingProvider"] = "is required"
	}
	if s.cfg.EmbeddingDimension <= 0 {
		fields["embeddingDimension"] = "must be positive"
	}
	if strings.TrimSpace(s.cfg.QdrantCollection) == "" {
		fields["qdrantCollection"] = "is required"
	}
	if len(fields) > 0 {
		return ReadyStatus{}, ValidationError("readiness configuration is invalid", fields)
	}

	return ReadyStatus{
		Service:            "knowledge",
		Status:             "ready",
		Version:            s.cfg.Version,
		Environment:        s.cfg.Environment,
		StorageBackend:     s.cfg.StorageBackend,
		EmbeddingProvider:  s.cfg.EmbeddingProvider,
		EmbeddingModel:     s.cfg.EmbeddingModel,
		EmbeddingDimension: s.cfg.EmbeddingDimension,
		QdrantCollection:   s.cfg.QdrantCollection,
	}, nil
}
