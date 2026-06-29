package worker_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/embedding"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/parser"
	sourceplatform "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/source"
	vectorplatform "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/vector"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/worker"
)

func TestIngestionHandlerRejectsInvalidPayloadWithoutTouchingState(t *testing.T) {
	handler, knowledge, repo, _ := newWorkerTestHarness(t, missingSourceReader{})
	seedKnowledgeBase(t, repo, "kb_jobs", "usr_123")
	handoff := createIngestionJob(t, knowledge, "kb_jobs", "file_123")

	err := handler.HandleIngestionPayload(context.Background(), mustJSON(t, map[string]string{
		"requestId": "req_worker",
		"jobId":     handoff.JobID,
	}))

	var appErr *service.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want AppError", err)
	}
	if appErr.Code != service.CodeValidation || appErr.Fields["userId"] == "" {
		t.Fatalf("appErr = %+v", appErr)
	}
	job, err := knowledge.GetJob(context.Background(), actorContext(), handoff.JobID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if job.Status != service.JobStatusQueued {
		t.Fatalf("job status = %s, want queued", job.Status)
	}
}

func TestIngestionHandlerProcessesQueuedJobToReady(t *testing.T) {
	sourceReader := sourceplatform.NewMemorySourceReader()
	sourceReader.Put("file_123", "# Intro\n\nThis is enough content for a text chunk.", "text/markdown")
	handler, knowledge, repo, _ := newWorkerTestHarness(t, sourceReader)
	seedKnowledgeBase(t, repo, "kb_jobs", "usr_123")
	handoff := createIngestionJob(t, knowledge, "kb_jobs", "file_123")

	if err := handler.HandleIngestionPayload(context.Background(), mustJSON(t, worker.IngestionPayload{
		RequestID: "req_worker",
		JobID:     handoff.JobID,
		UserID:    "usr_123",
	})); err != nil {
		t.Fatalf("HandleIngestionPayload() error = %v", err)
	}

	job, err := knowledge.GetJob(context.Background(), actorContext(), handoff.JobID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if job.Status != service.JobStatusSucceeded || job.ProgressPercent != 100 {
		t.Fatalf("job = %+v", job)
	}
	doc, err := knowledge.GetDocument(context.Background(), actorContext(), handoff.DocumentID)
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	if doc.Status != service.DocumentStatusReady || doc.ChunkCount == 0 {
		t.Fatalf("doc = %+v", doc)
	}
	chunks, err := knowledge.ListChunks(context.Background(), actorContext(), service.ListChunksInput{DocumentID: handoff.DocumentID})
	if err != nil {
		t.Fatalf("ListChunks() error = %v", err)
	}
	if chunks.Page.Total == 0 || chunks.Items[0].QdrantPointID == nil {
		t.Fatalf("chunks = %+v", chunks)
	}
}

func TestIngestionHandlerProcessesOfficeDocumentsToReady(t *testing.T) {
	sourceReader := sourceplatform.NewMemorySourceReader()
	handler, knowledge, repo, _ := newWorkerTestHarness(t, sourceReader)
	seedKnowledgeBase(t, repo, "kb_jobs", "usr_123")

	tests := []struct {
		name        string
		fileID      string
		contentType string
		body        []byte
	}{
		{
			name:        "docx",
			fileID:      "file_docx",
			contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			body: workerOfficeZip(t, map[string]string{
				"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Safety Manual</w:t></w:r></w:p><w:p><w:r><w:t>Breaker checklist</w:t></w:r></w:p></w:body></w:document>`,
			}),
		},
		{
			name:        "pptx",
			fileID:      "file_pptx",
			contentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
			body: workerOfficeZip(t, map[string]string{
				"ppt/presentation.xml":  `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`,
				"ppt/slides/slide1.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Intro slide</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
			}),
		},
		{
			name:        "xlsx",
			fileID:      "file_xlsx",
			contentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			body: workerOfficeZip(t, map[string]string{
				"xl/workbook.xml":          `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"/>`,
				"xl/sharedStrings.xml":     `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>Asset</t></si><si><t>Status</t></si></sst>`,
				"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row><row r="2"><c r="A2"><is><t>Transformer</t></is></c><c r="B2"><is><t>Ready</t></is></c></row></sheetData></worksheet>`,
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceReader.Put(tt.fileID, string(tt.body), tt.contentType)
			handoff := createIngestionJob(t, knowledge, "kb_jobs", tt.fileID)

			if err := handler.HandleIngestionPayload(context.Background(), mustJSON(t, worker.IngestionPayload{
				RequestID: "req_worker",
				JobID:     handoff.JobID,
				UserID:    "usr_123",
			})); err != nil {
				t.Fatalf("HandleIngestionPayload() error = %v", err)
			}
			job, err := knowledge.GetJob(context.Background(), actorContext(), handoff.JobID)
			if err != nil {
				t.Fatalf("GetJob() error = %v", err)
			}
			if job.Status != service.JobStatusSucceeded {
				t.Fatalf("job = %+v", job)
			}
			doc, err := knowledge.GetDocument(context.Background(), actorContext(), handoff.DocumentID)
			if err != nil {
				t.Fatalf("GetDocument() error = %v", err)
			}
			if doc.Status != service.DocumentStatusReady || doc.ChunkCount == 0 {
				t.Fatalf("doc = %+v", doc)
			}
			chunks, err := knowledge.ListChunks(context.Background(), actorContext(), service.ListChunksInput{DocumentID: handoff.DocumentID})
			if err != nil {
				t.Fatalf("ListChunks() error = %v", err)
			}
			if chunks.Page.Total == 0 || chunks.Items[0].Content == "" || chunks.Items[0].QdrantPointID == nil {
				t.Fatalf("chunks = %+v", chunks)
			}
		})
	}
}

func TestIngestionHandlerFailsUnsupportedDocumentsWithoutChunks(t *testing.T) {
	sourceReader := sourceplatform.NewMemorySourceReader()
	handler, knowledge, repo, vectorIndex := newWorkerTestHarness(t, sourceReader)
	seedKnowledgeBase(t, repo, "kb_jobs", "usr_123")

	tests := []struct {
		name        string
		fileID      string
		fileName    string
		contentType string
		body        string
	}{
		{name: "pdf", fileID: "file_pdf", fileName: "scan.pdf", contentType: "application/pdf", body: "%PDF-1.7\nsecret document text"},
		{name: "image", fileID: "file_png", fileName: "photo.png", contentType: "image/png", body: string([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 's', 'e', 'c', 'r', 'e', 't'})},
		{name: "damaged docx", fileID: "file_bad_docx", fileName: "broken.docx", contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", body: "not a zip but contains secret"},
		{name: "unknown utf8", fileID: "file_unknown", fileName: "blob.bin", contentType: "", body: "secret but not declared text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceReader.Put(tt.fileID, tt.body, tt.contentType)
			handoff := createIngestionJobWithName(t, knowledge, "kb_jobs", tt.fileID, tt.fileName)

			err := handler.HandleIngestionPayload(context.Background(), mustJSON(t, worker.IngestionPayload{
				RequestID: "req_worker",
				JobID:     handoff.JobID,
				UserID:    "usr_123",
			}))
			var appErr *service.AppError
			if !errors.As(err, &appErr) || appErr.Code != service.CodeValidation {
				t.Fatalf("HandleIngestionPayload() error = %v, want validation AppError", err)
			}
			if appErr.Error() == "" || bytes.Contains([]byte(appErr.Error()), []byte("secret")) {
				t.Fatalf("appErr leaked source content: %+v", appErr)
			}
			job, err := knowledge.GetJob(context.Background(), actorContext(), handoff.JobID)
			if err != nil {
				t.Fatalf("GetJob() error = %v", err)
			}
			if job.Status != service.JobStatusFailed || job.ErrorCode == nil || *job.ErrorCode != "parse_failed" {
				t.Fatalf("job = %+v", job)
			}
			doc, err := knowledge.GetDocument(context.Background(), actorContext(), handoff.DocumentID)
			if err != nil {
				t.Fatalf("GetDocument() error = %v", err)
			}
			if doc.Status != service.DocumentStatusFailed || doc.ErrorCode == nil || *doc.ErrorCode != "parse_failed" || doc.ChunkCount != 0 {
				t.Fatalf("doc = %+v", doc)
			}
			if vectorIndex.UpsertCount() != 0 {
				t.Fatalf("vector upsert count = %d, want 0", vectorIndex.UpsertCount())
			}
		})
	}
}

func TestIngestionHandlerProcessesPDFAndPPTXImageOCRToReady(t *testing.T) {
	sourceReader := sourceplatform.NewMemorySourceReader()
	ocr := &fakeWorkerOCRClient{textByName: map[string]string{
		"scan.pdf":             "Breaker cabinet warning",
		"ppt/media/image1.png": "Pump Station 3",
	}}
	handler, knowledge, repo, _ := newWorkerTestHarnessWithParser(t, sourceReader, parser.NewRouterWithOCR(ocr))
	seedKnowledgeBase(t, repo, "kb_jobs", "usr_123")

	pptx := workerOfficeZip(t, map[string]string{
		"ppt/presentation.xml":             `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rId1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":            `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:spTree><p:pic><p:blipFill><a:blip r:embed="rId2"/></p:blipFill></p:pic></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId2" Target="../media/image1.png" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"/></Relationships>`,
		"ppt/media/image1.png":             string([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 'l', 'a', 'b', 'e', 'l'}),
	})

	tests := []struct {
		name        string
		fileID      string
		fileName    string
		contentType string
		body        string
		wantText    string
	}{
		{name: "pdf", fileID: "file_pdf", fileName: "scan.pdf", contentType: "application/pdf", body: "%PDF-1.7\npage image", wantText: "Breaker cabinet warning"},
		{name: "pptx image", fileID: "file_pptx_image", fileName: "training.pptx", contentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation", body: string(pptx), wantText: "Pump Station 3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceReader.Put(tt.fileID, tt.body, tt.contentType)
			handoff := createIngestionJobWithName(t, knowledge, "kb_jobs", tt.fileID, tt.fileName)

			if err := handler.HandleIngestionPayload(context.Background(), mustJSON(t, worker.IngestionPayload{
				RequestID: "req_worker",
				JobID:     handoff.JobID,
				UserID:    "usr_123",
			})); err != nil {
				t.Fatalf("HandleIngestionPayload() error = %v", err)
			}
			doc, err := knowledge.GetDocument(context.Background(), actorContext(), handoff.DocumentID)
			if err != nil {
				t.Fatalf("GetDocument() error = %v", err)
			}
			if doc.Status != service.DocumentStatusReady || doc.ChunkCount == 0 {
				t.Fatalf("doc = %+v", doc)
			}
			chunks, err := knowledge.ListChunks(context.Background(), actorContext(), service.ListChunksInput{DocumentID: handoff.DocumentID})
			if err != nil {
				t.Fatalf("ListChunks() error = %v", err)
			}
			if chunks.Page.Total == 0 || !strings.Contains(chunks.Items[0].Content, tt.wantText) || chunks.Items[0].QdrantPointID == nil {
				t.Fatalf("chunks = %+v", chunks)
			}
		})
	}
}

func TestIngestionHandlerFailsDependencyWhenOCRFails(t *testing.T) {
	sourceReader := sourceplatform.NewMemorySourceReader()
	sourceReader.Put("file_pdf", "%PDF-1.7\nsecret page", "application/pdf")
	ocr := &fakeWorkerOCRClient{err: errors.New("ocr unavailable with secret")}
	handler, knowledge, repo, vectorIndex := newWorkerTestHarnessWithParser(t, sourceReader, parser.NewRouterWithOCR(ocr))
	seedKnowledgeBase(t, repo, "kb_jobs", "usr_123")
	handoff := createIngestionJobWithName(t, knowledge, "kb_jobs", "file_pdf", "scan.pdf")

	err := handler.HandleIngestionPayload(context.Background(), mustJSON(t, worker.IngestionPayload{
		RequestID: "req_worker",
		JobID:     handoff.JobID,
		UserID:    "usr_123",
	}))
	var appErr *service.AppError
	if !errors.As(err, &appErr) || appErr.Code != service.CodeDependency {
		t.Fatalf("HandleIngestionPayload() error = %v, want dependency AppError", err)
	}
	if bytes.Contains([]byte(appErr.Error()), []byte("secret")) {
		t.Fatalf("appErr leaked source content: %+v", appErr)
	}
	job, err := knowledge.GetJob(context.Background(), actorContext(), handoff.JobID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if job.Status != service.JobStatusFailed || job.ErrorCode == nil || *job.ErrorCode != "dependency_error" {
		t.Fatalf("job = %+v", job)
	}
	doc, err := knowledge.GetDocument(context.Background(), actorContext(), handoff.DocumentID)
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	if doc.Status != service.DocumentStatusFailed || doc.ErrorCode == nil || *doc.ErrorCode != "dependency_error" || doc.ChunkCount != 0 {
		t.Fatalf("doc = %+v", doc)
	}
	if vectorIndex.UpsertCount() != 0 {
		t.Fatalf("vector upsert count = %d, want 0", vectorIndex.UpsertCount())
	}
}

func TestIngestionHandlerDoesNotReprocessSucceededJob(t *testing.T) {
	sourceReader := sourceplatform.NewMemorySourceReader()
	sourceReader.Put("file_123", "content for exactly one processing run", "text/plain")
	handler, knowledge, repo, _ := newWorkerTestHarness(t, sourceReader)
	seedKnowledgeBase(t, repo, "kb_jobs", "usr_123")
	handoff := createIngestionJob(t, knowledge, "kb_jobs", "file_123")
	payload := mustJSON(t, worker.IngestionPayload{RequestID: "req_worker", JobID: handoff.JobID, UserID: "usr_123"})

	if err := handler.HandleIngestionPayload(context.Background(), payload); err != nil {
		t.Fatalf("first HandleIngestionPayload() error = %v", err)
	}
	err := handler.HandleIngestionPayload(context.Background(), payload)

	var appErr *service.AppError
	if !errors.As(err, &appErr) || appErr.Code != service.CodeConflict {
		t.Fatalf("second error = %v, want conflict", err)
	}
	chunks, err := knowledge.ListChunks(context.Background(), actorContext(), service.ListChunksInput{DocumentID: handoff.DocumentID})
	if err != nil {
		t.Fatalf("ListChunks() error = %v", err)
	}
	if chunks.Page.Total != 1 {
		t.Fatalf("chunk total = %d, want 1", chunks.Page.Total)
	}
}

func newWorkerTestHarness(t *testing.T, sourceReader service.SourceReader) (*worker.IngestionHandler, *service.KnowledgeService, *repository.MemoryRepository, *recordingWorkerVectorIndex) {
	t.Helper()
	return newWorkerTestHarnessWithParser(t, sourceReader, parser.NewRouter())
}

func newWorkerTestHarnessWithParser(t *testing.T, sourceReader service.SourceReader, documentParser service.Parser) (*worker.IngestionHandler, *service.KnowledgeService, *repository.MemoryRepository, *recordingWorkerVectorIndex) {
	t.Helper()
	repo := repository.NewMemoryRepository()
	vectorIndex := newRecordingWorkerVectorIndex()
	knowledge := service.NewKnowledgeService(
		repo,
		service.WithClock(func() time.Time { return fixedNow() }),
		service.WithIDGenerator(sequenceIDs()),
		service.WithPipeline(missingSourceReader{}, documentParser, parser.NewFixedChunker()),
		service.WithVectorIndex(embedding.NewLocalHasher("local_hashing", "local_hashing", 16), vectorIndex),
	)
	if sourceReader != nil {
		vectorIndex = newRecordingWorkerVectorIndex()
		knowledge = service.NewKnowledgeService(
			repo,
			service.WithClock(func() time.Time { return fixedNow() }),
			service.WithIDGenerator(sequenceIDs()),
			service.WithPipeline(sourceReader, documentParser, parser.NewFixedChunker()),
			service.WithVectorIndex(embedding.NewLocalHasher("local_hashing", "local_hashing", 16), vectorIndex),
		)
	}
	return worker.NewIngestionHandler(knowledge), knowledge, repo, vectorIndex
}

type missingSourceReader struct{}

func (missingSourceReader) ReadSource(ctx context.Context, fileID string) (service.SourceDocument, error) {
	return service.SourceDocument{}, errors.New("missing source")
}

func seedKnowledgeBase(t *testing.T, repo *repository.MemoryRepository, id string, owner string) {
	t.Helper()
	_, err := repo.CreateKnowledgeBase(context.Background(), service.KnowledgeBase{
		ID:                id,
		Name:              "Jobs",
		DocType:           "GENERAL",
		ChunkStrategy:     service.ChunkStrategy{"type": "SEMANTIC_TEXT", "chunkSize": 64, "overlap": 0},
		RetrievalStrategy: service.RetrievalStrategy{"mode": "VECTOR"},
		CreatedBy:         owner,
		CreatedAt:         fixedNow(),
		UpdatedAt:         fixedNow(),
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}
}

func createIngestionJob(t *testing.T, knowledge *service.KnowledgeService, kbID string, fileID string) service.HandoffResult {
	t.Helper()
	return createIngestionJobWithName(t, knowledge, kbID, fileID, "manual.md")
}

func createIngestionJobWithName(t *testing.T, knowledge *service.KnowledgeService, kbID string, fileID string, name string) service.HandoffResult {
	t.Helper()
	handoff, err := knowledge.CreateIngestionJob(context.Background(), actorContext(), service.HandoffInput{
		KnowledgeBaseID: kbID,
		FileID:          fileID,
		Name:            name,
	})
	if err != nil {
		t.Fatalf("CreateIngestionJob() error = %v", err)
	}
	return handoff
}

type recordingWorkerVectorIndex struct {
	inner    *vectorplatform.MemoryIndex
	upserted []service.VectorPoint
}

func newRecordingWorkerVectorIndex() *recordingWorkerVectorIndex {
	return &recordingWorkerVectorIndex{inner: vectorplatform.NewMemoryIndex()}
}

func (i *recordingWorkerVectorIndex) Upsert(ctx context.Context, points []service.VectorPoint) error {
	i.upserted = append(i.upserted, points...)
	return i.inner.Upsert(ctx, points)
}

func (i *recordingWorkerVectorIndex) DeleteByDocument(ctx context.Context, documentID string) error {
	return i.inner.DeleteByDocument(ctx, documentID)
}

func (i *recordingWorkerVectorIndex) Search(ctx context.Context, request service.VectorSearchRequest) ([]service.VectorSearchHit, error) {
	return i.inner.Search(ctx, request)
}

func (i *recordingWorkerVectorIndex) UpsertCount() int {
	return len(i.upserted)
}

type fakeWorkerOCRClient struct {
	textByName map[string]string
	err        error
	requests   []parser.OCRRequest
}

func (c *fakeWorkerOCRClient) ExtractText(ctx context.Context, request parser.OCRRequest) (parser.OCRResult, error) {
	c.requests = append(c.requests, request)
	if c.err != nil {
		return parser.OCRResult{}, c.err
	}
	return parser.OCRResult{Text: c.textByName[request.DocumentName]}, nil
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return data
}

func workerOfficeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("Create(%q) error = %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("Write(%q) error = %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip Close() error = %v", err)
	}
	return buf.Bytes()
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
}

func actorContext() service.RequestContext {
	return service.RequestContext{RequestID: "req_test", UserID: "usr_123"}
}

func sequenceIDs() func(prefix string) (string, error) {
	counter := 0
	return func(prefix string) (string, error) {
		counter++
		return prefix + "_" + strconv.Itoa(counter), nil
	}
}
