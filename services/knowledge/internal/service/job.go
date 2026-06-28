package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type JobType string
type JobStatus string
type JobStage string

const (
	JobTypeIngest        JobType = "ingest"
	JobTypeReprocess     JobType = "reprocess"
	JobTypeDeleteCleanup JobType = "delete_cleanup"

	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"

	JobStageHandoff JobStage = "handoff"
)

type ProcessingJob struct {
	ID              string
	KnowledgeBaseID string
	DocumentID      *string
	JobType         JobType
	Status          JobStatus
	CurrentStage    *JobStage
	ProgressPercent int
	Message         *string
	ErrorCode       *string
	ErrorMessage    *string
	Attempts        int
	MaxAttempts     int
	IdempotencyKey  *string
	StartedAt       *time.Time
	FinishedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type HandoffInput struct {
	KnowledgeBaseID string
	FileID          string
	Name            string
	ContentType     string
	SizeBytes       int64
	Tags            []string
	CreatedBy       string
	IdempotencyKey  string
}

type HandoffResult struct {
	DocumentID string
	JobID      string
	Status     DocumentStatus
}

type JobRepository interface {
	CreateIngestionJob(ctx context.Context, doc KnowledgeDocument, job ProcessingJob) (KnowledgeDocument, ProcessingJob, error)
	CreateProcessingJob(ctx context.Context, job ProcessingJob) (ProcessingJob, error)
	FindJobByID(ctx context.Context, id string) (ProcessingJob, error)
	UpdateJobState(ctx context.Context, id string, update JobStateUpdate) (ProcessingJob, error)
}

func (s *KnowledgeService) CreateIngestionJob(ctx context.Context, reqCtx RequestContext, input HandoffInput) (HandoffResult, error) {
	if err := validateActor(reqCtx); err != nil {
		return HandoffResult{}, err
	}
	knowledgeBaseID := strings.TrimSpace(input.KnowledgeBaseID)
	if knowledgeBaseID == "" {
		return HandoffResult{}, ValidationError("request validation failed", map[string]string{"knowledgeBaseId": "is required"})
	}
	base, err := s.repo.FindKnowledgeBaseByID(ctx, knowledgeBaseID)
	if err != nil {
		return HandoffResult{}, mapKnowledgeBaseRepositoryError(err, "knowledge base not found", "knowledge base metadata access failed")
	}
	if !canAccessKnowledgeBase(reqCtx, base) {
		return HandoffResult{}, NotFoundError("knowledge base not found")
	}

	fields := map[string]string{}
	fileID := strings.TrimSpace(input.FileID)
	if fileID == "" {
		fields["fileId"] = "is required"
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		fields["name"] = "is required"
	} else if len(name) > 255 {
		fields["name"] = "must be at most 255 characters"
	}
	if strings.ContainsAny(name, "\x00\r\n") {
		fields["name"] = "is invalid"
	}
	if input.SizeBytes < 0 {
		fields["sizeBytes"] = "must be non-negative"
	}
	tags, err := NormalizeTags(input.Tags)
	if err != nil {
		fields["tags"] = err.Error()
	}
	if len(fields) > 0 {
		return HandoffResult{}, ValidationError("request validation failed", fields)
	}

	docID, err := s.newID("doc")
	if err != nil {
		return HandoffResult{}, DependencyError("document id generation failed", err)
	}
	jobID, err := s.newID("job")
	if err != nil {
		return HandoffResult{}, DependencyError("job id generation failed", err)
	}

	now := s.now().UTC()
	createdBy := strings.TrimSpace(input.CreatedBy)
	if createdBy == "" {
		createdBy = strings.TrimSpace(reqCtx.UserID)
	}
	var contentType *string
	if strings.TrimSpace(input.ContentType) != "" {
		trimmed := strings.TrimSpace(input.ContentType)
		contentType = &trimmed
	}
	stage := JobStageHandoff
	documentID := docID
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	var idempotency *string
	if idempotencyKey != "" {
		idempotency = &idempotencyKey
	}

	doc := KnowledgeDocument{
		ID:              docID,
		KnowledgeBaseID: knowledgeBaseID,
		FileID:          fileID,
		Name:            name,
		ContentType:     contentType,
		SizeBytes:       input.SizeBytes,
		Status:          DocumentStatusUploaded,
		ChunkCount:      0,
		Tags:            tags,
		CreatedBy:       createdBy,
		CreatedAt:       now,
		UpdatedAt:       &now,
		CurrentJobID:    &jobID,
	}
	job := ProcessingJob{
		ID:              jobID,
		KnowledgeBaseID: knowledgeBaseID,
		DocumentID:      &documentID,
		JobType:         JobTypeIngest,
		Status:          JobStatusQueued,
		CurrentStage:    &stage,
		ProgressPercent: 0,
		Attempts:        0,
		MaxAttempts:     3,
		IdempotencyKey:  idempotency,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	createdDoc, createdJob, err := s.repo.CreateIngestionJob(ctx, doc, job)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return HandoffResult{}, ConflictError("ingestion job already exists", err)
		}
		return HandoffResult{}, DependencyError("ingestion job write failed", err)
	}

	return HandoffResult{
		DocumentID: createdDoc.ID,
		JobID:      createdJob.ID,
		Status:     createdDoc.Status,
	}, nil
}

func (s *KnowledgeService) GetJob(ctx context.Context, reqCtx RequestContext, jobID string) (ProcessingJob, error) {
	if err := validateActor(reqCtx); err != nil {
		return ProcessingJob{}, err
	}
	id := strings.TrimSpace(jobID)
	if id == "" {
		return ProcessingJob{}, ValidationError("request validation failed", map[string]string{"jobId": "is required"})
	}
	job, err := s.repo.FindJobByID(ctx, id)
	if err != nil {
		return ProcessingJob{}, mapJobRepositoryError(err, "job not found", "job metadata access failed")
	}
	if job.DocumentID != nil {
		doc, err := s.repo.FindDocumentByID(ctx, *job.DocumentID)
		if err != nil {
			return ProcessingJob{}, mapDocumentRepositoryError(err, "job not found", "job metadata access failed")
		}
		if !canAccessKnowledgeDocument(reqCtx, doc) {
			return ProcessingJob{}, NotFoundError("job not found")
		}
	}
	return job, nil
}

func mapJobRepositoryError(err error, notFoundMessage string, dependencyMessage string) error {
	if errors.Is(err, ErrNotFound) {
		return NotFoundError(notFoundMessage)
	}
	if errors.Is(err, ErrConflict) {
		return ConflictError("job state conflict", err)
	}
	return DependencyError(dependencyMessage, err)
}

const (
	maxTags      = 32
	maxTagLength = 64
)

func NormalizeTags(tags []string) ([]string, error) {
	if len(tags) > maxTags {
		return nil, fmt.Errorf("must contain at most %d tags", maxTags)
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(tags))
	for _, raw := range tags {
		if strings.ContainsAny(raw, "\x00\r\n") {
			return nil, fmt.Errorf("must not contain control characters")
		}
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if len(tag) > maxTagLength {
			return nil, fmt.Errorf("each tag must be at most %d characters", maxTagLength)
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	return normalized, nil
}
