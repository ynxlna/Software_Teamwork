package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

type DocumentStatus string

const (
	DocumentStatusUploaded  DocumentStatus = "uploaded"
	DocumentStatusParsing   DocumentStatus = "parsing"
	DocumentStatusChunking  DocumentStatus = "chunking"
	DocumentStatusEmbedding DocumentStatus = "embedding"
	DocumentStatusReady     DocumentStatus = "ready"
	DocumentStatusFailed    DocumentStatus = "failed"
)

type KnowledgeDocument struct {
	ID              string
	KnowledgeBaseID string
	FileID          string
	Name            string
	ContentType     *string
	SizeBytes       int64
	Status          DocumentStatus
	ErrorCode       *string
	ErrorMessage    *string
	ParsedContent   *string
	ChunkCount      int
	Tags            []string
	ParserBackend   *string
	CreatedBy       string
	CreatedAt       time.Time
	UpdatedAt       *time.Time
	DeletedAt       *time.Time
	CurrentJobID    *string
}

type DocumentChunk struct {
	ID                 string
	KnowledgeBaseID    string
	DocumentID         string
	ChunkIndex         int
	SectionPath        *string
	Content            string
	TokenCount         int
	ChunkType          *string
	QdrantPointID      *string
	EmbeddingProvider  *string
	EmbeddingModel     *string
	EmbeddingDimension *int
	Metadata           map[string]any
	CreatedAt          time.Time
}

type ListDocumentsInput struct {
	KnowledgeBaseID string
	Page            int
	PageSize        int
	Status          string
}

type ListChunksInput struct {
	DocumentID string
	Page       int
	PageSize   int
}

type DocumentList struct {
	Items []KnowledgeDocument
	Page  Page
}

type ChunkList struct {
	Items []DocumentChunk
	Page  Page
}

type DocumentRepository interface {
	ListDocuments(ctx context.Context, filter DocumentFilter) (DocumentList, error)
	FindDocumentByID(ctx context.Context, id string) (KnowledgeDocument, error)
	UpdateDocumentProcessingState(ctx context.Context, id string, update DocumentStateUpdate) (KnowledgeDocument, error)
}

type ChunkRepository interface {
	ListChunks(ctx context.Context, filter ChunkFilter) (ChunkList, error)
	ReplaceDocumentChunks(ctx context.Context, documentID string, chunks []DocumentChunk) error
	FindChunksByIDs(ctx context.Context, ids []string) ([]DocumentChunk, error)
}

type DocumentFilter struct {
	KnowledgeBaseID string
	OwnerUserID     string
	Page            int
	PageSize        int
	Status          DocumentStatus
}

type ChunkFilter struct {
	DocumentID string
	Page       int
	PageSize   int
}

func (s *KnowledgeService) ListDocuments(ctx context.Context, reqCtx RequestContext, input ListDocumentsInput) (DocumentList, error) {
	if err := validateActor(reqCtx); err != nil {
		return DocumentList{}, err
	}
	knowledgeBaseID := strings.TrimSpace(input.KnowledgeBaseID)
	if knowledgeBaseID == "" {
		return DocumentList{}, ValidationError("request validation failed", map[string]string{"knowledgeBaseId": "is required"})
	}
	base, err := s.repo.FindKnowledgeBaseByID(ctx, knowledgeBaseID)
	if err != nil {
		return DocumentList{}, mapKnowledgeBaseRepositoryError(err, "knowledge base not found", "knowledge base metadata access failed")
	}
	if !canAccessKnowledgeBase(reqCtx, base) {
		return DocumentList{}, NotFoundError("knowledge base not found")
	}
	page, pageSize, err := normalizePagination(input.Page, input.PageSize)
	if err != nil {
		return DocumentList{}, err
	}
	status, err := normalizeOptionalDocumentStatus(input.Status)
	if err != nil {
		return DocumentList{}, ValidationError("request validation failed", map[string]string{"status": err.Error()})
	}

	result, err := s.repo.ListDocuments(ctx, DocumentFilter{
		KnowledgeBaseID: knowledgeBaseID,
		OwnerUserID:     ownerFilter(reqCtx),
		Page:            page,
		PageSize:        pageSize,
		Status:          status,
	})
	if err != nil {
		return DocumentList{}, DependencyError("document metadata access failed", err)
	}
	return result, nil
}

func (s *KnowledgeService) GetDocument(ctx context.Context, reqCtx RequestContext, documentID string) (KnowledgeDocument, error) {
	if err := validateActor(reqCtx); err != nil {
		return KnowledgeDocument{}, err
	}
	id := strings.TrimSpace(documentID)
	if id == "" {
		return KnowledgeDocument{}, ValidationError("request validation failed", map[string]string{"documentId": "is required"})
	}
	doc, err := s.repo.FindDocumentByID(ctx, id)
	if err != nil {
		return KnowledgeDocument{}, mapDocumentRepositoryError(err, "document not found", "document metadata access failed")
	}
	if !canAccessKnowledgeDocument(reqCtx, doc) {
		return KnowledgeDocument{}, NotFoundError("document not found")
	}
	return doc, nil
}

func (s *KnowledgeService) ListChunks(ctx context.Context, reqCtx RequestContext, input ListChunksInput) (ChunkList, error) {
	doc, err := s.GetDocument(ctx, reqCtx, input.DocumentID)
	if err != nil {
		return ChunkList{}, err
	}
	page, pageSize, err := normalizePagination(input.Page, input.PageSize)
	if err != nil {
		return ChunkList{}, err
	}
	if pageSize > 500 {
		return ChunkList{}, ValidationError("request validation failed", map[string]string{"pageSize": "must be between 1 and 500"})
	}
	if doc.Status != DocumentStatusReady {
		return ChunkList{}, ConflictError("document chunks are available only when document is ready", nil)
	}
	result, err := s.repo.ListChunks(ctx, ChunkFilter{
		DocumentID: doc.ID,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		return ChunkList{}, DependencyError("document chunks access failed", err)
	}
	return result, nil
}

func normalizeOptionalDocumentStatus(value string) (DocumentStatus, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	status := DocumentStatus(strings.ToLower(strings.TrimSpace(value)))
	switch status {
	case DocumentStatusUploaded, DocumentStatusParsing, DocumentStatusChunking, DocumentStatusEmbedding, DocumentStatusReady, DocumentStatusFailed:
		return status, nil
	default:
		return "", errors.New("is invalid")
	}
}

func canAccessKnowledgeDocument(reqCtx RequestContext, doc KnowledgeDocument) bool {
	userID := strings.TrimSpace(reqCtx.UserID)
	if userID == "" {
		return false
	}
	if hasPermission(reqCtx, "knowledge:read:any") || hasPermission(reqCtx, "knowledge:write:any") {
		return true
	}
	return strings.TrimSpace(doc.CreatedBy) == userID
}

func mapDocumentRepositoryError(err error, notFoundMessage string, dependencyMessage string) error {
	if errors.Is(err, ErrNotFound) {
		return NotFoundError(notFoundMessage)
	}
	if errors.Is(err, ErrConflict) {
		return ConflictError("document state conflict", err)
	}
	return DependencyError(dependencyMessage, err)
}
