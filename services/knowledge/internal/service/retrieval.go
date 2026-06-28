package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	maxKnowledgeQueryLength = 2000
	maxRetrievalTopK        = 100
	defaultContentPreview   = 500
)

type KnowledgeQueryInput struct {
	Query            string
	KnowledgeBaseIDs []string
	TopK             int
	ScoreThreshold   *float64
	Tags             []string
	MetadataFilter   map[string]string
	Rerank           bool
	RerankTopN       *int
}

type KnowledgeQuerySummary struct {
	ID      string
	Query   string
	Results []KnowledgeQueryResult
	Trace   KnowledgeQueryTrace
}

type KnowledgeQueryResult struct {
	Score           float64
	PointID         string
	KnowledgeBaseID string
	DocumentID      string
	ChunkID         string
	DocumentName    string
	SectionPath     *string
	ChunkIndex      *int
	ChunkType       *string
	ContentPreview  string
	Tags            []string
}

type KnowledgeQueryTrace struct {
	EmbeddingProvider  string
	EmbeddingModel     string
	EmbeddingDimension int
	QdrantCollection   string
	SearchTopK         int
	ScoreThreshold     float64
	HitCount           int
	Rerank             bool
	RerankTopN         *int
}

func (s *KnowledgeService) CreateKnowledgeQuery(ctx context.Context, reqCtx RequestContext, input KnowledgeQueryInput) (KnowledgeQuerySummary, error) {
	if err := validateActor(reqCtx); err != nil {
		return KnowledgeQuerySummary{}, err
	}

	query := strings.TrimSpace(input.Query)
	fields := map[string]string{}
	if query == "" {
		fields["query"] = "is required"
	} else if len(query) > maxKnowledgeQueryLength {
		fields["query"] = fmt.Sprintf("must be at most %d characters", maxKnowledgeQueryLength)
	}
	runtimeConfig := s.getRuntimeConfig()
	topK := input.TopK
	if topK == 0 {
		topK = runtimeConfig.RetrievalTopK
	}
	if topK < 1 || topK > maxRetrievalTopK {
		fields["topK"] = fmt.Sprintf("must be between 1 and %d", maxRetrievalTopK)
	}
	scoreThreshold := runtimeConfig.ScoreThreshold
	if input.ScoreThreshold != nil {
		scoreThreshold = *input.ScoreThreshold
	}
	if scoreThreshold < 0 {
		fields["scoreThreshold"] = "must be non-negative"
	}
	rerankTopN := cloneIntPointer(input.RerankTopN)
	if rerankTopN != nil && (*rerankTopN < 1 || *rerankTopN > topK) {
		fields["rerankTopN"] = "must be between 1 and topK"
	}
	tags, err := NormalizeTags(input.Tags)
	if err != nil {
		fields["tags"] = err.Error()
	}
	metadataFilter := normalizeMetadataFilter(input.MetadataFilter)
	if len(fields) > 0 {
		return KnowledgeQuerySummary{}, ValidationError("request validation failed", fields)
	}
	if s.embedder == nil || s.vectorIndex == nil {
		return KnowledgeQuerySummary{}, DependencyError("retrieval pipeline is not configured", nil)
	}

	allowedKnowledgeBaseIDs, err := s.resolveRetrievalKnowledgeBases(ctx, reqCtx, input.KnowledgeBaseIDs)
	if err != nil {
		return KnowledgeQuerySummary{}, err
	}
	if len(allowedKnowledgeBaseIDs) == 0 {
		return KnowledgeQuerySummary{}, NotFoundError("knowledge base not found")
	}

	embedding, err := s.embedder.Embed(ctx, EmbeddingRequest{Texts: []string{query}})
	if err != nil {
		return KnowledgeQuerySummary{}, DependencyError("knowledge query embedding failed", err)
	}
	if len(embedding.Vectors) != 1 {
		return KnowledgeQuerySummary{}, DependencyError("knowledge query embedding failed", nil)
	}

	hits, err := s.vectorIndex.Search(ctx, VectorSearchRequest{
		Vector:           embedding.Vectors[0],
		KnowledgeBaseIDs: allowedKnowledgeBaseIDs,
		Tags:             tags,
		MetadataFilter:   metadataFilter,
		Limit:            topK,
		ScoreThreshold:   scoreThreshold,
	})
	if err != nil {
		return KnowledgeQuerySummary{}, DependencyError("knowledge vector search failed", err)
	}

	results, err := s.hydrateRetrievalResults(ctx, reqCtx, hits)
	if err != nil {
		return KnowledgeQuerySummary{}, err
	}
	if rerankTopN != nil && len(results) > *rerankTopN {
		results = results[:*rerankTopN]
	}

	queryID, err := s.newID("kq")
	if err != nil {
		return KnowledgeQuerySummary{}, DependencyError("knowledge query id generation failed", err)
	}
	return KnowledgeQuerySummary{
		ID:      queryID,
		Query:   query,
		Results: results,
		Trace: KnowledgeQueryTrace{
			EmbeddingProvider:  embedding.Provider,
			EmbeddingModel:     embedding.Model,
			EmbeddingDimension: embedding.Dimension,
			QdrantCollection:   s.vectorCollection,
			SearchTopK:         topK,
			ScoreThreshold:     scoreThreshold,
			HitCount:           len(results),
			Rerank:             input.Rerank,
			RerankTopN:         rerankTopN,
		},
	}, nil
}

