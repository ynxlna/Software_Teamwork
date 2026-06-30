package repository_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestPostgresRepositoryDocumentUploadLifecycle(t *testing.T) {
	repo, pool, cleanup := newPostgresRepositoryForTest(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2026, 6, 29, 14, 0, 0, 0, time.UTC)
	scope := service.AccessScope{UserID: "usr_1", CanWrite: true}

	kb, err := repo.CreateKnowledgeBase(ctx, service.CreateKnowledgeBaseRecord{
		ID:                "kb_1",
		Name:              "规程库",
		Description:       "",
		DocType:           "GENERAL",
		ChunkStrategy:     json.RawMessage(`{"type":"fixed"}`),
		RetrievalStrategy: json.RawMessage(`{"mode":"vector"}`),
		CreatedBy:         "usr_1",
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}
	if kb.ID != "kb_1" || kb.CreatedBy != "usr_1" {
		t.Fatalf("knowledge base = %+v", kb)
	}

	doc, job, err := repo.CreateDocumentWithJob(ctx, service.CreateDocumentWithJobRecord{
		DocumentID:      "doc_1",
		KnowledgeBaseID: kb.ID,
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
	if doc.CurrentJobID == nil || *doc.CurrentJobID != job.ID {
		t.Fatalf("document/job link = %+v / %+v", doc, job)
	}
	if job.DocumentID == nil || *job.DocumentID != doc.ID || job.Status != service.JobStatusQueued {
		t.Fatalf("job = %+v", job)
	}

	list, err := repo.ListDocumentsByKnowledgeBase(ctx, kb.ID, nil, scope, service.PageInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListDocumentsByKnowledgeBase() error = %v", err)
	}
	if list.Page.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != doc.ID {
		t.Fatalf("document list = %+v", list)
	}

	failedAt := now.Add(time.Minute)
	if err := repo.MarkDocumentJobFailed(ctx, doc.ID, job.ID, nil, "dependency_error", "queue failed", failedAt); err != nil {
		t.Fatalf("MarkDocumentJobFailed() error = %v", err)
	}
	failedDoc, err := repo.GetDocument(ctx, doc.ID, scope)
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	if failedDoc.Status != service.DocumentStatusFailed ||
		failedDoc.ErrorCode == nil || *failedDoc.ErrorCode != "dependency_error" ||
		failedDoc.ErrorMessage == nil || *failedDoc.ErrorMessage != "queue failed" {
		t.Fatalf("failed document = %+v", failedDoc)
	}

	var jobStatus, jobErrorCode, jobErrorMessage string
	var jobFinishedAt, jobUpdatedAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT status, COALESCE(error_code, ''), COALESCE(error_message, ''), finished_at, updated_at
		FROM processing_jobs
		WHERE id = $1
	`, job.ID).Scan(&jobStatus, &jobErrorCode, &jobErrorMessage, &jobFinishedAt, &jobUpdatedAt); err != nil {
		t.Fatalf("query failed processing job: %v", err)
	}
	if jobStatus != string(service.JobStatusFailed) ||
		jobErrorCode != "dependency_error" ||
		jobErrorMessage != "queue failed" ||
		!jobFinishedAt.Equal(failedAt) ||
		!jobUpdatedAt.Equal(failedAt) {
		t.Fatalf("failed job status = %q errorCode = %q errorMessage = %q finishedAt = %s updatedAt = %s",
			jobStatus, jobErrorCode, jobErrorMessage, jobFinishedAt, jobUpdatedAt)
	}
}

func TestPostgresRepositoryDocumentLifecycleUpdateAndDelete(t *testing.T) {
	repo, pool, cleanup := newPostgresRepositoryForTest(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC)
	ownerScope := service.AccessScope{UserID: "usr_owner", CanWrite: true}
	otherScope := service.AccessScope{UserID: "usr_other", CanWrite: true}

	kb, err := repo.CreateKnowledgeBase(ctx, service.CreateKnowledgeBaseRecord{
		ID:                "kb_lifecycle",
		Name:              "生命周期库",
		Description:       "",
		DocType:           "GENERAL",
		ChunkStrategy:     json.RawMessage(`{"type":"fixed"}`),
		RetrievalStrategy: json.RawMessage(`{"mode":"vector"}`),
		CreatedBy:         "usr_owner",
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}

	doc, _, err := repo.CreateDocumentWithJob(ctx, service.CreateDocumentWithJobRecord{
		DocumentID:      "doc_lifecycle",
		KnowledgeBaseID: kb.ID,
		FileRef:         "file_lifecycle",
		Name:            "生命周期.pdf",
		ContentType:     "application/pdf",
		SizeBytes:       32,
		Status:          service.DocumentStatusReady,
		Tags:            []string{"old"},
		CurrentJobID:    "job_ingest_lifecycle",
		CreatedBy:       "usr_owner",
		JobID:           "job_ingest_lifecycle",
		JobType:         service.JobTypeDocumentIngestion,
		JobStatus:       service.JobStatusSucceeded,
		JobStage:        "ready",
		JobMessage:      "document ready",
		MaxAttempts:     3,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, ownerScope)
	if err != nil {
		t.Fatalf("CreateDocumentWithJob() error = %v", err)
	}

	if _, err := repo.UpdateDocument(ctx, service.UpdateDocumentRecord{
		ID:        doc.ID,
		Tags:      []string{"new", "reviewed"},
		UpdatedAt: now.Add(time.Minute),
	}, otherScope); err == nil {
		t.Fatal("UpdateDocument() by unrelated user succeeded, want not found")
	}

	updated, err := repo.UpdateDocument(ctx, service.UpdateDocumentRecord{
		ID:        doc.ID,
		Tags:      []string{"new", "reviewed"},
		UpdatedAt: now.Add(2 * time.Minute),
	}, ownerScope)
	if err != nil {
		t.Fatalf("UpdateDocument() error = %v", err)
	}
	if got, want := strings.Join(updated.Tags, ","), "new,reviewed"; got != want {
		t.Fatalf("updated tags = %q, want %q", got, want)
	}

	deleteAt := now.Add(3 * time.Minute)
	if err := repo.SoftDeleteDocument(ctx, service.DeleteDocumentRecord{
		DocumentID:  doc.ID,
		JobID:       "job_delete_cleanup_lifecycle",
		JobType:     service.JobTypeDeleteCleanup,
		JobStatus:   service.JobStatusQueued,
		JobStage:    "delete_cleanup",
		JobMessage:  "document queued for delete cleanup",
		MaxAttempts: 1,
		DeletedAt:   deleteAt,
		CreatedAt:   deleteAt,
		UpdatedAt:   deleteAt,
	}, otherScope); err == nil {
		t.Fatal("SoftDeleteDocument() by unrelated user succeeded, want not found")
	}

	if err := repo.SoftDeleteDocument(ctx, service.DeleteDocumentRecord{
		DocumentID:  doc.ID,
		JobID:       "job_delete_cleanup_lifecycle",
		JobType:     service.JobTypeDeleteCleanup,
		JobStatus:   service.JobStatusQueued,
		JobStage:    "delete_cleanup",
		JobMessage:  "document queued for delete cleanup",
		MaxAttempts: 1,
		DeletedAt:   deleteAt,
		CreatedAt:   deleteAt,
		UpdatedAt:   deleteAt,
	}, ownerScope); err != nil {
		t.Fatalf("SoftDeleteDocument() error = %v", err)
	}

	if _, err := repo.GetDocument(ctx, doc.ID, ownerScope); err == nil {
		t.Fatal("GetDocument() after delete succeeded, want not found")
	}

	list, err := repo.ListDocumentsByKnowledgeBase(ctx, kb.ID, nil, ownerScope, service.PageInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListDocumentsByKnowledgeBase() after delete error = %v", err)
	}
	if list.Page.Total != 0 || len(list.Items) != 0 {
		t.Fatalf("deleted document still visible in list: %+v", list)
	}

	var jobType, jobStatus, jobStage, jobMessage string
	var maxAttempts int32
	var deletedAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT j.job_type, j.status, COALESCE(j.current_stage, ''), COALESCE(j.message, ''), j.max_attempts, d.deleted_at
		FROM processing_jobs j
		JOIN knowledge_documents d ON d.id = j.document_id
		WHERE j.id = $1
	`, "job_delete_cleanup_lifecycle").Scan(&jobType, &jobStatus, &jobStage, &jobMessage, &maxAttempts, &deletedAt); err != nil {
		t.Fatalf("query delete cleanup job: %v", err)
	}
	if jobType != service.JobTypeDeleteCleanup ||
		jobStatus != service.JobStatusQueued ||
		jobStage != "delete_cleanup" ||
		jobMessage != "document queued for delete cleanup" ||
		maxAttempts != 1 ||
		!deletedAt.Equal(deleteAt) {
		t.Fatalf("cleanup job = type:%q status:%q stage:%q message:%q maxAttempts:%d deletedAt:%s",
			jobType, jobStatus, jobStage, jobMessage, maxAttempts, deletedAt)
	}
}

func newPostgresRepositoryForTest(t *testing.T) (*repository.PostgresRepository, *pgxpool.Pool, func()) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("KNOWLEDGE_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set KNOWLEDGE_TEST_DATABASE_URL to run Postgres repository integration tests")
	}

	ctx := context.Background()
	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}

	schema := fmt.Sprintf("knowledge_test_%d", time.Now().UnixNano())
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		adminPool.Close()
		t.Fatalf("create test schema: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE")
		adminPool.Close()
		t.Fatalf("parse test database url: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE")
		adminPool.Close()
		t.Fatalf("connect isolated test schema: %v", err)
	}

	applyKnowledgeMigration(t, ctx, pool)
	cleanup := func() {
		pool.Close()
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE")
		adminPool.Close()
	}
	return repository.NewPostgresRepository(pool), pool, cleanup
}

func applyKnowledgeMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	for _, migration := range []string{
		"../../migrations/0001_create_knowledge_core_tables.sql",
		"../../migrations/0002_create_parser_configs.sql",
	} {
		contents, err := os.ReadFile(migration)
		if err != nil {
			t.Fatalf("read knowledge migration %s: %v", migration, err)
		}
		upSQL, _, _ := strings.Cut(string(contents), "-- +goose Down")
		upSQL = strings.ReplaceAll(upSQL, "-- +goose Up", "")

		for _, statement := range strings.Split(upSQL, ";") {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if _, err := pool.Exec(ctx, statement); err != nil {
				t.Fatalf("apply migration %s statement %q: %v", migration, statement, err)
			}
		}
	}
}
