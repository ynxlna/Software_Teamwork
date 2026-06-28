package repository_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestMemoryRepositoryKnowledgeBaseCRUD(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)

	base := service.KnowledgeBase{
		ID:                "kb_1",
		Name:              "Base",
		Description:       "Desc",
		DocType:           "GENERAL",
		ChunkStrategy:     service.ChunkStrategy{"chunkSize": 100},
		RetrievalStrategy: service.RetrievalStrategy{"topK": 5},
		CreatedBy:         "usr_123",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	created, err := repo.CreateKnowledgeBase(context.Background(), base)
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}
	if created.ID != "kb_1" {
		t.Fatalf("created = %+v", created)
	}

	found, err := repo.FindKnowledgeBaseByID(context.Background(), "kb_1")
	if err != nil {
		t.Fatalf("FindKnowledgeBaseByID() error = %v", err)
	}
	if found.Name != "Base" {
		t.Fatalf("found = %+v", found)
	}

	found.Name = "Updated"
	updated, err := repo.UpdateKnowledgeBase(context.Background(), found)
	if err != nil {
		t.Fatalf("UpdateKnowledgeBase() error = %v", err)
	}
	if updated.Name != "Updated" {
		t.Fatalf("updated = %+v", updated)
	}

	list, err := repo.ListKnowledgeBases(context.Background(), service.KnowledgeBaseFilter{
		OwnerUserID: "usr_123",
		Page:        1,
		PageSize:    20,
		Keyword:     "update",
		DocType:     "GENERAL",
	})
	if err != nil {
		t.Fatalf("ListKnowledgeBases() error = %v", err)
	}
	if list.Page.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("list = %+v", list)
	}

	if err := repo.MarkKnowledgeBaseDeleted(context.Background(), "kb_1", now.Add(time.Hour)); err != nil {
		t.Fatalf("MarkKnowledgeBaseDeleted() error = %v", err)
	}
	if _, err := repo.FindKnowledgeBaseByID(context.Background(), "kb_1"); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("FindKnowledgeBaseByID() error = %v, want ErrNotFound", err)
	}
}

func TestMemoryRepositoryRejectsDuplicateKnowledgeBase(t *testing.T) {
	repo := repository.NewMemoryRepository()
	base := service.KnowledgeBase{
		ID:                "kb_1",
		Name:              "Base",
		DocType:           "GENERAL",
		ChunkStrategy:     service.ChunkStrategy{},
		RetrievalStrategy: service.RetrievalStrategy{},
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	if _, err := repo.CreateKnowledgeBase(context.Background(), base); err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}
	if _, err := repo.CreateKnowledgeBase(context.Background(), base); !errors.Is(err, service.ErrConflict) {
		t.Fatalf("duplicate error = %v, want ErrConflict", err)
	}
}