func (s *KnowledgeService) resolveRetrievalKnowledgeBases(ctx context.Context, reqCtx RequestContext, ids []string) ([]string, error) {
	normalized := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		base, err := s.repo.FindKnowledgeBaseByID(ctx, id)
		if err != nil {
			return nil, mapKnowledgeBaseRepositoryError(err, "knowledge base not found", "knowledge base metadata access failed")
		}
		if !canAccessKnowledgeBase(reqCtx, base) {
			return nil, NotFoundError("knowledge base not found")
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	if len(normalized) > 0 {
		return normalized, nil
	}

	list, err := s.repo.ListKnowledgeBases(ctx, KnowledgeBaseFilter{
		OwnerUserID: ownerFilter(reqCtx),
		Page:        1,
		PageSize:    200,
	})
	if err != nil {
		return nil, DependencyError("knowledge base metadata access failed", err)
	}
	for _, base := range list.Items {
		normalized = append(normalized, base.ID)
	}
	return normalized, nil
}

func (s *KnowledgeService) hydrateRetrievalResults(ctx context.Context, reqCtx RequestContext, hits []VectorSearchHit) ([]KnowledgeQueryResult, error) {
	if len(hits) == 0 {
		return []KnowledgeQueryResult{}, nil
	}
	chunkIDs := make([]string, 0, len(hits))
	for _, hit := range hits {
		if chunkID := stringPayload(hit.Payload, "chunk_id"); chunkID != "" {
			chunkIDs = append(chunkIDs, chunkID)
		}
	}
	chunks, err := s.repo.FindChunksByIDs(ctx, chunkIDs)
	if err != nil {
		return nil, DependencyError("document chunks access failed", err)
	}
	chunksByID := make(map[string]DocumentChunk, len(chunks))
	for _, chunk := range chunks {
		chunksByID[chunk.ID] = chunk
	}

	results := make([]KnowledgeQueryResult, 0, len(hits))
	for _, hit := range hits {
		chunkID := stringPayload(hit.Payload, "chunk_id")
		chunk, ok := chunksByID[chunkID]
		if !ok {
			continue
		}
		doc, err := s.repo.FindDocumentByID(ctx, chunk.DocumentID)
		if err != nil {
			if errorsIsNotFound(err) {
				continue
			}
			return nil, mapDocumentRepositoryError(err, "document not found", "document metadata access failed")
		}
		if doc.Status != DocumentStatusReady || !canAccessKnowledgeDocument(reqCtx, doc) {
			continue
		}
		chunkIndex := chunk.ChunkIndex
		results = append(results, KnowledgeQueryResult{
			Score:           hit.Score,
			PointID:         hit.ID,
			KnowledgeBaseID: chunk.KnowledgeBaseID,
			DocumentID:      chunk.DocumentID,
			ChunkID:         chunk.ID,
			DocumentName:    doc.Name,
			SectionPath:     cloneStringPointer(chunk.SectionPath),
			ChunkIndex:      &chunkIndex,
			ChunkType:       cloneStringPointer(chunk.ChunkType),
			ContentPreview:  contentPreview(chunk.Content, defaultContentPreview),
			Tags:            append([]string(nil), doc.Tags...),
		})
	}
	return results, nil
}

func normalizeMetadataFilter(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	filter := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		filter[key] = value
	}
	return filter
}

func stringPayload(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func contentPreview(content string, limit int) string {
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if len(runes) <= limit {
		return content
	}
	return string(runes[:limit])
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
