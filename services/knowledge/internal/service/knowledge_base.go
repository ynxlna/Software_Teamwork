package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	DefaultDocType               = "GENERAL"
	DefaultChunkStrategyType     = "SEMANTIC_TEXT"
	DefaultChunkStrategySize     = 1600
	DefaultChunkStrategyOverlap  = 200
	DefaultRetrievalStrategyMode = "VECTOR"
	DefaultRetrievalTopK         = 10
	DefaultScoreThreshold        = 0.35

	maxKnowledgeBaseNameLength        = 120
	maxKnowledgeBaseDescriptionLength = 2000
)

type ChunkStrategy map[string]any
type RetrievalStrategy map[string]any

type KnowledgeBase struct {
	ID                string
	Name              string
	Description       string
	DocType           string
	ChunkStrategy     ChunkStrategy
	RetrievalStrategy RetrievalStrategy
	DocumentCount     int
	ChunkCount        int
	CreatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

type Page struct {
	Page     int
	PageSize int
	Total    int
}

type KnowledgeBaseList struct {
	Items []KnowledgeBase
	Page  Page
}

type ListKnowledgeBasesInput struct {
	Page     int
	PageSize int
	Keyword  string
	DocType  string
}

type CreateKnowledgeBaseInput struct {
	ID                string
	Name              string
	Description       string
	DocType           string
	ChunkStrategy     ChunkStrategy
	RetrievalStrategy RetrievalStrategy
}

type UpdateKnowledgeBaseInput struct {
	ID                string
	Name              *string
	Description       *string
	DocType           *string
	ChunkStrategy     ChunkStrategy
	RetrievalStrategy RetrievalStrategy
}

type KnowledgeBaseRepository interface {
	CreateKnowledgeBase(ctx context.Context, base KnowledgeBase) (KnowledgeBase, error)
	ListKnowledgeBases(ctx context.Context, filter KnowledgeBaseFilter) (KnowledgeBaseList, error)
	FindKnowledgeBaseByID(ctx context.Context, id string) (KnowledgeBase, error)
	UpdateKnowledgeBase(ctx context.Context, base KnowledgeBase) (KnowledgeBase, error)
	MarkKnowledgeBaseDeleted(ctx context.Context, id string, deletedAt time.Time) error
}

type Repository interface {
	KnowledgeBaseRepository
	DocumentRepository
	ChunkRepository
	JobRepository
	StatsRepository
}

type KnowledgeBaseFilter struct {
	OwnerUserID string
	Page        int
	PageSize    int
	Keyword     string
	DocType     string
}

type KnowledgeService struct {
	repo             Repository
	sourceReader     SourceReader
	parser           Parser
	chunker          Chunker
	embedder         Embedder
	vectorIndex      VectorIndex
	vectorCollection string
	runtimeMu        sync.RWMutex
	runtimeConfig    RuntimeConfig
	now              func() time.Time
	newID            func(prefix string) (string, error)
}

type KnowledgeOption func(*KnowledgeService)

func NewKnowledgeService(repo Repository, opts ...KnowledgeOption) *KnowledgeService {
	s := &KnowledgeService{
		repo:  repo,
		now:   func() time.Time { return time.Now().UTC() },
		newID: newPublicID,
	}
	s.runtimeConfig = RuntimeConfig{
		EmbeddingProvider:    "local_hashing",
		EmbeddingModel:       "local_hashing",
		EmbeddingDimension:   384,
		QdrantCollection:     "knowledge_chunks",
		ParserBackend:        "text",
		RerankProvider:       "none",
		RetrievalTopK:        DefaultRetrievalTopK,
		ScoreThreshold:       DefaultScoreThreshold,
		MaxConcurrentJobs:    1,
		ProcessingTimeoutSec: 300,
		SecretRefs:           map[string]string{},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func WithClock(now func() time.Time) KnowledgeOption {
	return func(s *KnowledgeService) {
		if now != nil {
			s.now = now
		}
	}
}

func WithIDGenerator(newID func(prefix string) (string, error)) KnowledgeOption {
	return func(s *KnowledgeService) {
		if newID != nil {
			s.newID = newID
		}
	}
}

func WithPipeline(sourceReader SourceReader, parser Parser, chunker Chunker) KnowledgeOption {
	return func(s *KnowledgeService) {
		s.sourceReader = sourceReader
		s.parser = parser
		s.chunker = chunker
	}
}

func WithVectorIndex(embedder Embedder, vectorIndex VectorIndex, collection ...string) KnowledgeOption {
	return func(s *KnowledgeService) {
		s.embedder = embedder
		s.vectorIndex = vectorIndex
		if len(collection) > 0 {
			s.vectorCollection = strings.TrimSpace(collection[0])
			s.runtimeConfig.QdrantCollection = s.vectorCollection
		}
	}
}

func WithRuntimeConfig(cfg RuntimeConfig) KnowledgeOption {
	return func(s *KnowledgeService) {
		if strings.TrimSpace(cfg.EmbeddingProvider) != "" {
			s.runtimeConfig.EmbeddingProvider = strings.TrimSpace(cfg.EmbeddingProvider)
		}
		if strings.TrimSpace(cfg.EmbeddingModel) != "" {
			s.runtimeConfig.EmbeddingModel = strings.TrimSpace(cfg.EmbeddingModel)
		}
		if cfg.EmbeddingDimension > 0 {
			s.runtimeConfig.EmbeddingDimension = cfg.EmbeddingDimension
		}
		if strings.TrimSpace(cfg.QdrantCollection) != "" {
			s.runtimeConfig.QdrantCollection = strings.TrimSpace(cfg.QdrantCollection)
			s.vectorCollection = s.runtimeConfig.QdrantCollection
		}
		if strings.TrimSpace(cfg.ParserBackend) != "" {
			s.runtimeConfig.ParserBackend = strings.TrimSpace(cfg.ParserBackend)
		}
		if strings.TrimSpace(cfg.RerankProvider) != "" {
			s.runtimeConfig.RerankProvider = strings.TrimSpace(cfg.RerankProvider)
		}
		if strings.TrimSpace(cfg.RerankModel) != "" {
			s.runtimeConfig.RerankModel = strings.TrimSpace(cfg.RerankModel)
		}
		if cfg.RetrievalTopK > 0 {
			s.runtimeConfig.RetrievalTopK = cfg.RetrievalTopK
		}
		if cfg.ScoreThreshold >= 0 {
			s.runtimeConfig.ScoreThreshold = cfg.ScoreThreshold
		}
		if cfg.MaxConcurrentJobs > 0 {
			s.runtimeConfig.MaxConcurrentJobs = cfg.MaxConcurrentJobs
		}
		if cfg.ProcessingTimeoutSec > 0 {
			s.runtimeConfig.ProcessingTimeoutSec = cfg.ProcessingTimeoutSec
		}
		if cfg.SecretRefs != nil {
			s.runtimeConfig.SecretRefs = normalizeSecretRefs(cfg.SecretRefs)
		}
	}
}

func (s *KnowledgeService) ListKnowledgeBases(ctx context.Context, reqCtx RequestContext, input ListKnowledgeBasesInput) (KnowledgeBaseList, error) {
	if err := validateActor(reqCtx); err != nil {
		return KnowledgeBaseList{}, err
	}

	page, pageSize, err := normalizePagination(input.Page, input.PageSize)
	if err != nil {
		return KnowledgeBaseList{}, err
	}
	docType, err := normalizeOptionalDocType(input.DocType)
	if err != nil {
		return KnowledgeBaseList{}, ValidationError("request validation failed", map[string]string{"docType": err.Error()})
	}

	result, err := s.repo.ListKnowledgeBases(ctx, KnowledgeBaseFilter{
		OwnerUserID: ownerFilter(reqCtx),
		Page:        page,
		PageSize:    pageSize,
		Keyword:     strings.TrimSpace(input.Keyword),
		DocType:     docType,
	})
	if err != nil {
		return KnowledgeBaseList{}, DependencyError("knowledge base metadata access failed", err)
	}
	return result, nil
}

func (s *KnowledgeService) CreateKnowledgeBase(ctx context.Context, reqCtx RequestContext, input CreateKnowledgeBaseInput) (KnowledgeBase, error) {
	if err := validateActor(reqCtx); err != nil {
		return KnowledgeBase{}, err
	}

	fields := map[string]string{}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		fields["name"] = "is required"
	} else if len(name) > maxKnowledgeBaseNameLength {
		fields["name"] = fmt.Sprintf("must be at most %d characters", maxKnowledgeBaseNameLength)
	}
	description := strings.TrimSpace(input.Description)
	if len(description) > maxKnowledgeBaseDescriptionLength {
		fields["description"] = fmt.Sprintf("must be at most %d characters", maxKnowledgeBaseDescriptionLength)
	}
	docType, err := normalizeDocType(input.DocType)
	if err != nil {
		fields["docType"] = err.Error()
	}
	chunkStrategy, err := normalizeChunkStrategy(input.ChunkStrategy)
	if err != nil {
		fields["chunkStrategy"] = err.Error()
	}
	retrievalStrategy, err := normalizeRetrievalStrategy(input.RetrievalStrategy)
	if err != nil {
		fields["retrievalStrategy"] = err.Error()
	}
	if len(fields) > 0 {
		return KnowledgeBase{}, ValidationError("request validation failed", fields)
	}

	id := strings.TrimSpace(input.ID)
	if id == "" {
		generated, err := s.newID("kb")
		if err != nil {
			return KnowledgeBase{}, DependencyError("knowledge base id generation failed", err)
		}
		id = generated
	} else if err := validatePublicID(id, "kb"); err != nil {
		return KnowledgeBase{}, ValidationError("request validation failed", map[string]string{"id": err.Error()})
	}

	now := s.now().UTC()
	base := KnowledgeBase{
		ID:                id,
		Name:              name,
		Description:       description,
		DocType:           docType,
		ChunkStrategy:     chunkStrategy,
		RetrievalStrategy: retrievalStrategy,
		CreatedBy:         strings.TrimSpace(reqCtx.UserID),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	created, err := s.repo.CreateKnowledgeBase(ctx, base)
	if err != nil {
		return KnowledgeBase{}, mapKnowledgeBaseRepositoryError(err, "knowledge base already exists", "knowledge base metadata write failed")
	}
	return created, nil
}

func (s *KnowledgeService) GetKnowledgeBase(ctx context.Context, reqCtx RequestContext, id string) (KnowledgeBase, error) {
	if err := validateActor(reqCtx); err != nil {
		return KnowledgeBase{}, err
	}
	knowledgeBaseID := strings.TrimSpace(id)
	if knowledgeBaseID == "" {
		return KnowledgeBase{}, ValidationError("request validation failed", map[string]string{"knowledgeBaseId": "is required"})
	}
	base, err := s.repo.FindKnowledgeBaseByID(ctx, knowledgeBaseID)
	if err != nil {
		return KnowledgeBase{}, mapKnowledgeBaseRepositoryError(err, "knowledge base not found", "knowledge base metadata access failed")
	}
	if !canAccessKnowledgeBase(reqCtx, base) {
		return KnowledgeBase{}, NotFoundError("knowledge base not found")
	}
	return base, nil
}

func (s *KnowledgeService) UpdateKnowledgeBase(ctx context.Context, reqCtx RequestContext, input UpdateKnowledgeBaseInput) (KnowledgeBase, error) {
	if err := validateActor(reqCtx); err != nil {
		return KnowledgeBase{}, err
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return KnowledgeBase{}, ValidationError("request validation failed", map[string]string{"knowledgeBaseId": "is required"})
	}

	existing, err := s.repo.FindKnowledgeBaseByID(ctx, id)
	if err != nil {
		return KnowledgeBase{}, mapKnowledgeBaseRepositoryError(err, "knowledge base not found", "knowledge base metadata access failed")
	}
	if !canAccessKnowledgeBase(reqCtx, existing) {
		return KnowledgeBase{}, NotFoundError("knowledge base not found")
	}

	fields := map[string]string{}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			fields["name"] = "must not be empty"
		} else if len(name) > maxKnowledgeBaseNameLength {
			fields["name"] = fmt.Sprintf("must be at most %d characters", maxKnowledgeBaseNameLength)
		} else {
			existing.Name = name
		}
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		if len(description) > maxKnowledgeBaseDescriptionLength {
			fields["description"] = fmt.Sprintf("must be at most %d characters", maxKnowledgeBaseDescriptionLength)
		} else {
			existing.Description = description
		}
	}
	if input.DocType != nil {
		docType, err := normalizeDocType(*input.DocType)
		if err != nil {
			fields["docType"] = err.Error()
		} else {
			existing.DocType = docType
		}
	}
	if input.ChunkStrategy != nil {
		chunkStrategy, err := normalizeChunkStrategy(input.ChunkStrategy)
		if err != nil {
			fields["chunkStrategy"] = err.Error()
		} else {
			existing.ChunkStrategy = chunkStrategy
		}
	}
	if input.RetrievalStrategy != nil {
		retrievalStrategy, err := normalizeRetrievalStrategy(input.RetrievalStrategy)
		if err != nil {
			fields["retrievalStrategy"] = err.Error()
		} else {
			existing.RetrievalStrategy = retrievalStrategy
		}
	}
	if len(fields) > 0 {
		return KnowledgeBase{}, ValidationError("request validation failed", fields)
	}

	existing.UpdatedAt = s.now().UTC()
	updated, err := s.repo.UpdateKnowledgeBase(ctx, existing)
	if err != nil {
		return KnowledgeBase{}, mapKnowledgeBaseRepositoryError(err, "knowledge base not found", "knowledge base metadata write failed")
	}
	return updated, nil
}

func (s *KnowledgeService) DeleteKnowledgeBase(ctx context.Context, reqCtx RequestContext, id string) error {
	if err := validateActor(reqCtx); err != nil {
		return err
	}
	knowledgeBaseID := strings.TrimSpace(id)
	if knowledgeBaseID == "" {
		return ValidationError("request validation failed", map[string]string{"knowledgeBaseId": "is required"})
	}
	existing, err := s.repo.FindKnowledgeBaseByID(ctx, knowledgeBaseID)
	if err != nil {
		return mapKnowledgeBaseRepositoryError(err, "knowledge base not found", "knowledge base metadata access failed")
	}
	if !canAccessKnowledgeBase(reqCtx, existing) {
		return NotFoundError("knowledge base not found")
	}
	if err := s.repo.MarkKnowledgeBaseDeleted(ctx, knowledgeBaseID, s.now().UTC()); err != nil {
		return mapKnowledgeBaseRepositoryError(err, "knowledge base not found", "knowledge base metadata write failed")
	}
	return nil
}

func normalizePagination(page int, pageSize int) (int, int, error) {
	fields := map[string]string{}
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if page < 1 {
		fields["page"] = "must be at least 1"
	}
	if pageSize < 1 || pageSize > 200 {
		fields["pageSize"] = "must be between 1 and 200"
	}
	if len(fields) > 0 {
		return 0, 0, ValidationError("request validation failed", fields)
	}
	return page, pageSize, nil
}

func normalizeOptionalDocType(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return normalizeDocType(value)
}

func normalizeDocType(value string) (string, error) {
	docType := strings.ToUpper(strings.TrimSpace(value))
	if docType == "" {
		return DefaultDocType, nil
	}
	if len(docType) > 64 {
		return "", fmt.Errorf("must be at most 64 characters")
	}
	for _, r := range docType {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return "", fmt.Errorf("must contain only letters, numbers, '_' or '-'")
	}
	return docType, nil
}

func normalizeChunkStrategy(strategy ChunkStrategy) (ChunkStrategy, error) {
	if len(strategy) == 0 {
		return ChunkStrategy{
			"type":       DefaultChunkStrategyType,
			"chunkSize":  DefaultChunkStrategySize,
			"overlap":    DefaultChunkStrategyOverlap,
			"separators": []string{"\n\n", "\n", "。", "."},
		}, nil
	}
	normalized := cloneMap(strategy)
	chunkSize, ok, err := positiveIntField(normalized, "chunkSize")
	if err != nil {
		return nil, err
	}
	if ok && chunkSize <= 0 {
		return nil, fmt.Errorf("chunkSize must be positive")
	}
	overlap, ok, err := positiveIntField(normalized, "overlap")
	if err != nil {
		return nil, err
	}
	if ok && overlap < 0 {
		return nil, fmt.Errorf("overlap must be non-negative")
	}
	if _, ok := normalized["type"]; !ok {
		normalized["type"] = DefaultChunkStrategyType
	}
	return normalized, nil
}

func normalizeRetrievalStrategy(strategy RetrievalStrategy) (RetrievalStrategy, error) {
	if len(strategy) == 0 {
		return RetrievalStrategy{
			"mode":           DefaultRetrievalStrategyMode,
			"topK":           DefaultRetrievalTopK,
			"scoreThreshold": DefaultScoreThreshold,
		}, nil
	}
	normalized := cloneMap(strategy)
	topK, ok, err := positiveIntField(normalized, "topK")
	if err != nil {
		return nil, err
	}
	if ok && (topK < 1 || topK > 100) {
		return nil, fmt.Errorf("topK must be between 1 and 100")
	}
	score, ok, err := numericField(normalized, "scoreThreshold")
	if err != nil {
		return nil, err
	}
	if ok && score < 0 {
		return nil, fmt.Errorf("scoreThreshold must be non-negative")
	}
	rerankTopN, ok, err := positiveIntField(normalized, "rerankTopN")
	if err != nil {
		return nil, err
	}
	if ok && rerankTopN < 1 {
		return nil, fmt.Errorf("rerankTopN must be positive")
	}
	if _, ok := normalized["mode"]; !ok {
		normalized["mode"] = DefaultRetrievalStrategyMode
	}
	return normalized, nil
}

func positiveIntField(values map[string]any, key string) (int, bool, error) {
	value, ok := values[key]
	if !ok || value == nil {
		return 0, false, nil
	}
	n, err := asInt(value)
	if err != nil {
		return 0, true, fmt.Errorf("%s must be an integer", key)
	}
	return n, true, nil
}

func numericField(values map[string]any, key string) (float64, bool, error) {
	value, ok := values[key]
	if !ok || value == nil {
		return 0, false, nil
	}
	switch typed := value.(type) {
	case float64:
		return typed, true, nil
	case float32:
		return float64(typed), true, nil
	case int:
		return float64(typed), true, nil
	case int64:
		return float64(typed), true, nil
	case int32:
		return float64(typed), true, nil
	default:
		return 0, true, fmt.Errorf("%s must be a number", key)
	}
}

func asInt(value any) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case int32:
		return int(typed), nil
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("not an integer")
		}
		return int(typed), nil
	case float32:
		if typed != float32(int(typed)) {
			return 0, fmt.Errorf("not an integer")
		}
		return int(typed), nil
	default:
		return 0, fmt.Errorf("not an integer")
	}
}