func TestMemoryRepositoryDocumentsAndChunks(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	repo.PutDocumentForTest(service.KnowledgeDocument{
		ID:              "doc_ready",
		KnowledgeBaseID: "kb_1",
		FileID:          "file_1",
		Name:            "Ready",
		Status:          service.DocumentStatusReady,
		CreatedBy:       "usr_123",
		CreatedAt:       now,
	})
	repo.PutDocumentForTest(service.KnowledgeDocument{
		ID:              "doc_failed",
		KnowledgeBaseID: "kb_1",
		FileID:          "file_2",
		Name:            "Failed",
		Status:          service.DocumentStatusFailed,
		CreatedBy:       "usr_123",
		CreatedAt:       now.Add(time.Minute),
	})
	repo.PutChunksForTest("doc_ready", []service.DocumentChunk{
		{ID: "chunk_2", DocumentID: "doc_ready", ChunkIndex: 1, Content: "two", CreatedAt: now},
		{ID: "chunk_1", DocumentID: "doc_ready", ChunkIndex: 0, Content: "one", CreatedAt: now},
	})

	docs, err := repo.ListDocuments(context.Background(), service.DocumentFilter{
		KnowledgeBaseID: "kb_1",
		OwnerUserID:     "usr_123",
		Page:            1,
		PageSize:        20,
		Status:          service.DocumentStatusReady,
	})
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if docs.Page.Total != 1 || len(docs.Items) != 1 || docs.Items[0].ID != "doc_ready" {
		t.Fatalf("docs = %+v", docs)
	}

	found, err := repo.FindDocumentByID(context.Background(), "doc_ready")
	if err != nil {
		t.Fatalf("FindDocumentByID() error = %v", err)
	}
	if found.Name != "Ready" {
		t.Fatalf("found = %+v", found)
	}

	chunks, err := repo.ListChunks(context.Background(), service.ChunkFilter{DocumentID: "doc_ready", Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("ListChunks() error = %v", err)
	}
	if chunks.Page.Total != 2 || chunks.Items[0].ID != "chunk_1" || chunks.Items[1].ID != "chunk_2" {
		t.Fatalf("chunks = %+v", chunks)
	}
}

func TestMemoryRepositoryCreateIngestionJob(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	docID := "doc_1"
	stage := service.JobStageHandoff
	key := "upload-123"
	doc := service.KnowledgeDocument{
		ID:              docID,
		KnowledgeBaseID: "kb_1",
		FileID:          "file_1",
		Name:            "manual.md",
		Status:          service.DocumentStatusUploaded,
		CreatedBy:       "usr_123",
		CreatedAt:       now,
	}
	job := service.ProcessingJob{
		ID:              "job_1",
		KnowledgeBaseID: "kb_1",
		DocumentID:      &docID,
		JobType:         service.JobTypeIngest,
		Status:          service.JobStatusQueued,
		CurrentStage:    &stage,
		IdempotencyKey:  &key,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	createdDoc, createdJob, err := repo.CreateIngestionJob(context.Background(), doc, job)
	if err != nil {
		t.Fatalf("CreateIngestionJob() error = %v", err)
	}
	if createdDoc.ID != doc.ID || createdJob.ID != job.ID {
		t.Fatalf("created doc/job = %+v %+v", createdDoc, createdJob)
	}
	secondDoc, secondJob, err := repo.CreateIngestionJob(context.Background(), doc, job)
	if err != nil {
		t.Fatalf("idempotent CreateIngestionJob() error = %v", err)
	}
	if secondDoc.ID != createdDoc.ID || secondJob.ID != createdJob.ID {
		t.Fatalf("second doc/job = %+v %+v", secondDoc, secondJob)
	}
	foundJob, err := repo.FindJobByID(context.Background(), "job_1")
	if err != nil {
		t.Fatalf("FindJobByID() error = %v", err)
	}
	if foundJob.Status != service.JobStatusQueued {
		t.Fatalf("found job = %+v", foundJob)
	}
}

func TestMemoryRepositoryKnowledgeStatsAggregatesAllVisibleRows(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 250; i++ {
		baseID := "kb_" + strconv.Itoa(i)
		if _, err := repo.CreateKnowledgeBase(context.Background(), service.KnowledgeBase{
			ID:                baseID,
			Name:              "Base " + strconv.Itoa(i),
			DocType:           "GENERAL",
			ChunkStrategy:     service.ChunkStrategy{"type": "SEMANTIC_TEXT"},
			RetrievalStrategy: service.RetrievalStrategy{"mode": "VECTOR"},
			CreatedBy:         "usr_123",
			CreatedAt:         now,
			UpdatedAt:         now,
		}); err != nil {
			t.Fatalf("CreateKnowledgeBase() error = %v", err)
		}
		status := service.DocumentStatusReady
		if i%2 == 0 {
			status = service.DocumentStatusFailed
		}
		docID := "doc_" + strconv.Itoa(i)
		repo.PutDocumentForTest(service.KnowledgeDocument{
			ID:              docID,
			KnowledgeBaseID: baseID,
			FileID:          "file_" + strconv.Itoa(i),
			Name:            "Doc " + strconv.Itoa(i),
			Status:          status,
			CreatedBy:       "usr_123",
			CreatedAt:       now,
		})
		repo.PutChunksForTest(docID, []service.DocumentChunk{{
			ID:              "chunk_" + strconv.Itoa(i),
			KnowledgeBaseID: baseID,
			DocumentID:      docID,
			ChunkIndex:      0,
			Content:         "content",
			CreatedAt:       now,
		}})
	}
	if _, err := repo.CreateKnowledgeBase(context.Background(), service.KnowledgeBase{
		ID:                "kb_other",
		Name:              "Other",
		DocType:           "GENERAL",
		ChunkStrategy:     service.ChunkStrategy{"type": "SEMANTIC_TEXT"},
		RetrievalStrategy: service.RetrievalStrategy{"mode": "VECTOR"},
		CreatedBy:         "usr_other",
		CreatedAt:         now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatalf("CreateKnowledgeBase() other error = %v", err)
	}

	stats, err := repo.GetKnowledgeStats(context.Background(), service.StatsFilter{
		OwnerUserID: "usr_123",
		Since:       now.AddDate(0, 0, -29),
		Until:       now,
	})
	if err != nil {
		t.Fatalf("GetKnowledgeStats() error = %v", err)
	}
	if stats.KnowledgeBaseCount != 250 || stats.DocumentCount != 250 || stats.ChunkCount != 250 {
		t.Fatalf("stats counts = %+v", stats)
	}
	if stats.ReadyDocumentCount != 125 || stats.FailedDocumentCount != 125 {
		t.Fatalf("status counts = %+v", stats)
	}
	if len(stats.RecentUploads) != 1 || stats.RecentUploads[0].Count != 250 {
		t.Fatalf("recent uploads = %+v", stats.RecentUploads)
	}
}
