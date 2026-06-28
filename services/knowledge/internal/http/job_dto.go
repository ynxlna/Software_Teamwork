package httpapi

import (
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type handoffRequest struct {
	FileID         string   `json:"fileId"`
	Name           string   `json:"name"`
	ContentType    string   `json:"contentType,omitempty"`
	SizeBytes      int64    `json:"sizeBytes,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	CreatedBy      string   `json:"createdBy,omitempty"`
	IdempotencyKey string   `json:"idempotencyKey,omitempty"`
}

type handoffResponse struct {
	DocumentID string                 `json:"documentId"`
	JobID      string                 `json:"jobId"`
	Status     service.DocumentStatus `json:"status"`
}

type processingJobSummary struct {
	ID              string            `json:"id"`
	KnowledgeBaseID string            `json:"knowledgeBaseId"`
	DocumentID      *string           `json:"documentId,omitempty"`
	JobType         service.JobType   `json:"jobType"`
	Status          service.JobStatus `json:"status"`
	CurrentStage    *service.JobStage `json:"currentStage,omitempty"`
	ProgressPercent int               `json:"progressPercent"`
	Message         *string           `json:"message,omitempty"`
	ErrorCode       *string           `json:"errorCode,omitempty"`
	ErrorMessage    *string           `json:"errorMessage,omitempty"`
	Attempts        int               `json:"attempts"`
	MaxAttempts     int               `json:"maxAttempts"`
	StartedAt       *string           `json:"startedAt,omitempty"`
	FinishedAt      *string           `json:"finishedAt,omitempty"`
	CreatedAt       string            `json:"createdAt"`
	UpdatedAt       string            `json:"updatedAt"`
}

func handoffResponseFromDomain(result service.HandoffResult) handoffResponse {
	return handoffResponse{
		DocumentID: result.DocumentID,
		JobID:      result.JobID,
		Status:     result.Status,
	}
}

func processingJobSummaryFromDomain(job service.ProcessingJob) processingJobSummary {
	var startedAt *string
	if job.StartedAt != nil {
		formatted := job.StartedAt.UTC().Format(time.RFC3339)
		startedAt = &formatted
	}
	var finishedAt *string
	if job.FinishedAt != nil {
		formatted := job.FinishedAt.UTC().Format(time.RFC3339)
		finishedAt = &formatted
	}
	return processingJobSummary{
		ID:              job.ID,
		KnowledgeBaseID: job.KnowledgeBaseID,
		DocumentID:      cloneStringPtr(job.DocumentID),
		JobType:         job.JobType,
		Status:          job.Status,
		CurrentStage:    cloneJobStagePtr(job.CurrentStage),
		ProgressPercent: job.ProgressPercent,
		Message:         cloneStringPtr(job.Message),
		ErrorCode:       cloneStringPtr(job.ErrorCode),
		ErrorMessage:    cloneStringPtr(job.ErrorMessage),
		Attempts:        job.Attempts,
		MaxAttempts:     job.MaxAttempts,
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		CreatedAt:       job.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       job.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func cloneJobStagePtr(value *service.JobStage) *service.JobStage {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