func cloneMap[M ~map[string]any](source M) M {
	if source == nil {
		return nil
	}
	clone := make(M, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func ownerFilter(reqCtx RequestContext) string {
	if hasPermission(reqCtx, "knowledge:read:any") {
		return ""
	}
	return strings.TrimSpace(reqCtx.UserID)
}

func canAccessKnowledgeBase(reqCtx RequestContext, base KnowledgeBase) bool {
	userID := strings.TrimSpace(reqCtx.UserID)
	if userID == "" {
		return false
	}
	if hasPermission(reqCtx, "knowledge:read:any") || hasPermission(reqCtx, "knowledge:write:any") {
		return true
	}
	return strings.TrimSpace(base.CreatedBy) == userID
}

func validatePublicID(id string, prefix string) error {
	if !strings.HasPrefix(id, prefix+"_") {
		return fmt.Errorf("must start with %s_", prefix)
	}
	if strings.ContainsAny(id, "\x00\r\n\t /") {
		return fmt.Errorf("contains invalid characters")
	}
	if len(id) > 128 {
		return fmt.Errorf("must be at most 128 characters")
	}
	return nil
}

func mapKnowledgeBaseRepositoryError(err error, conflictMessage string, dependencyMessage string) error {
	if errors.Is(err, ErrNotFound) {
		return NotFoundError("knowledge base not found")
	}
	if errors.Is(err, ErrConflict) {
		return ConflictError(conflictMessage, err)
	}
	return DependencyError(dependencyMessage, err)
}

func newPublicID(prefix string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(bytes), nil
}
