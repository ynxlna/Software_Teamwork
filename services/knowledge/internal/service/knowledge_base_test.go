package service_test

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	vectorplatform "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/vector"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestCreateKnowledgeBaseDefaultsAndNormalizes(t *testing.T) {
	knowledge := newKnowledgeService(t)

	base, err := knowledge.CreateKnowledgeBase(context.Background(), actorContext(), service.CreateKnowledgeBaseInput{
		Name: "  通用文档  ",
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}
	if base.ID != "kb_1" {
		t.Fatalf("id = %q", base.ID)
	}
	if base.Name != "通用文档" || base.DocType != service.DefaultDocType {
		t.Fatalf("base = %+v", base)
	}
	if base.ChunkStrategy["type"] != service.DefaultChunkStrategyType {
		t.Fatalf("chunk strategy = %+v", base.ChunkStrategy)
	}
	if base.RetrievalStrategy["topK"] != service.DefaultRetrievalTopK {
		t.Fatalf("retrieval strategy = %+v", base.RetrievalStrategy)
	}
	if base.CreatedBy != "usr_123" {
		t.Fatalf("createdBy = %q", base.CreatedBy)
	}
}

func TestListKnowledgeBasesFiltersByOwnerKeywordAndDocType(t *testing.T) {
	repo := repository.NewMemoryRepository()
	knowledge := service.NewKnowledgeService(repo, service.WithIDGenerator(sequenceIDs()))

	if _, err := knowledge.CreateKnowledgeBase(context.Background(), actorContext(), service.CreateKnowledgeBaseInput{Name: "汽机规程", DocType: "procedure"}); err != nil {
		t.Fatalf("create owner base error = %v", err)
	}
	if _, err := knowledge.CreateKnowledgeBase(context.Background(), service.RequestContext{UserID: "usr_other"}, service.CreateKnowledgeBaseInput{Name: "汽机报告", DocType: "report"}); err != nil {
		t.Fatalf("create other base error = %v", err)
	}

	list, err := knowledge.ListKnowledgeBases(context.Background(), actorContext(), service.ListKnowledgeBasesInput{
		Page:     1,
		PageSize: 20,
		Keyword:  "规程",
		DocType:  "PROCEDURE",
	})
	if err != nil {
		t.Fatalf("ListKnowledgeBases() error = %v", err)
	}
	if list.Page.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("list = %+v", list)
	}
	if list.Items[0].Name != "汽机规程" {
		t.Fatalf("item = %+v", list.Items[0])
	}

	adminList, err := knowledge.ListKnowledgeBases(context.Background(), service.RequestContext{UserID: "usr_admin", Permissions: []string{"knowledge:read:any"}}, service.ListKnowledgeBasesInput{})
	if err != nil {
		t.Fatalf("admin ListKnowledgeBases() error = %v", err)
	}
	if adminList.Page.Total != 2 {
		t.Fatalf("admin total = %d", adminList.Page.Total)
	}
}

func TestUpdateAndDeleteKnowledgeBase(t *testing.T) {
	knowledge := newKnowledgeService(t)
	base, err := knowledge.CreateKnowledgeBase(context.Background(), actorContext(), service.CreateKnowledgeBaseInput{Name: "Old"})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}

	newName := "New"
	newDocType := "report"
	updated, err := knowledge.UpdateKnowledgeBase(context.Background(), actorContext(), service.UpdateKnowledgeBaseInput{
		ID:      base.ID,
		Name:    &newName,
		DocType: &newDocType,
	})
	if err != nil {
		t.Fatalf("UpdateKnowledgeBase() error = %v", err)
	}
	if updated.Name != "New" || updated.DocType != "REPORT" {
		t.Fatalf("updated = %+v", updated)
	}

	if err := knowledge.DeleteKnowledgeBase(context.Background(), actorContext(), base.ID); err != nil {
		t.Fatalf("DeleteKnowledgeBase() error = %v", err)
	}
	if _, err := knowledge.GetKnowledgeBase(context.Background(), actorContext(), base.ID); !hasCode(err, service.CodeNotFound) {
		t.Fatalf("GetKnowledgeBase() error = %v, want not_found", err)
	}
}

