package service

import (
	"context"
	"crypto/sha256"
	"fmt"
)

type EmbeddingRequest struct {
	Texts []string
}

type EmbeddingResult struct {
	Vectors   [][]float32
	Provider  string
	Model     string
	Dimension int
}

type Embedder interface {
	Embed(ctx context.Context, request EmbeddingRequest) (EmbeddingResult, error)
}

type VectorPoint struct {
	ID      string
	Vector  []float32
	Payload map[string]any
}

type VectorSearchRequest struct {
	Vector           []float32
	KnowledgeBaseIDs []string
	Tags             []string
	MetadataFilter   map[string]string
	Limit            int
	ScoreThreshold   float64
}

type VectorSearchHit struct {
	ID      string
	Score   float64
	Payload map[string]any
}

type VectorIndex interface {
	Upsert(ctx context.Context, points []VectorPoint) error
	DeleteByDocument(ctx context.Context, documentID string) error
	Search(ctx context.Context, request VectorSearchRequest) ([]VectorSearchHit, error)
}

func stableVectorPointID(sourceID string) string {
	sum := sha256.Sum256([]byte(sourceID))
	sum[6] = (sum[6] & 0x0f) | 0x50
	sum[8] = (sum[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}
