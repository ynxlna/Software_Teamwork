package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestKnowledgeBaseCRUDAndSoftDelete(t *testing.T) {
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	repo := repository.NewMemoryRepository()
	svc := service.NewWithOptions(repo, func() time.Time { return now }, func(prefix string) string {
		return prefix + "_test"
	})
	reqCtx := writeContext("usr_1")

	kb, err := svc.CreateKnowledgeBase(context.Background(), reqCtx, service.CreateKnowledgeBaseInput{
		Name: "规程库",
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}
	if kb.ID != "kb_test" || kb.CreatedBy != "usr_1" || kb.DocType != "GENERAL" {
		t.Fatalf("created knowledge base = %+v", kb)
	}
	if !json.Valid(kb.ChunkStrategy) || !json.Valid(kb.RetrievalStrategy) {
		t.Fatalf("default strategies are not valid JSON")
	}

	newName := "规程库 2026"
	updated, err := svc.UpdateKnowledgeBase(context.Background(), reqCtx, service.UpdateKnowledgeBaseInput{
		ID:   kb.ID,
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("UpdateKnowledgeBase() error = %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("updated name = %q", updated.Name)
	}

	list, err := svc.ListKnowledgeBases(context.Background(), readContext("usr_1"), service.ListKnowledgeBasesInput{})
	if err != nil {
		t.Fatalf("ListKnowledgeBases() error = %v", err)
	}
	if list.Page.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("list = %+v", list)
	}

	if err := svc.DeleteKnowledgeBase(context.Background(), reqCtx, kb.ID); err != nil {
		t.Fatalf("DeleteKnowledgeBase() error = %v", err)
	}
	_, err = svc.GetKnowledgeBase(context.Background(), reqCtx, kb.ID)
	if !hasCode(err, service.CodeNotFound) {
		t.Fatalf("GetKnowledgeBase() after delete error = %v", err)
	}
}

func TestCreateRequiresWritePermission(t *testing.T) {
	svc := service.New(repository.NewMemoryRepository())

	_, err := svc.CreateKnowledgeBase(context.Background(), readContext("usr_1"), service.CreateKnowledgeBaseInput{Name: "规程库"})
	if !hasCode(err, service.CodeForbidden) {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}
}

func TestMissingUserReturnsUnauthorized(t *testing.T) {
	svc := service.New(repository.NewMemoryRepository())

	_, err := svc.ListKnowledgeBases(context.Background(), service.RequestContext{}, service.ListKnowledgeBasesInput{})
	if !hasCode(err, service.CodeUnauthorized) {
		t.Fatalf("ListKnowledgeBases() error = %v", err)
	}
}

func TestOwnerFilteringHidesOtherUsersResources(t *testing.T) {
	repo := repository.NewMemoryRepository()
	svc := service.NewWithOptions(repo, fixedClock(), func(prefix string) string { return prefix + "_owner" })
	kb, err := svc.CreateKnowledgeBase(context.Background(), writeContext("owner"), service.CreateKnowledgeBaseInput{Name: "owner kb"})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}

	_, err = svc.GetKnowledgeBase(context.Background(), readContext("other"), kb.ID)
	if !hasCode(err, service.CodeNotFound) {
		t.Fatalf("GetKnowledgeBase() for other user error = %v", err)
	}

	got, err := svc.GetKnowledgeBase(context.Background(), service.RequestContext{UserID: "reader", Permissions: []string{service.PermissionKnowledgeRead}}, kb.ID)
	if err != nil {
		t.Fatalf("GetKnowledgeBase() with read permission error = %v", err)
	}
	if got.ID != kb.ID {
		t.Fatalf("knowledge base id = %q", got.ID)
	}
}

func TestDocumentListAndDetailExcludeDeletedKnowledgeBase(t *testing.T) {
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	repo := repository.NewMemoryRepository()
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
		ChunkCount:      3,
		Tags:            []string{"锅炉"},
		CreatedBy:       "usr_1",
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	svc := service.NewWithOptions(repo, func() time.Time { return now.Add(time.Hour) }, nil)

	status := service.DocumentStatusReady
	list, err := svc.ListDocuments(context.Background(), readContext("usr_1"), service.ListDocumentsInput{
		KnowledgeBaseID: "kb_1",
		Status:          &status,
	})
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if list.Page.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != "doc_1" {
		t.Fatalf("document list = %+v", list)
	}

	if err := svc.DeleteKnowledgeBase(context.Background(), writeContext("usr_1"), "kb_1"); err != nil {
		t.Fatalf("DeleteKnowledgeBase() error = %v", err)
	}
	_, err = svc.GetDocument(context.Background(), readContext("usr_1"), "doc_1")
	if !hasCode(err, service.CodeNotFound) {
		t.Fatalf("GetDocument() after kb delete error = %v", err)
	}
	_, err = svc.ListDocuments(context.Background(), readContext("usr_1"), service.ListDocumentsInput{KnowledgeBaseID: "kb_1"})
	if !hasCode(err, service.CodeNotFound) {
		t.Fatalf("ListDocuments() after kb delete error = %v", err)
	}
}

func TestInvalidPatchAndPagination(t *testing.T) {
	svc := service.New(repository.NewMemoryRepository())

	blank := " "
	_, err := svc.UpdateKnowledgeBase(context.Background(), writeContext("usr_1"), service.UpdateKnowledgeBaseInput{
		ID:   "kb_1",
		Name: &blank,
	})
	if !hasCode(err, service.CodeValidation) {
		t.Fatalf("UpdateKnowledgeBase() error = %v", err)
	}

	_, err = svc.ListKnowledgeBases(context.Background(), readContext("usr_1"), service.ListKnowledgeBasesInput{
		Page: service.PageInput{Page: 0, PageSize: 201},
	})
	if !hasCode(err, service.CodeValidation) {
		t.Fatalf("ListKnowledgeBases() error = %v", err)
	}
}

func TestInvalidDocumentStatus(t *testing.T) {
	svc := service.New(repository.NewMemoryRepository())
	status := service.DocumentStatus("deleted")

	_, err := svc.ListDocuments(context.Background(), readContext("usr_1"), service.ListDocumentsInput{
		KnowledgeBaseID: "kb_1",
		Status:          &status,
	})
	if !hasCode(err, service.CodeValidation) {
		t.Fatalf("ListDocuments() error = %v", err)
	}
}

func writeContext(userID string) service.RequestContext {
	return service.RequestContext{UserID: userID, Permissions: []string{service.PermissionKnowledgeWrite}}
}

func readContext(userID string) service.RequestContext {
	return service.RequestContext{UserID: userID}
}

func fixedClock() service.Clock {
	return func() time.Time {
		return time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	}
}

func hasCode(err error, code service.Code) bool {
	var appErr *service.AppError
	return errors.As(err, &appErr) && appErr.Code == code
}
