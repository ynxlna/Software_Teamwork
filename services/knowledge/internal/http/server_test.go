package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	knowledgehttp "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/http"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/embedding"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/parser"
	sourceplatform "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/source"
	vectorplatform "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/vector"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestHealthReturnsEnvelope(t *testing.T) {
	server := newHTTPTestServer()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "req_health")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if got := res.Header().Get("X-Request-Id"); got != "req_health" {
		t.Fatalf("X-Request-Id = %q", got)
	}
	var body healthResponseBody
	decodeJSON(t, res.Body, &body)
	if body.RequestID != "req_health" {
		t.Fatalf("requestId = %q", body.RequestID)
	}
	if body.Data.Service != "knowledge" || body.Data.Status != "ok" {
		t.Fatalf("data = %+v", body.Data)
	}
}

func TestReadyReturnsConfigurationSummary(t *testing.T) {
	server := newHTTPTestServer()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	req.Header.Set("X-Request-Id", "req_ready")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body readyResponseBody
	decodeJSON(t, res.Body, &body)
	if body.RequestID != "req_ready" {
		t.Fatalf("requestId = %q", body.RequestID)
	}
	if body.Data.Service != "knowledge" || body.Data.Status != "ready" {
		t.Fatalf("data = %+v", body.Data)
	}
	if body.Data.EmbeddingProvider != "local_hashing" || body.Data.EmbeddingDimension != 384 {
		t.Fatalf("embedding data = %+v", body.Data)
	}
	if body.Data.QdrantCollection != "knowledge_chunks" {
		t.Fatalf("qdrant collection = %q", body.Data.QdrantCollection)
	}
}

func TestReadyReportsInvalidConfiguration(t *testing.T) {
	status := service.NewStatusService(service.StatusConfig{
		Version:            "test",
		Environment:        "test",
		StorageBackend:     "memory",
		EmbeddingProvider:  "",
		EmbeddingModel:     "local_hashing",
		EmbeddingDimension: 0,
		QdrantCollection:   "",
	})
	server := knowledgehttp.NewServer(status, newKnowledgeTestService(), knowledgehttp.Config{})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	req.Header.Set("X-Request-Id", "req_bad_ready")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body errorResponseBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "validation_error" || body.Error.RequestID != "req_bad_ready" {
		t.Fatalf("error = %+v", body.Error)
	}
	if body.Error.Fields["embeddingDimension"] == "" || body.Error.Fields["qdrantCollection"] == "" {
		t.Fatalf("fields = %+v", body.Error.Fields)
	}
}

func TestUnknownRouteReturnsErrorEnvelope(t *testing.T) {
	server := newHTTPTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-bases", nil)
	req.Header.Set("X-Request-Id", "req_missing")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d", res.Code)
	}
	var body errorResponseBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "not_found" || body.Error.RequestID != "req_missing" {
		t.Fatalf("error = %+v", body.Error)
	}
}