func TestKnowledgeBaseValidation(t *testing.T) {
	knowledge := newKnowledgeService(t)
	_, err := knowledge.CreateKnowledgeBase(context.Background(), actorContext(), service.CreateKnowledgeBaseInput{
		Name:              "",
		DocType:           "bad type",
		ChunkStrategy:     service.ChunkStrategy{"chunkSize": 0},
		RetrievalStrategy: service.RetrievalStrategy{"topK": 101},
	})
	var appErr *service.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want AppError", err)
	}
	if appErr.Code != service.CodeValidation {
		t.Fatalf("code = %q", appErr.Code)
	}
	if appErr.Fields["name"] == "" || appErr.Fields["docType"] == "" || appErr.Fields["chunkStrategy"] == "" || appErr.Fields["retrievalStrategy"] == "" {
		t.Fatalf("fields = %+v", appErr.Fields)
	}
}

func TestKnowledgeBaseOwnerIsolation(t *testing.T) {
	knowledge := newKnowledgeService(t)
	base, err := knowledge.CreateKnowledgeBase(context.Background(), actorContext(), service.CreateKnowledgeBaseInput{Name: "Private"})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}
	_, err = knowledge.GetKnowledgeBase(context.Background(), service.RequestContext{UserID: "usr_other"}, base.ID)
	if !hasCode(err, service.CodeNotFound) {
		t.Fatalf("other user GetKnowledgeBase() error = %v, want not_found", err)
	}
}

