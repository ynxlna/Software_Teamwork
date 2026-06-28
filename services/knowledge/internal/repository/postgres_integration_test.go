package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestPostgresRepositoryMigrationAndStatsIntegration(t *testing.T) {
	databaseURL := os.Getenv("KNOWLEDGE_POSTGRES_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set KNOWLEDGE_POSTGRES_TEST_DATABASE_URL to run PostgreSQL integration test")
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer db.Close()

	schema := fmt.Sprintf("knowledge_test_%d", time.Now().UnixNano())
	quotedSchema := quoteTestIdentifier(schema)
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		t.Fatalf("create schema error = %v", err)
	}
	defer db.ExecContext(context.Background(), "DROP SCHEMA "+quotedSchema+" CASCADE")
	if _, err := db.ExecContext(ctx, "SET search_path TO "+quotedSchema); err != nil {
		t.Fatalf("set search_path error = %v", err)
	}

	migration, err := os.ReadFile("../../migrations/0001_create_knowledge_core_tables.sql")
	if err != nil {
		t.Fatalf("read migration error = %v", err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatalf("apply migration error = %v", err)
	}

	repo := repository.NewPostgresRepository(db)
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	base, err := repo.CreateKnowledgeBase(ctx, service.KnowledgeBase{
		ID:                "kb_pg",
		Name:              "Postgres KB",
		DocType:           "GENERAL",
		ChunkStrategy:     service.ChunkStrategy{"type": "SEMANTIC_TEXT"},
		RetrievalStrategy: service.RetrievalStrategy{"mode": "VECTOR"},
		CreatedBy:         "usr_123",
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}
	if base.ID != "kb_pg" {
		t.Fatalf("base = %+v", base)
	}

	for i := 0; i < 205; i++ {
		docID := fmt.Sprintf("doc_pg_%d", i)
		jobID := fmt.Sprintf("job_pg_%d", i)
		status := service.DocumentStatusReady
		if i%2 == 0 {
			status = service.DocumentStatusFailed
		}
		doc := service.KnowledgeDocument{
			ID:              docID,
			KnowledgeBaseID: "kb_pg",
			FileID:          fmt.Sprintf("file_pg_%d", i),
			Name:            fmt.Sprintf("Doc %d", i),
			Status:          service.DocumentStatusUploaded,
			CreatedBy:       "usr_123",
			CreatedAt:       now,
			UpdatedAt:       &now,
		}
		stage := service.JobStageHandoff
		job := service.ProcessingJob{
			ID:              jobID,
			KnowledgeBaseID: "kb_pg",
			DocumentID:      &docID,
			JobType:         service.JobTypeIngest,
			Status:          service.JobStatusQueued,
			CurrentStage:    &stage,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if _, _, err := repo.CreateIngestionJob(ctx, doc, job); err != nil {
			t.Fatalf("CreateIngestionJob(%d) error = %v", i, err)
		}
		if _, err := repo.UpdateDocumentProcessingState(ctx, docID, service.DocumentStateUpdate{
			Status:    status,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("UpdateDocumentProcessingState(%d) error = %v", i, err)
		}
		if err := repo.ReplaceDocumentChunks(ctx, docID, []service.DocumentChunk{{
			ID:              fmt.Sprintf("chunk_pg_%d", i),
			KnowledgeBaseID: "kb_pg",
			DocumentID:      docID,
			ChunkIndex:      0,
			Content:         "content",
			TokenCount:      1,
			Metadata:        map[string]any{},
			CreatedAt:       now,
		}}); err != nil {
			t.Fatalf("ReplaceDocumentChunks(%d) error = %v", i, err)
		}
	}

	stats, err := repo.GetKnowledgeStats(ctx, service.StatsFilter{
		OwnerUserID: "usr_123",
		Since:       now.AddDate(0, 0, -29),
		Until:       now,
	})
	if err != nil {
		t.Fatalf("GetKnowledgeStats() error = %v", err)
	}
	if stats.KnowledgeBaseCount != 1 || stats.DocumentCount != 205 || stats.ChunkCount != 205 {
		t.Fatalf("stats counts = %+v", stats)
	}
	if stats.ReadyDocumentCount != 102 || stats.FailedDocumentCount != 103 {
		t.Fatalf("status counts = %+v", stats)
	}
	if len(stats.RecentUploads) != 1 || stats.RecentUploads[0].Count != 205 {
		t.Fatalf("recent uploads = %+v", stats.RecentUploads)
	}
}

func quoteTestIdentifier(identifier string) string {
	if identifier == "" {
		panic("empty test identifier")
	}
	for _, r := range identifier {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		panic("invalid test identifier: " + identifier)
	}
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