func TestKnowledgeBaseCRUD(t *testing.T) {
	server := newHTTPTestServer()

	createReq := authorizedRequest(http.MethodPost, "/internal/v1/knowledge-bases", strings.NewReader(`{
		"id":"kb_test",
		"name":"规程规范",
		"description":"汽机规程",
		"docType":"general",
		"chunkStrategy":{"type":"SEMANTIC_TEXT","chunkSize":1200,"overlap":100},
		"retrievalStrategy":{"mode":"VECTOR","topK":8,"scoreThreshold":0.2}
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	server.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRes.Code, createRes.Body.String())
	}

	var createBody knowledgeBaseResponseBody
	decodeJSON(t, createRes.Body, &createBody)
	if createBody.RequestID != "req_test" {
		t.Fatalf("create requestId = %q", createBody.RequestID)
	}
	if createBody.Data.ID != "kb_test" || createBody.Data.Name != "规程规范" || createBody.Data.DocType != "GENERAL" {
		t.Fatalf("created base = %+v", createBody.Data)
	}
	if createBody.Data.ChunkStrategy["chunkSize"].(float64) != 1200 {
		t.Fatalf("chunk strategy = %+v", createBody.Data.ChunkStrategy)
	}

	getReq := authorizedRequest(http.MethodGet, "/internal/v1/knowledge-bases/kb_test", nil)
	getRes := httptest.NewRecorder()
	server.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getRes.Code, getRes.Body.String())
	}

	patchReq := authorizedRequest(http.MethodPatch, "/internal/v1/knowledge-bases/kb_test", strings.NewReader(`{"name":"更新后的规程","retrievalStrategy":{"mode":"VECTOR","topK":5}}`))
	patchReq.Header.Set("Content-Type", "application/json")
	patchRes := httptest.NewRecorder()
	server.ServeHTTP(patchRes, patchReq)
	if patchRes.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body = %s", patchRes.Code, patchRes.Body.String())
	}
	var patchBody knowledgeBaseResponseBody
	decodeJSON(t, patchRes.Body, &patchBody)
	if patchBody.Data.Name != "更新后的规程" || patchBody.Data.RetrievalStrategy["topK"].(float64) != 5 {
		t.Fatalf("patched base = %+v", patchBody.Data)
	}

	listReq := authorizedRequest(http.MethodGet, "/internal/v1/knowledge-bases?page=1&pageSize=20&keyword=更新&docType=GENERAL", nil)
	listRes := httptest.NewRecorder()
	server.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRes.Code, listRes.Body.String())
	}
	var listBody knowledgeBaseListResponseBody
	decodeJSON(t, listRes.Body, &listBody)
	if listBody.Page.Total != 1 || len(listBody.Data) != 1 {
		t.Fatalf("list body = %+v", listBody)
	}

	deleteReq := authorizedRequest(http.MethodDelete, "/internal/v1/knowledge-bases/kb_test", nil)
	deleteRes := httptest.NewRecorder()
	server.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", deleteRes.Code, deleteRes.Body.String())
	}

	getDeletedReq := authorizedRequest(http.MethodGet, "/internal/v1/knowledge-bases/kb_test", nil)
	getDeletedRes := httptest.NewRecorder()
	server.ServeHTTP(getDeletedRes, getDeletedReq)
	if getDeletedRes.Code != http.StatusNotFound {
		t.Fatalf("get deleted status = %d, body = %s", getDeletedRes.Code, getDeletedRes.Body.String())
	}
}

func TestKnowledgeBaseRoutesRequireGatewayUser(t *testing.T) {
	server := newHTTPTestServer()
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/knowledge-bases", nil)
	req.Header.Set("X-Request-Id", "req_no_user")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body errorResponseBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "unauthorized" {
		t.Fatalf("error code = %q", body.Error.Code)
	}
}

func TestKnowledgeBaseValidationErrors(t *testing.T) {
	server := newHTTPTestServer()
	req := authorizedRequest(http.MethodPost, "/internal/v1/knowledge-bases", bytes.NewBufferString(`{"name":"","retrievalStrategy":{"topK":101}}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body errorResponseBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "validation_error" || body.Error.Fields["name"] == "" || body.Error.Fields["retrievalStrategy"] == "" {
		t.Fatalf("error body = %+v", body)
	}
}

func TestKnowledgeBaseOwnerFiltering(t *testing.T) {
	server := newHTTPTestServer()
	createReq := authorizedRequest(http.MethodPost, "/internal/v1/knowledge-bases", strings.NewReader(`{"id":"kb_owner","name":"Owner KB"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	server.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRes.Code, createRes.Body.String())
	}

	otherReq := httptest.NewRequest(http.MethodGet, "/internal/v1/knowledge-bases/kb_owner", nil)
	otherReq.Header.Set("X-Request-Id", "req_other")
	otherReq.Header.Set("X-User-Id", "usr_other")
	otherRes := httptest.NewRecorder()
	server.ServeHTTP(otherRes, otherReq)
	if otherRes.Code != http.StatusNotFound {
		t.Fatalf("other user status = %d, body = %s", otherRes.Code, otherRes.Body.String())
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/internal/v1/knowledge-bases/kb_owner", nil)
	adminReq.Header.Set("X-Request-Id", "req_admin")
	adminReq.Header.Set("X-User-Id", "usr_admin")
	adminReq.Header.Set("X-User-Permissions", "knowledge:read:any")
	adminRes := httptest.NewRecorder()
	server.ServeHTTP(adminRes, adminReq)
	if adminRes.Code != http.StatusOK {
		t.Fatalf("admin status = %d, body = %s", adminRes.Code, adminRes.Body.String())
	}
}

func TestDocumentAndChunkReadAPIs(t *testing.T) {
	server, repo := newHTTPTestServerWithRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	seedKnowledgeBase(t, repo, "kb_docs", "usr_123", now)
	contentType := "text/markdown"
	parserBackend := "markdown"
	jobID := "job_123"
	repo.PutDocumentForTest(service.KnowledgeDocument{
		ID:              "doc_ready",
		KnowledgeBaseID: "kb_docs",
		FileID:          "file_123",
		Name:            "manual.md",
		ContentType:     &contentType,
		SizeBytes:       128,
		Status:          service.DocumentStatusReady,
		Tags:            []string{"policy"},
		ParserBackend:   &parserBackend,
		CreatedBy:       "usr_123",
		CreatedAt:       now,
		UpdatedAt:       &now,
		CurrentJobID:    &jobID,
		ChunkCount:      2,
	})
	sectionPath := "root"
	chunkType := "text"
	pointID := "point_1"
	embeddingProvider := "local_hashing"
	embeddingDimension := 384
	repo.PutChunksForTest("doc_ready", []service.DocumentChunk{
		{
			ID:                 "chunk_2",
			KnowledgeBaseID:    "kb_docs",
			DocumentID:         "doc_ready",
			ChunkIndex:         1,
			SectionPath:        &sectionPath,
			Content:            "second chunk",
			TokenCount:         2,
			ChunkType:          &chunkType,
			QdrantPointID:      &pointID,
			EmbeddingProvider:  &embeddingProvider,
			EmbeddingDimension: &embeddingDimension,
			Metadata:           map[string]any{"source": "test"},
			CreatedAt:          now,
		},
		{
			ID:              "chunk_1",
			KnowledgeBaseID: "kb_docs",
			DocumentID:      "doc_ready",
			ChunkIndex:      0,
			Content:         "first chunk",
			TokenCount:      2,
			CreatedAt:       now,
		},
	})

	listReq := authorizedRequest(http.MethodGet, "/internal/v1/knowledge-bases/kb_docs/documents?status=ready", nil)
	listRes := httptest.NewRecorder()
	server.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list documents status = %d, body = %s", listRes.Code, listRes.Body.String())
	}
	var listBody documentListResponseBody
	decodeJSON(t, listRes.Body, &listBody)
	if listBody.Page.Total != 1 || len(listBody.Data) != 1 || listBody.Data[0].ID != "doc_ready" {
		t.Fatalf("document list = %+v", listBody)
	}
	if listBody.Data[0].JobID == nil || *listBody.Data[0].JobID != "job_123" {
		t.Fatalf("document job id = %+v", listBody.Data[0].JobID)
	}

	getReq := authorizedRequest(http.MethodGet, "/internal/v1/documents/doc_ready", nil)
	getRes := httptest.NewRecorder()
	server.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("get document status = %d, body = %s", getRes.Code, getRes.Body.String())
	}

	chunksReq := authorizedRequest(http.MethodGet, "/internal/v1/documents/doc_ready/chunks?page=1&pageSize=50", nil)
	chunksRes := httptest.NewRecorder()
	server.ServeHTTP(chunksRes, chunksReq)
	if chunksRes.Code != http.StatusOK {
		t.Fatalf("chunks status = %d, body = %s", chunksRes.Code, chunksRes.Body.String())
	}
	var chunksBody chunkListResponseBody
	decodeJSON(t, chunksRes.Body, &chunksBody)
	if chunksBody.Page.Total != 2 || len(chunksBody.Data) != 2 {
		t.Fatalf("chunks body = %+v", chunksBody)
	}
	if chunksBody.Data[0].ID != "chunk_1" || chunksBody.Data[1].ID != "chunk_2" {
		t.Fatalf("chunk order = %+v", chunksBody.Data)
	}
	if chunksBody.Data[1].QdrantPointID == nil || *chunksBody.Data[1].QdrantPointID != "point_1" {
		t.Fatalf("chunk point id = %+v", chunksBody.Data[1].QdrantPointID)
	}
}

func TestDocumentChunkReadRequiresReadyDocument(t *testing.T) {
	server, repo := newHTTPTestServerWithRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	seedKnowledgeBase(t, repo, "kb_docs", "usr_123", now)
	repo.PutDocumentForTest(service.KnowledgeDocument{
		ID:              "doc_processing",
		KnowledgeBaseID: "kb_docs",
		FileID:          "file_123",
		Name:            "manual.md",
		Status:          service.DocumentStatusParsing,
		CreatedBy:       "usr_123",
		CreatedAt:       now,
	})

	req := authorizedRequest(http.MethodGet, "/internal/v1/documents/doc_processing/chunks", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body errorResponseBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "conflict" {
		t.Fatalf("error = %+v", body.Error)
	}
}

func TestDocumentRoutesApplyOwnerFiltering(t *testing.T) {
	server, repo := newHTTPTestServerWithRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	seedKnowledgeBase(t, repo, "kb_docs", "usr_123", now)
	repo.PutDocumentForTest(service.KnowledgeDocument{
		ID:              "doc_private",
		KnowledgeBaseID: "kb_docs",
		FileID:          "file_123",
		Name:            "manual.md",
		Status:          service.DocumentStatusReady,
		CreatedBy:       "usr_123",
		CreatedAt:       now,
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/documents/doc_private", nil)
	req.Header.Set("X-Request-Id", "req_other")
	req.Header.Set("X-User-Id", "usr_other")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
}

func TestCreateIngestionJobAndGetJob(t *testing.T) {
	server, repo := newHTTPTestServerWithRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	seedKnowledgeBase(t, repo, "kb_jobs", "usr_123", now)

	req := authorizedRequest(http.MethodPost, "/internal/v1/knowledge-bases/kb_jobs/ingestion-jobs", strings.NewReader(`{
		"fileId":"file_123",
		"name":"manual.md",
		"contentType":"text/markdown",
		"sizeBytes":128,
		"tags":[" policy ","policy","manual"],
		"idempotencyKey":"upload-123"
	}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("handoff status = %d, body = %s", res.Code, res.Body.String())
	}
	var body handoffResponseBody
	decodeJSON(t, res.Body, &body)
	if body.Data.DocumentID == "" || body.Data.JobID == "" || body.Data.Status != "uploaded" {
		t.Fatalf("handoff body = %+v", body)
	}

	docReq := authorizedRequest(http.MethodGet, "/internal/v1/documents/"+body.Data.DocumentID, nil)
	docRes := httptest.NewRecorder()
	server.ServeHTTP(docRes, docReq)
	if docRes.Code != http.StatusOK {
		t.Fatalf("document status = %d, body = %s", docRes.Code, docRes.Body.String())
	}
	var docBody documentResponseBody
	decodeJSON(t, docRes.Body, &docBody)
	if docBody.Data.JobID == nil || *docBody.Data.JobID != body.Data.JobID {
		t.Fatalf("document body = %+v", docBody)
	}
	if strings.Join(docBody.Data.Tags, ",") != "policy,manual" {
		t.Fatalf("document tags = %+v", docBody.Data.Tags)
	}

	jobReq := authorizedRequest(http.MethodGet, "/internal/v1/jobs/"+body.Data.JobID, nil)
	jobRes := httptest.NewRecorder()
	server.ServeHTTP(jobRes, jobReq)
	if jobRes.Code != http.StatusOK {
		t.Fatalf("job status = %d, body = %s", jobRes.Code, jobRes.Body.String())
	}
	var jobBody jobResponseBody
	decodeJSON(t, jobRes.Body, &jobBody)
	if jobBody.Data.Status != "queued" || jobBody.Data.CurrentStage == nil || *jobBody.Data.CurrentStage != "handoff" {
		t.Fatalf("job body = %+v", jobBody)
	}

	retryReq := authorizedRequest(http.MethodPost, "/internal/v1/knowledge-bases/kb_jobs/ingestion-jobs", strings.NewReader(`{
		"fileId":"file_123",
		"name":"manual.md",
		"idempotencyKey":"upload-123"
	}`))
	retryReq.Header.Set("Content-Type", "application/json")
	retryRes := httptest.NewRecorder()
	server.ServeHTTP(retryRes, retryReq)
	if retryRes.Code != http.StatusCreated {
		t.Fatalf("idempotent status = %d, body = %s", retryRes.Code, retryRes.Body.String())
	}
	var retryBody handoffResponseBody
	decodeJSON(t, retryRes.Body, &retryBody)
	if retryBody.Data.DocumentID != body.Data.DocumentID || retryBody.Data.JobID != body.Data.JobID {
		t.Fatalf("idempotent body = %+v, want %+v", retryBody.Data, body.Data)
	}
}

func TestCreateIngestionJobValidation(t *testing.T) {
	server, repo := newHTTPTestServerWithRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	seedKnowledgeBase(t, repo, "kb_jobs", "usr_123", now)
	req := authorizedRequest(http.MethodPost, "/internal/v1/knowledge-bases/kb_jobs/ingestion-jobs", strings.NewReader(`{"fileId":"","name":""}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body errorResponseBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "validation_error" || body.Error.Fields["fileId"] == "" || body.Error.Fields["name"] == "" {
		t.Fatalf("error body = %+v", body)
	}
}

func TestRunProcessingPipeline(t *testing.T) {
	sourceReader := sourceplatform.NewMemorySourceReader()
	sourceReader.Put("file_123", "# Intro\n\nThis is content for chunking.", "text/markdown")
	server, repo := newHTTPTestServerWithRepositoryAndPipeline(sourceReader)
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	seedKnowledgeBase(t, repo, "kb_jobs", "usr_123", now)

	handoffReq := authorizedRequest(http.MethodPost, "/internal/v1/knowledge-bases/kb_jobs/ingestion-jobs", strings.NewReader(`{"fileId":"file_123","name":"manual.md"}`))
	handoffReq.Header.Set("Content-Type", "application/json")
	handoffRes := httptest.NewRecorder()
	server.ServeHTTP(handoffRes, handoffReq)
	if handoffRes.Code != http.StatusCreated {
		t.Fatalf("handoff status = %d, body = %s", handoffRes.Code, handoffRes.Body.String())
	}
	var handoffBody handoffResponseBody
	decodeJSON(t, handoffRes.Body, &handoffBody)

	runReq := authorizedRequest(http.MethodPost, "/internal/v1/jobs/"+handoffBody.Data.JobID+"/processing-runs", nil)
	runRes := httptest.NewRecorder()
	server.ServeHTTP(runRes, runReq)
	if runRes.Code != http.StatusCreated {
		t.Fatalf("run status = %d, body = %s", runRes.Code, runRes.Body.String())
	}
	var jobBody jobResponseBody
	decodeJSON(t, runRes.Body, &jobBody)
	if jobBody.Data.Status != "succeeded" {
		t.Fatalf("job body = %+v", jobBody)
	}

	chunksReq := authorizedRequest(http.MethodGet, "/internal/v1/documents/"+handoffBody.Data.DocumentID+"/chunks", nil)
	chunksRes := httptest.NewRecorder()
	server.ServeHTTP(chunksRes, chunksReq)
	if chunksRes.Code != http.StatusOK {
		t.Fatalf("chunks status = %d, body = %s", chunksRes.Code, chunksRes.Body.String())
	}
	var chunksBody chunkListResponseBody
	decodeJSON(t, chunksRes.Body, &chunksBody)
	if chunksBody.Page.Total == 0 || chunksBody.Data[0].Content == "" {
		t.Fatalf("chunks body = %+v", chunksBody)
	}
	if chunksBody.Data[0].QdrantPointID == nil || *chunksBody.Data[0].QdrantPointID == "" {
		t.Fatalf("chunk qdrant point id missing = %+v", chunksBody.Data[0])
	}

	queryReq := authorizedRequest(http.MethodPost, "/internal/v1/knowledge-queries", strings.NewReader(`{
		"query":"content for chunking",
		"knowledgeBaseIds":["kb_jobs"],
		"topK":5,
		"scoreThreshold":0,
		"rerank":true,
		"rerankTopN":1
	}`))
	queryReq.Header.Set("Content-Type", "application/json")
	queryRes := httptest.NewRecorder()
	server.ServeHTTP(queryRes, queryReq)
	if queryRes.Code != http.StatusCreated {
		t.Fatalf("query status = %d, body = %s", queryRes.Code, queryRes.Body.String())
	}
	var queryBody knowledgeQueryResponseBody
	decodeJSON(t, queryRes.Body, &queryBody)
	if queryBody.Data.Query != "content for chunking" || queryBody.Data.Trace.HitCount != 1 {
		t.Fatalf("query body = %+v", queryBody)
	}
	if len(queryBody.Data.Results) != 1 || queryBody.Data.Results[0].DocumentID != handoffBody.Data.DocumentID {
		t.Fatalf("query results = %+v", queryBody.Data.Results)
	}
	if queryBody.Data.Results[0].ContentPreview == "" || queryBody.Data.Results[0].PointID == "" {
		t.Fatalf("query result = %+v", queryBody.Data.Results[0])
	}
}

func TestKnowledgeQueryValidation(t *testing.T) {
	server := newHTTPTestServer()
	req := authorizedRequest(http.MethodPost, "/internal/v1/knowledge-queries", strings.NewReader(`{"query":"","topK":101}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body errorResponseBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "validation_error" || body.Error.Fields["query"] == "" || body.Error.Fields["topK"] == "" {
		t.Fatalf("error body = %+v", body)
	}
}

func TestRuntimeConfigStatsAndReprocessJob(t *testing.T) {
	server, repo := newHTTPTestServerWithRepository()
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	seedKnowledgeBase(t, repo, "kb_admin", "usr_123", now)
	repo.PutDocumentForTest(service.KnowledgeDocument{
		ID:              "doc_ready",
		KnowledgeBaseID: "kb_admin",
		FileID:          "file_1",
		Name:            "ready.md",
		Status:          service.DocumentStatusReady,
		CreatedBy:       "usr_123",
		CreatedAt:       now,
	})

	getCfgReq := adminRequest(http.MethodGet, "/internal/v1/runtime-config", nil)
	getCfgRes := httptest.NewRecorder()
	server.ServeHTTP(getCfgRes, getCfgReq)
	if getCfgRes.Code != http.StatusOK {
		t.Fatalf("get config status = %d, body = %s", getCfgRes.Code, getCfgRes.Body.String())
	}

	patchCfgReq := adminRequest(http.MethodPatch, "/internal/v1/runtime-config", strings.NewReader(`{"retrievalTopK":15,"secretRefs":{"embedding":"ref_embedding"}}`))
	patchCfgReq.Header.Set("Content-Type", "application/json")
	patchCfgRes := httptest.NewRecorder()
	server.ServeHTTP(patchCfgRes, patchCfgReq)
	if patchCfgRes.Code != http.StatusOK {
		t.Fatalf("patch config status = %d, body = %s", patchCfgRes.Code, patchCfgRes.Body.String())
	}
	var cfgBody runtimeConfigResponseBody
	decodeJSON(t, patchCfgRes.Body, &cfgBody)
	if cfgBody.Data.RetrievalTopK != 15 || cfgBody.Data.SecretRefs["embedding"] != "ref_embedding" {
		t.Fatalf("config body = %+v", cfgBody)
	}

	jobReq := adminRequest(http.MethodPost, "/internal/v1/knowledge-bases/kb_admin/jobs", strings.NewReader(`{"jobType":"reprocess"}`))
	jobReq.Header.Set("Content-Type", "application/json")
	jobRes := httptest.NewRecorder()
	server.ServeHTTP(jobRes, jobReq)
	if jobRes.Code != http.StatusAccepted {
		t.Fatalf("job status = %d, body = %s", jobRes.Code, jobRes.Body.String())
	}
	var jobBody jobResponseBody
	decodeJSON(t, jobRes.Body, &jobBody)
	if jobBody.Data.Status != "queued" {
		t.Fatalf("job body = %+v", jobBody)
	}

	statsReq := adminRequest(http.MethodGet, "/internal/v1/knowledge-stats", nil)
	statsRes := httptest.NewRecorder()
	server.ServeHTTP(statsRes, statsReq)
	if statsRes.Code != http.StatusOK {
		t.Fatalf("stats status = %d, body = %s", statsRes.Code, statsRes.Body.String())
	}
	var statsBody knowledgeStatsResponseBody
	decodeJSON(t, statsRes.Body, &statsBody)
	if statsBody.Data.KnowledgeBaseCount != 1 || statsBody.Data.DocumentCount != 1 || statsBody.Data.ReadyDocumentCount != 1 {
		t.Fatalf("stats body = %+v", statsBody)
	}
}

func TestGeneratesRequestIDWhenMissing(t *testing.T) {
	server := newHTTPTestServer()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	requestID := res.Header().Get("X-Request-Id")
	if !strings.HasPrefix(requestID, "req_") {
		t.Fatalf("generated request id = %q", requestID)
	}
	var body healthResponseBody
	decodeJSON(t, res.Body, &body)
	if body.RequestID != requestID {
		t.Fatalf("body requestId = %q, header = %q", body.RequestID, requestID)
	}
}

func newHTTPTestServer() http.Handler {
	server, _ := newHTTPTestServerWithRepository()
	return server
}

func newHTTPTestServerWithRepository() (http.Handler, *repository.MemoryRepository) {
	return newHTTPTestServerWithRepositoryAndPipeline(nil)
}

func newHTTPTestServerWithRepositoryAndPipeline(sourceReader service.SourceReader) (http.Handler, *repository.MemoryRepository) {
	status := service.NewStatusService(service.StatusConfig{
		Version:            "test",
		Environment:        "test",
		StorageBackend:     "memory",
		EmbeddingProvider:  "local_hashing",
		EmbeddingModel:     "local_hashing",
		EmbeddingDimension: 384,
		QdrantCollection:   "knowledge_chunks",
	})
	repo := repository.NewMemoryRepository()
	options := []service.KnowledgeOption{}
	if sourceReader != nil {
		options = append(options, service.WithPipeline(sourceReader, parser.NewTextParser(), parser.NewFixedChunker()))
		options = append(options, service.WithVectorIndex(embedding.NewLocalHasher("local_hashing", "local_hashing", 16), vectorplatform.NewMemoryIndex()))
	}
	return knowledgehttp.NewServer(status, service.NewKnowledgeService(repo, options...), knowledgehttp.Config{}), repo
}

func newKnowledgeTestService() *service.KnowledgeService {
	repo := repository.NewMemoryRepository()
	return service.NewKnowledgeService(repo)
}

func seedKnowledgeBase(t *testing.T, repo *repository.MemoryRepository, id string, owner string, createdAt time.Time) {
	t.Helper()
	_, err := repo.CreateKnowledgeBase(context.Background(), service.KnowledgeBase{
		ID:                id,
		Name:              "KB " + id,
		Description:       "",
		DocType:           "GENERAL",
		ChunkStrategy:     service.ChunkStrategy{"type": "SEMANTIC_TEXT"},
		RetrievalStrategy: service.RetrievalStrategy{"mode": "VECTOR"},
		CreatedBy:         owner,
		CreatedAt:         createdAt,
		UpdatedAt:         createdAt,
	})
	if err != nil {
		t.Fatalf("seed knowledge base error = %v", err)
	}
}

func authorizedRequest(method string, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("X-Request-Id", "req_test")
	req.Header.Set("X-User-Id", "usr_123")
	req.Header.Set("X-User-Roles", "admin")
	req.Header.Set("X-User-Permissions", "knowledge:read,knowledge:write")
	return req
}

func adminRequest(method string, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("X-Request-Id", "req_admin")
	req.Header.Set("X-User-Id", "usr_admin")
	req.Header.Set("X-User-Roles", "admin")
	req.Header.Set("X-User-Permissions", "knowledge:admin,knowledge:read:any,knowledge:write:any")
	return req
}

func decodeJSON(t *testing.T, reader io.Reader, target any) {
	t.Helper()
	if err := json.NewDecoder(reader).Decode(target); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
}

type healthResponseBody struct {
	Data struct {
		Service string `json:"service"`
		Status  string `json:"status"`
	} `json:"data"`
	RequestID string `json:"requestId"`
}

type readyResponseBody struct {
	Data struct {
		Service            string `json:"service"`
		Status             string `json:"status"`
		Version            string `json:"version"`
		Environment        string `json:"environment"`
		StorageBackend     string `json:"storageBackend"`
		EmbeddingProvider  string `json:"embeddingProvider"`
		EmbeddingModel     string `json:"embeddingModel"`
		EmbeddingDimension int    `json:"embeddingDimension"`
		QdrantCollection   string `json:"qdrantCollection"`
	} `json:"data"`
	RequestID string `json:"requestId"`
}

type errorResponseBody struct {
	Error struct {
		Code      string            `json:"code"`
		Message   string            `json:"message"`
		RequestID string            `json:"requestId"`
		Fields    map[string]string `json:"fields"`
	} `json:"error"`
}

type knowledgeBaseResponseBody struct {
	Data struct {
		ID                string         `json:"id"`
		Name              string         `json:"name"`
		Description       string         `json:"description"`
		DocType           string         `json:"docType"`
		ChunkStrategy     map[string]any `json:"chunkStrategy"`
		RetrievalStrategy map[string]any `json:"retrievalStrategy"`
		DocumentCount     int            `json:"documentCount"`
		ChunkCount        int            `json:"chunkCount"`
		CreatedBy         string         `json:"createdBy"`
		CreatedAt         string         `json:"createdAt"`
		UpdatedAt         string         `json:"updatedAt"`
	} `json:"data"`
	RequestID string `json:"requestId"`
}

type knowledgeBaseListResponseBody struct {
	Data []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		DocType string `json:"docType"`
	} `json:"data"`
	Page struct {
		Page     int `json:"page"`
		PageSize int `json:"pageSize"`
		Total    int `json:"total"`
	} `json:"page"`
	RequestID string `json:"requestId"`
}

type documentListResponseBody struct {
	Data []struct {
		ID              string   `json:"id"`
		KnowledgeBaseID string   `json:"knowledgeBaseId"`
		Name            string   `json:"name"`
		Status          string   `json:"status"`
		ChunkCount      int      `json:"chunkCount"`
		Tags            []string `json:"tags"`
		JobID           *string  `json:"jobId"`
	} `json:"data"`
	Page struct {
		Page     int `json:"page"`
		PageSize int `json:"pageSize"`
		Total    int `json:"total"`
	} `json:"page"`
	RequestID string `json:"requestId"`
}

type documentResponseBody struct {
	Data struct {
		ID     string   `json:"id"`
		Tags   []string `json:"tags"`
		JobID  *string  `json:"jobId"`
		Status string   `json:"status"`
	} `json:"data"`
	RequestID string `json:"requestId"`
}

type chunkListResponseBody struct {
	Data []struct {
		ID            string  `json:"id"`
		DocumentID    string  `json:"documentId"`
		ChunkIndex    int     `json:"chunkIndex"`
		Content       string  `json:"content"`
		QdrantPointID *string `json:"qdrantPointId"`
	} `json:"data"`
	Page struct {
		Page     int `json:"page"`
		PageSize int `json:"pageSize"`
		Total    int `json:"total"`
	} `json:"page"`
	RequestID string `json:"requestId"`
}

type handoffResponseBody struct {
	Data struct {
		DocumentID string `json:"documentId"`
		JobID      string `json:"jobId"`
		Status     string `json:"status"`
	} `json:"data"`
	RequestID string `json:"requestId"`
}

type jobResponseBody struct {
	Data struct {
		ID           string  `json:"id"`
		DocumentID   *string `json:"documentId"`
		Status       string  `json:"status"`
		CurrentStage *string `json:"currentStage"`
	} `json:"data"`
	RequestID string `json:"requestId"`
}

type knowledgeQueryResponseBody struct {
	Data struct {
		ID      string `json:"id"`
		Query   string `json:"query"`
		Results []struct {
			Score           float64 `json:"score"`
			PointID         string  `json:"pointId"`
			KnowledgeBaseID string  `json:"knowledgeBaseId"`
			DocumentID      string  `json:"documentId"`
			ChunkID         string  `json:"chunkId"`
			DocumentName    string  `json:"documentName"`
			ContentPreview  string  `json:"contentPreview"`
		} `json:"results"`
		Trace struct {
			EmbeddingProvider  string `json:"embeddingProvider"`
			EmbeddingModel     string `json:"embeddingModel"`
			EmbeddingDimension int    `json:"embeddingDimension"`
			QdrantCollection   string `json:"qdrantCollection"`
			SearchTopK         int    `json:"searchTopK"`
			HitCount           int    `json:"hitCount"`
			Rerank             bool   `json:"rerank"`
		} `json:"trace"`
	} `json:"data"`
	RequestID string `json:"requestId"`
}

type runtimeConfigResponseBody struct {
	Data struct {
		RetrievalTopK int               `json:"retrievalTopK"`
		SecretRefs    map[string]string `json:"secretRefs"`
	} `json:"data"`
	RequestID string `json:"requestId"`
}

type knowledgeStatsResponseBody struct {
	Data struct {
		KnowledgeBaseCount int `json:"knowledgeBaseCount"`
		DocumentCount      int `json:"documentCount"`
		ReadyDocumentCount int `json:"readyDocumentCount"`
	} `json:"data"`
	RequestID string `json:"requestId"`
}