func TestListDocumentsAndChunks(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	_, err := repo.CreateKnowledgeBase(context.Background(), service.KnowledgeBase{
		ID:                "kb_docs",
		Name:              "Docs",
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
	repo.PutDocumentForTest(service.KnowledgeDocument{
		ID:              "doc_ready",
		KnowledgeBaseID: "kb_docs",
		FileID:          "file_1",
		Name:            "Ready",
		Status:          service.DocumentStatusReady,
		CreatedBy:       "usr_123",
		CreatedAt:       now,
	})
	repo.PutDocumentForTest(service.KnowledgeDocument{
		ID:              "doc_failed",
		KnowledgeBaseID: "kb_docs",
		FileID:          "file_2",
		Name:            "Failed",
		Status:          service.DocumentStatusFailed,
		CreatedBy:       "usr_123",
		CreatedAt:       now.Add(time.Minute),
	})
	repo.PutChunksForTest("doc_ready", []service.DocumentChunk{
		{ID: "chunk_2", KnowledgeBaseID: "kb_docs", DocumentID: "doc_ready", ChunkIndex: 1, Content: "two", TokenCount: 1, CreatedAt: now},
		{ID: "chunk_1", KnowledgeBaseID: "kb_docs", DocumentID: "doc_ready", ChunkIndex: 0, Content: "one", TokenCount: 1, CreatedAt: now},
	})
	knowledge := service.NewKnowledgeService(repo)

	docs, err := knowledge.ListDocuments(context.Background(), actorContext(), service.ListDocumentsInput{
		KnowledgeBaseID: "kb_docs",
		Status:          "ready",
	})
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if docs.Page.Total != 1 || len(docs.Items) != 1 || docs.Items[0].ID != "doc_ready" {
		t.Fatalf("docs = %+v", docs)
	}

	chunks, err := knowledge.ListChunks(context.Background(), actorContext(), service.ListChunksInput{
		DocumentID: "doc_ready",
		Page:       1,
		PageSize:   50,
	})
	if err != nil {
		t.Fatalf("ListChunks() error = %v", err)
	}
	if chunks.Page.Total != 2 || len(chunks.Items) != 2 || chunks.Items[0].ID != "chunk_1" {
		t.Fatalf("chunks = %+v", chunks)
	}
}

func TestListChunksRejectsNonReadyDocument(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	_, err := repo.CreateKnowledgeBase(context.Background(), service.KnowledgeBase{
		ID:                "kb_docs",
		Name:              "Docs",
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
	repo.PutDocumentForTest(service.KnowledgeDocument{
		ID:              "doc_processing",
		KnowledgeBaseID: "kb_docs",
		FileID:          "file_1",
		Name:            "Processing",
		Status:          service.DocumentStatusParsing,
		CreatedBy:       "usr_123",
		CreatedAt:       now,
	})
	knowledge := service.NewKnowledgeService(repo)

	_, err = knowledge.ListChunks(context.Background(), actorContext(), service.ListChunksInput{DocumentID: "doc_processing"})
	if !hasCode(err, service.CodeConflict) {
		t.Fatalf("ListChunks() error = %v, want conflict", err)
	}
}

func TestCreateIngestionJobIsIdempotent(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	_, err := repo.CreateKnowledgeBase(context.Background(), service.KnowledgeBase{
		ID:                "kb_jobs",
		Name:              "Jobs",
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
	knowledge := service.NewKnowledgeService(repo, service.WithClock(func() time.Time { return now }), service.WithIDGenerator(sequenceIDs()))

	first, err := knowledge.CreateIngestionJob(context.Background(), actorContext(), service.HandoffInput{
		KnowledgeBaseID: "kb_jobs",
		FileID:          "file_123",
		Name:            "manual.md",
		Tags:            []string{"tag", "tag"},
		IdempotencyKey:  "upload-123",
	})
	if err != nil {
		t.Fatalf("CreateIngestionJob() error = %v", err)
	}
	second, err := knowledge.CreateIngestionJob(context.Background(), actorContext(), service.HandoffInput{
		KnowledgeBaseID: "kb_jobs",
		FileID:          "file_123",
		Name:            "manual.md",
		IdempotencyKey:  "upload-123",
	})
	if err != nil {
		t.Fatalf("second CreateIngestionJob() error = %v", err)
	}
	if second.DocumentID != first.DocumentID || second.JobID != first.JobID {
		t.Fatalf("second = %+v, first = %+v", second, first)
	}

	job, err := knowledge.GetJob(context.Background(), actorContext(), first.JobID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if job.Status != service.JobStatusQueued || job.CurrentStage == nil || *job.CurrentStage != service.JobStageHandoff {
		t.Fatalf("job = %+v", job)
	}
}

func TestProcessIngestionJobTransitionsDocumentToReady(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	_, err := repo.CreateKnowledgeBase(context.Background(), service.KnowledgeBase{
		ID:                "kb_jobs",
		Name:              "Jobs",
		DocType:           "GENERAL",
		ChunkStrategy:     service.ChunkStrategy{"type": "SEMANTIC_TEXT", "chunkSize": 12, "overlap": 0},
		RetrievalStrategy: service.RetrievalStrategy{"mode": "VECTOR"},
		CreatedBy:         "usr_123",
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}
	knowledge := service.NewKnowledgeService(
		repo,
		service.WithClock(func() time.Time { return now }),
		service.WithIDGenerator(sequenceIDs()),
		service.WithPipeline(fakeSourceReader{content: "# Intro\n\ncontent for chunks"}, fakeParser{}, fakeChunker{}),
		service.WithVectorIndex(fakeEmbedder{}, vectorplatform.NewMemoryIndex()),
	)
	result, err := knowledge.CreateIngestionJob(context.Background(), actorContext(), service.HandoffInput{
		KnowledgeBaseID: "kb_jobs",
		FileID:          "file_123",
		Name:            "manual.md",
	})
	if err != nil {
		t.Fatalf("CreateIngestionJob() error = %v", err)
	}

	job, err := knowledge.ProcessIngestionJob(context.Background(), actorContext(), result.JobID)
	if err != nil {
		t.Fatalf("ProcessIngestionJob() error = %v", err)
	}
	if job.Status != service.JobStatusSucceeded {
		t.Fatalf("job = %+v", job)
	}
	doc, err := knowledge.GetDocument(context.Background(), actorContext(), result.DocumentID)
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	if doc.Status != service.DocumentStatusReady || doc.ChunkCount != 1 {
		t.Fatalf("doc = %+v", doc)
	}
	chunks, err := knowledge.ListChunks(context.Background(), actorContext(), service.ListChunksInput{DocumentID: result.DocumentID})
	if err != nil {
		t.Fatalf("ListChunks() error = %v", err)
	}
	if chunks.Page.Total != 1 || chunks.Items[0].Content != "content for chunks" {
		t.Fatalf("chunks = %+v", chunks)
	}
	if chunks.Items[0].QdrantPointID == nil || chunks.Items[0].EmbeddingProvider == nil || *chunks.Items[0].EmbeddingProvider != "fake" {
		t.Fatalf("chunk embedding metadata = %+v", chunks.Items[0])
	}
}

func TestCreateKnowledgeQueryReturnsHydratedResults(t *testing.T) {
	repo := repository.NewMemoryRepository()
	index := vectorplatform.NewMemoryIndex()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	_, err := repo.CreateKnowledgeBase(context.Background(), service.KnowledgeBase{
		ID:                "kb_query",
		Name:              "Query KB",
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
	knowledge := service.NewKnowledgeService(
		repo,
		service.WithClock(func() time.Time { return now }),
		service.WithIDGenerator(sequenceIDs()),
		service.WithPipeline(fakeSourceReader{content: "source"}, fakeParser{}, fakeChunker{}),
		service.WithVectorIndex(fakeEmbedder{}, index, "knowledge_chunks"),
	)
	handoff, err := knowledge.CreateIngestionJob(context.Background(), actorContext(), service.HandoffInput{
		KnowledgeBaseID: "kb_query",
		FileID:          "file_123",
		Name:            "manual.md",
		Tags:            []string{"policy"},
	})
	if err != nil {
		t.Fatalf("CreateIngestionJob() error = %v", err)
	}
	if _, err := knowledge.ProcessIngestionJob(context.Background(), actorContext(), handoff.JobID); err != nil {
		t.Fatalf("ProcessIngestionJob() error = %v", err)
	}

	query, err := knowledge.CreateKnowledgeQuery(context.Background(), actorContext(), service.KnowledgeQueryInput{
		Query:            "content",
		KnowledgeBaseIDs: []string{"kb_query"},
		TopK:             5,
		Tags:             []string{"policy"},
		Rerank:           true,
		RerankTopN:       intPtr(1),
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeQuery() error = %v", err)
	}
	if query.ID != "kq_4" || query.Trace.QdrantCollection != "knowledge_chunks" || query.Trace.HitCount != 1 {
		t.Fatalf("query trace = %+v id=%q", query.Trace, query.ID)
	}
	if len(query.Results) != 1 {
		t.Fatalf("results = %+v", query.Results)
	}
	result := query.Results[0]
	if result.KnowledgeBaseID != "kb_query" || result.DocumentID != handoff.DocumentID || result.DocumentName != "manual.md" {
		t.Fatalf("result = %+v", result)
	}
	if result.ContentPreview != "content for chunks" || len(result.Tags) != 1 || result.Tags[0] != "policy" {
		t.Fatalf("result content/tags = %+v", result)
	}
}

func TestCreateKnowledgeQueryUsesRuntimeDefaults(t *testing.T) {
	repo := repository.NewMemoryRepository()
	index := &recordingVectorIndex{}
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	_, err := repo.CreateKnowledgeBase(context.Background(), service.KnowledgeBase{
		ID:                "kb_query",
		Name:              "Query KB",
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
	repo.PutDocumentForTest(service.KnowledgeDocument{
		ID:              "doc_ready",
		KnowledgeBaseID: "kb_query",
		FileID:          "file_1",
		Name:            "manual.md",
		Status:          service.DocumentStatusReady,
		CreatedBy:       "usr_123",
		CreatedAt:       now,
	})
	repo.PutChunksForTest("doc_ready", []service.DocumentChunk{{
		ID:              "chunk_1",
		KnowledgeBaseID: "kb_query",
		DocumentID:      "doc_ready",
		ChunkIndex:      0,
		Content:         "content",
		CreatedAt:       now,
	}})
	index.hits = []service.VectorSearchHit{{
		ID:      "point_1",
		Score:   0.9,
		Payload: map[string]any{"chunk_id": "chunk_1"},
	}}
	knowledge := service.NewKnowledgeService(
		repo,
		service.WithClock(func() time.Time { return now }),
		service.WithIDGenerator(sequenceIDs()),
		service.WithVectorIndex(fakeEmbedder{}, index, "knowledge_chunks"),
	)
	topK := 17
	threshold := 0.7
	if _, err := knowledge.UpdateRuntimeConfig(context.Background(), adminContext(), service.RuntimeConfigUpdate{
		RetrievalTopK:  &topK,
		ScoreThreshold: &threshold,
	}); err != nil {
		t.Fatalf("UpdateRuntimeConfig() error = %v", err)
	}

	query, err := knowledge.CreateKnowledgeQuery(context.Background(), actorContext(), service.KnowledgeQueryInput{
		Query:            "content",
		KnowledgeBaseIDs: []string{"kb_query"},
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeQuery() error = %v", err)
	}
	if query.Trace.SearchTopK != 17 || query.Trace.ScoreThreshold != 0.7 {
		t.Fatalf("query trace = %+v", query.Trace)
	}
	if index.lastSearch.Limit != 17 || index.lastSearch.ScoreThreshold != 0.7 {
		t.Fatalf("last search = %+v", index.lastSearch)
	}
}

func TestProcessIngestionJobCleansVectorsWhenChunkPersistenceFails(t *testing.T) {
	repo := repository.NewMemoryRepository()
	failingRepo := &failingChunkRepository{Repository: repo}
	index := &recordingVectorIndex{}
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	_, err := repo.CreateKnowledgeBase(context.Background(), service.KnowledgeBase{
		ID:                "kb_pipeline",
		Name:              "Pipeline KB",
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
	knowledge := service.NewKnowledgeService(
		failingRepo,
		service.WithClock(func() time.Time { return now }),
		service.WithIDGenerator(sequenceIDs()),
		service.WithPipeline(fakeSourceReader{content: "source"}, fakeParser{}, fakeChunker{}),
		service.WithVectorIndex(fakeEmbedder{}, index, "knowledge_chunks"),
	)
	handoff, err := knowledge.CreateIngestionJob(context.Background(), actorContext(), service.HandoffInput{
		KnowledgeBaseID: "kb_pipeline",
		FileID:          "file_123",
		Name:            "manual.md",
	})
	if err != nil {
		t.Fatalf("CreateIngestionJob() error = %v", err)
	}

	_, err = knowledge.ProcessIngestionJob(context.Background(), actorContext(), handoff.JobID)
	if !hasCode(err, service.CodeDependency) {
		t.Fatalf("ProcessIngestionJob() error = %v, want dependency_error", err)
	}
	if len(index.upserted) == 0 {
		t.Fatalf("expected vectors to be upserted before repository failure")
	}
	if len(index.deletedDocuments) != 2 || index.deletedDocuments[1] != handoff.DocumentID {
		t.Fatalf("deleted documents = %+v", index.deletedDocuments)
	}
	doc, err := repo.FindDocumentByID(context.Background(), handoff.DocumentID)
	if err != nil {
		t.Fatalf("FindDocumentByID() error = %v", err)
	}
	if doc.Status != service.DocumentStatusFailed {
		t.Fatalf("document status = %s", doc.Status)
	}
}

func TestCreateKnowledgeQueryValidation(t *testing.T) {
	knowledge := newKnowledgeService(t)
	_, err := knowledge.CreateKnowledgeQuery(context.Background(), actorContext(), service.KnowledgeQueryInput{
		Query: "",
		TopK:  101,
	})
	var appErr *service.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want AppError", err)
	}
	if appErr.Code != service.CodeValidation || appErr.Fields["query"] == "" || appErr.Fields["topK"] == "" {
		t.Fatalf("appErr = %+v", appErr)
	}
}

func TestRuntimeConfigUpdateRequiresAdminAndSanitizesSecretRefs(t *testing.T) {
	knowledge := newKnowledgeService(t)
	_, err := knowledge.GetRuntimeConfig(context.Background(), actorContext())
	if !hasCode(err, service.CodeForbidden) {
		t.Fatalf("GetRuntimeConfig() error = %v, want forbidden", err)
	}
	topK := 20
	cfg, err := knowledge.UpdateRuntimeConfig(context.Background(), adminContext(), service.RuntimeConfigUpdate{
		RetrievalTopK: &topK,
		SecretRefs:    map[string]string{"embedding": "  ref_embedding  ", "empty": ""},
	})
	if err != nil {
		t.Fatalf("UpdateRuntimeConfig() error = %v", err)
	}
	if cfg.RetrievalTopK != 20 || cfg.SecretRefs["embedding"] != "ref_embedding" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if _, exists := cfg.SecretRefs["empty"]; exists {
		t.Fatalf("empty secret ref was retained: %+v", cfg.SecretRefs)
	}
}

func TestCreateReprocessingJobAndStats(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	_, err := repo.CreateKnowledgeBase(context.Background(), service.KnowledgeBase{
		ID:                "kb_admin",
		Name:              "Admin KB",
		DocType:           "GENERAL",
		ChunkStrategy:     service.ChunkStrategy{"type": "SEMANTIC_TEXT"},
		RetrievalStrategy: service.RetrievalStrategy{"mode": "VECTOR"},
		CreatedBy:         "usr_123",
		DocumentCount:     2,
		ChunkCount:        3,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}
	repo.PutDocumentForTest(service.KnowledgeDocument{
		ID:              "doc_ready",
		KnowledgeBaseID: "kb_admin",
		FileID:          "file_1",
		Name:            "Ready",
		Status:          service.DocumentStatusReady,
		CreatedBy:       "usr_123",
		CreatedAt:       now,
	})
	repo.PutDocumentForTest(service.KnowledgeDocument{
		ID:              "doc_failed",
		KnowledgeBaseID: "kb_admin",
		FileID:          "file_2",
		Name:            "Failed",
		Status:          service.DocumentStatusFailed,
		CreatedBy:       "usr_123",
		CreatedAt:       now,
	})
	knowledge := service.NewKnowledgeService(repo, service.WithClock(func() time.Time { return now }), service.WithIDGenerator(sequenceIDs()))

	job, err := knowledge.CreateReprocessingJob(context.Background(), adminContext(), "kb_admin")
	if err != nil {
		t.Fatalf("CreateReprocessingJob() error = %v", err)
	}
	if job.JobType != service.JobTypeReprocess || job.Status != service.JobStatusQueued {
		t.Fatalf("job = %+v", job)
	}

	stats, err := knowledge.GetKnowledgeStats(context.Background(), adminContext())
	if err != nil {
		t.Fatalf("GetKnowledgeStats() error = %v", err)
	}
	if stats.KnowledgeBaseCount != 1 || stats.DocumentCount != 2 || stats.ReadyDocumentCount != 1 || stats.FailedDocumentCount != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	if len(stats.RecentUploads) != 30 || stats.RecentUploads[29].Count != 2 {
		t.Fatalf("recent uploads = %+v", stats.RecentUploads)
	}
}

type fakeSourceReader struct {
	content string
}

func (r fakeSourceReader) ReadSource(ctx context.Context, fileID string) (service.SourceDocument, error) {
	return service.SourceDocument{
		Body:        io.NopCloser(strings.NewReader(r.content)),
		ContentType: "text/markdown",
		SizeBytes:   int64(len(r.content)),
	}, nil
}

type fakeParser struct{}

func (p fakeParser) Parse(ctx context.Context, input service.ParseInput) (service.ParsedDocument, error) {
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return service.ParsedDocument{}, err
	}
	return service.ParsedDocument{Content: string(data), Title: "Intro"}, nil
}

type fakeChunker struct{}

func (c fakeChunker) Chunk(ctx context.Context, input service.ChunkInput) ([]service.ChunkSpec, error) {
	chunkType := "text"
	return []service.ChunkSpec{{
		Content:    "content for chunks",
		TokenCount: 3,
		ChunkType:  &chunkType,
		Metadata:   map[string]any{"test": true},
	}}, nil
}

type fakeEmbedder struct{}

func (e fakeEmbedder) Embed(ctx context.Context, request service.EmbeddingRequest) (service.EmbeddingResult, error) {
	vectors := make([][]float32, 0, len(request.Texts))
	for range request.Texts {
		vectors = append(vectors, []float32{0.1, 0.2})
	}
	return service.EmbeddingResult{
		Vectors:   vectors,
		Provider:  "fake",
		Model:     "fake-model",
		Dimension: 2,
	}, nil
}

type recordingVectorIndex struct {
	mu               sync.Mutex
	upserted         []service.VectorPoint
	deletedDocuments []string
	hits             []service.VectorSearchHit
	lastSearch       service.VectorSearchRequest
}

func (i *recordingVectorIndex) Upsert(ctx context.Context, points []service.VectorPoint) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.upserted = append(i.upserted, points...)
	return nil
}

func (i *recordingVectorIndex) DeleteByDocument(ctx context.Context, documentID string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.deletedDocuments = append(i.deletedDocuments, documentID)
	return nil
}

func (i *recordingVectorIndex) Search(ctx context.Context, request service.VectorSearchRequest) ([]service.VectorSearchHit, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.lastSearch = request
	return append([]service.VectorSearchHit(nil), i.hits...), nil
}

type failingChunkRepository struct {
	service.Repository
}

func (r *failingChunkRepository) ReplaceDocumentChunks(ctx context.Context, documentID string, chunks []service.DocumentChunk) error {
	return errors.New("forced chunk persistence failure")
}

func newKnowledgeService(t *testing.T) *service.KnowledgeService {
	t.Helper()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	return service.NewKnowledgeService(
		repository.NewMemoryRepository(),
		service.WithClock(func() time.Time { return now }),
		service.WithIDGenerator(sequenceIDs()),
	)
}

func sequenceIDs() func(prefix string) (string, error) {
	counter := 0
	return func(prefix string) (string, error) {
		counter++
		return prefix + "_" + strconv.Itoa(counter), nil
	}
}

func actorContext() service.RequestContext {
	return service.RequestContext{RequestID: "req_test", UserID: "usr_123"}
}

func adminContext() service.RequestContext {
	return service.RequestContext{RequestID: "req_admin", UserID: "usr_admin", Permissions: []string{"knowledge:admin", "knowledge:write:any", "knowledge:read:any"}}
}

func hasCode(err error, code service.Code) bool {
	var appErr *service.AppError
	return errors.As(err, &appErr) && appErr.Code == code
}

func intPtr(value int) *int {
	return &value
}
