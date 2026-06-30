package repository_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestMemoryRepositorySoftDeleteKnowledgeBaseHidesDocuments(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	scope := service.AccessScope{UserID: "usr_1", CanWrite: true}

	repo.SeedKnowledgeBase(service.KnowledgeBase{
		ID:                "kb_1",
		Name:              "规程库",
		Description:       "",
		DocType:           "GENERAL",
		ChunkStrategy:     json.RawMessage(`{}`),
		RetrievalStrategy: json.RawMessage(`{}`),
		CreatedBy:         "usr_1",
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	repo.SeedDocument(service.KnowledgeDocument{
		ID:              "doc_1",
		KnowledgeBaseID: "kb_1",
		Name:            "规程.pdf",
		Status:          service.DocumentStatusReady,
		CreatedBy:       "usr_1",
		CreatedAt:       now,
		UpdatedAt:       now,
	})

	if err := repo.SoftDeleteKnowledgeBase(context.Background(), "kb_1", now.Add(time.Hour), scope); err != nil {
		t.Fatalf("SoftDeleteKnowledgeBase() error = %v", err)
	}
	if _, err := repo.GetKnowledgeBase(context.Background(), "kb_1", scope); err != service.ErrNotFound {
		t.Fatalf("GetKnowledgeBase() error = %v", err)
	}
	if _, err := repo.GetDocument(context.Background(), "doc_1", scope); err != service.ErrNotFound {
		t.Fatalf("GetDocument() error = %v", err)
	}
}

func TestMemoryRepositoryCreateDocumentWithJobAndMarkFailed(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 29, 14, 0, 0, 0, time.UTC)
	scope := service.AccessScope{UserID: "usr_1", CanWrite: true}
	repo.SeedKnowledgeBase(service.KnowledgeBase{
		ID:                "kb_1",
		Name:              "规程库",
		Description:       "",
		DocType:           "GENERAL",
		ChunkStrategy:     json.RawMessage(`{}`),
		RetrievalStrategy: json.RawMessage(`{}`),
		CreatedBy:         "usr_1",
		CreatedAt:         now,
		UpdatedAt:         now,
	})

	doc, job, err := repo.CreateDocumentWithJob(context.Background(), service.CreateDocumentWithJobRecord{
		DocumentID:      "doc_1",
		KnowledgeBaseID: "kb_1",
		FileRef:         "file_1",
		Name:            "规程.pdf",
		ContentType:     "application/pdf",
		SizeBytes:       9,
		Status:          service.DocumentStatusUploaded,
		Tags:            []string{"锅炉"},
		CurrentJobID:    "job_1",
		CreatedBy:       "usr_1",
		JobID:           "job_1",
		JobType:         service.JobTypeDocumentIngestion,
		JobStatus:       service.JobStatusQueued,
		JobStage:        "uploaded",
		JobMessage:      "document uploaded and queued for ingestion",
		MaxAttempts:     3,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, scope)
	if err != nil {
		t.Fatalf("CreateDocumentWithJob() error = %v", err)
	}
	if doc.ID != "doc_1" || doc.CurrentJobID == nil || *doc.CurrentJobID != "job_1" {
		t.Fatalf("document = %+v", doc)
	}
	if job.ID != "job_1" || job.DocumentID == nil || *job.DocumentID != "doc_1" || job.Status != service.JobStatusQueued {
		t.Fatalf("job = %+v", job)
	}

	if err := repo.MarkDocumentJobFailed(context.Background(), "doc_1", "job_1", nil, "dependency_error", "queue failed", now.Add(time.Minute)); err != nil {
		t.Fatalf("MarkDocumentJobFailed() error = %v", err)
	}
	failedDoc, err := repo.GetDocument(context.Background(), "doc_1", scope)
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	if failedDoc.Status != service.DocumentStatusFailed || failedDoc.ErrorCode == nil || *failedDoc.ErrorCode != "dependency_error" {
		t.Fatalf("failed document = %+v", failedDoc)
	}
}

func TestMemoryRepositoryMarkFailedKeepsJobTerminalWhenDocumentWasDeleted(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 29, 14, 0, 0, 0, time.UTC)
	scope := service.AccessScope{UserID: "usr_1", CanWrite: true}
	repo.SeedKnowledgeBase(service.KnowledgeBase{
		ID:                "kb_1",
		Name:              "规程库",
		Description:       "",
		DocType:           "GENERAL",
		ChunkStrategy:     json.RawMessage(`{}`),
		RetrievalStrategy: json.RawMessage(`{}`),
		CreatedBy:         "usr_1",
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	_, job, err := repo.CreateDocumentWithJob(context.Background(), service.CreateDocumentWithJobRecord{
		DocumentID:      "doc_1",
		KnowledgeBaseID: "kb_1",
		FileRef:         "file_1",
		Name:            "规程.pdf",
		ContentType:     "application/pdf",
		SizeBytes:       9,
		Status:          service.DocumentStatusUploaded,
		CurrentJobID:    "job_1",
		CreatedBy:       "usr_1",
		JobID:           "job_1",
		JobType:         service.JobTypeDocumentIngestion,
		JobStatus:       service.JobStatusRunning,
		JobStage:        "parsing",
		JobMessage:      "document ingestion in progress",
		MaxAttempts:     3,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, scope)
	if err != nil {
		t.Fatalf("CreateDocumentWithJob() error = %v", err)
	}
	if job.Status != service.JobStatusRunning {
		t.Fatalf("job status = %s, want running", job.Status)
	}
	if err := repo.SoftDeleteKnowledgeBase(context.Background(), "kb_1", now.Add(time.Minute), scope); err != nil {
		t.Fatalf("SoftDeleteKnowledgeBase() error = %v", err)
	}

	if err := repo.MarkDocumentJobFailed(context.Background(), "doc_1", "job_1", nil, "dependency_error", "source content read failed", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("MarkDocumentJobFailed() error = %v", err)
	}
	failedJob, err := repo.GetProcessingJob(context.Background(), "job_1")
	if err != nil {
		t.Fatalf("GetProcessingJob() error = %v", err)
	}
	if failedJob.Status != service.JobStatusFailed || failedJob.ErrorCode == nil || *failedJob.ErrorCode != "dependency_error" {
		t.Fatalf("failed job = %+v", failedJob)
	}
}
