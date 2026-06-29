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
