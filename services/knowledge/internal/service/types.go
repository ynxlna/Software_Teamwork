package service

import (
	"context"
	"encoding/json"
	"time"
)

const (
	PermissionKnowledgeRead  = "knowledge:read"
	PermissionKnowledgeWrite = "knowledge:write"
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

type RequestContext struct {
	RequestID      string
	UserID         string
	Roles          []string
	Permissions    []string
	ForwardedFor   string
	ForwardedProto string
}

type AccessScope struct {
	UserID     string
	CanReadAll bool
	CanWrite   bool
}

type Page struct {
	Page     int
	PageSize int
	Total    int64
}

type PageInput struct {
	Page     int
	PageSize int
}

type KnowledgeBase struct {
	ID                string
	Name              string
	Description       string
	DocType           string
	ChunkStrategy     json.RawMessage
	RetrievalStrategy json.RawMessage
	DocumentCount     int64
	ChunkCount        int64
	CreatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

type KnowledgeDocument struct {
	ID              string
	KnowledgeBaseID string
	FileRef         *string
	Name            string
	ContentType     *string
	SizeBytes       *int64
	Status          DocumentStatus
	ErrorCode       *string
	ErrorMessage    *string
	ChunkCount      int64
	Tags            []string
	ParserBackend   *string
	CurrentJobID    *string
	CreatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

type ProcessingJob struct {
	ID              string
	KnowledgeBaseID string
	DocumentID      *string
	JobType         string
	Status          string
	CurrentStage    *string
	ProgressPercent int32
	Message         *string
	ErrorCode       *string
	ErrorMessage    *string
	Attempts        int32
	MaxAttempts     int32
	StartedAt       *time.Time
	FinishedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type KnowledgeBaseList struct {
	Items []KnowledgeBase
	Page  Page
}

type DocumentList struct {
	Items []KnowledgeDocument
	Page  Page
}

type CreateKnowledgeBaseInput struct {
	ID                string
	Name              string
	Description       *string
	DocType           *string
	ChunkStrategy     json.RawMessage
	RetrievalStrategy json.RawMessage
}

type UpdateKnowledgeBaseInput struct {
	ID                string
	Name              *string
	Description       *string
	DocType           *string
	ChunkStrategy     *json.RawMessage
	RetrievalStrategy *json.RawMessage
}

type ListKnowledgeBasesInput struct {
	Page PageInput
}

type ListDocumentsInput struct {
	KnowledgeBaseID string
	Status          *DocumentStatus
	Page            PageInput
}

type Repository interface {
	CreateKnowledgeBase(ctx context.Context, input CreateKnowledgeBaseRecord) (KnowledgeBase, error)
	ListKnowledgeBases(ctx context.Context, scope AccessScope, page PageInput) (KnowledgeBaseList, error)
	GetKnowledgeBase(ctx context.Context, id string, scope AccessScope) (KnowledgeBase, error)
	UpdateKnowledgeBase(ctx context.Context, input UpdateKnowledgeBaseRecord, scope AccessScope) (KnowledgeBase, error)
	SoftDeleteKnowledgeBase(ctx context.Context, id string, deletedAt time.Time, scope AccessScope) error
	ListDocumentsByKnowledgeBase(ctx context.Context, knowledgeBaseID string, status *DocumentStatus, scope AccessScope, page PageInput) (DocumentList, error)
	GetDocument(ctx context.Context, id string, scope AccessScope) (KnowledgeDocument, error)
}

type CreateKnowledgeBaseRecord struct {
	ID                string
	Name              string
	Description       string
	DocType           string
	ChunkStrategy     json.RawMessage
	RetrievalStrategy json.RawMessage
	CreatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type UpdateKnowledgeBaseRecord struct {
	ID                string
	Name              *string
	Description       *string
	DocType           *string
	ChunkStrategy     *json.RawMessage
	RetrievalStrategy *json.RawMessage
	UpdatedAt         time.Time
}
