package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type Config struct {
	Logger *slog.Logger
}

type Server struct {
	status    *service.StatusService
	knowledge *service.KnowledgeService
	logger    *slog.Logger
	mux       *http.ServeMux
}

func NewServer(status *service.StatusService, knowledge *service.KnowledgeService, cfg Config) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	s := &Server{
		status:    status,
		knowledge: knowledge,
		logger:    cfg.Logger,
		mux:       http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /readyz", s.handleReady)
	s.mux.HandleFunc("GET /internal/v1/knowledge-bases", s.handleListKnowledgeBases)
	s.mux.HandleFunc("POST /internal/v1/knowledge-bases", s.handleCreateKnowledgeBase)
	s.mux.HandleFunc("GET /internal/v1/knowledge-bases/{knowledgeBaseId}", s.handleGetKnowledgeBase)
	s.mux.HandleFunc("PATCH /internal/v1/knowledge-bases/{knowledgeBaseId}", s.handleUpdateKnowledgeBase)
	s.mux.HandleFunc("DELETE /internal/v1/knowledge-bases/{knowledgeBaseId}", s.handleDeleteKnowledgeBase)
	s.mux.HandleFunc("GET /internal/v1/knowledge-bases/{knowledgeBaseId}/documents", s.handleListDocuments)
	s.mux.HandleFunc("POST /internal/v1/knowledge-bases/{knowledgeBaseId}/ingestion-jobs", s.handleCreateIngestionJob)
	s.mux.HandleFunc("POST /internal/v1/knowledge-bases/{knowledgeBaseId}/jobs", s.handleCreateKnowledgeBaseJob)
	s.mux.HandleFunc("GET /internal/v1/documents/{documentId}", s.handleGetDocument)
	s.mux.HandleFunc("GET /internal/v1/documents/{documentId}/chunks", s.handleListChunks)
	s.mux.HandleFunc("GET /internal/v1/jobs/{jobId}", s.handleGetJob)
	s.mux.HandleFunc("POST /internal/v1/jobs/{jobId}/processing-runs", s.handleRunJobProcessing)
	s.mux.HandleFunc("POST /internal/v1/knowledge-queries", s.handleCreateKnowledgeQuery)
	s.mux.HandleFunc("GET /internal/v1/runtime-config", s.handleGetRuntimeConfig)
	s.mux.HandleFunc("PATCH /internal/v1/runtime-config", s.handleUpdateRuntimeConfig)
	s.mux.HandleFunc("GET /internal/v1/knowledge-stats", s.handleGetKnowledgeStats)
	s.mux.HandleFunc("/", s.handleNotFound)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
	if requestID == "" {
		requestID = newRequestID()
	}

	ctx := contextWithRequestID(r.Context(), requestID)
	r = r.WithContext(ctx)
	w.Header().Set("X-Request-Id", requestID)

	recorder := &statusRecorder{ResponseWriter: w}
	start := time.Now()
	defer func() {
		if recovered := recover(); recovered != nil {
			s.logger.ErrorContext(ctx, "http panic recovered", "service", "knowledge", "request_id", requestID, "operation", "http_request")
			writeAppError(recorder, r, service.NewError(service.CodeInternal, "internal server error", nil))
		}
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		if status >= http.StatusInternalServerError {
			s.logger.ErrorContext(ctx, "http request failed", "service", "knowledge", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "status", status, "duration_ms", time.Since(start).Milliseconds())
		}
	}()

	s.mux.ServeHTTP(recorder, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := s.status.Health(r.Context())
	writeJSON(w, http.StatusOK, healthResponseFromDomain(status), requestIDFromContext(r.Context()))
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	status, err := s.status.Ready(r.Context())
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, readyResponseFromDomain(status), requestIDFromContext(r.Context()))
}

func (s *Server) handleListKnowledgeBases(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	input, err := listKnowledgeBasesInputFromRequest(r)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	result, err := s.knowledge.ListKnowledgeBases(r.Context(), reqCtx, input)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writePaginatedJSON(w, http.StatusOK, knowledgeBaseListFromDomain(result), pageInfoFromDomain(result.Page), requestIDFromContext(r.Context()))
}

func (s *Server) handleCreateKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()

	var payload knowledgeBaseRequest
	if err := decodeJSONBody(r, &payload); err != nil {
		writeAppError(w, r, err)
		return
	}

	base, err := s.knowledge.CreateKnowledgeBase(r.Context(), reqCtx, service.CreateKnowledgeBaseInput{
		ID:                payload.ID,
		Name:              payload.Name,
		Description:       payload.Description,
		DocType:           payload.DocType,
		ChunkStrategy:     service.ChunkStrategy(payload.ChunkStrategy),
		RetrievalStrategy: service.RetrievalStrategy(payload.RetrievalStrategy),
	})
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, knowledgeBaseSummaryFromDomain(base), requestIDFromContext(r.Context()))
}

func (s *Server) handleGetKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	base, err := s.knowledge.GetKnowledgeBase(r.Context(), reqCtx, r.PathValue("knowledgeBaseId"))
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, knowledgeBaseSummaryFromDomain(base), requestIDFromContext(r.Context()))
}

func (s *Server) handleUpdateKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()

	var payload updateKnowledgeBaseRequest
	if err := decodeJSONBody(r, &payload); err != nil {
		writeAppError(w, r, err)
		return
	}

	input := service.UpdateKnowledgeBaseInput{
		ID: r.PathValue("knowledgeBaseId"),
	}
	if payload.Name != nil {
		input.Name = payload.Name
	}
	if payload.Description != nil {
		input.Description = payload.Description
	}
	if payload.DocType != nil {
		input.DocType = payload.DocType
	}
	if payload.ChunkStrategy != nil {
		input.ChunkStrategy = service.ChunkStrategy(payload.ChunkStrategy)
	}
	if payload.RetrievalStrategy != nil {
		input.RetrievalStrategy = service.RetrievalStrategy(payload.RetrievalStrategy)
	}

	base, err := s.knowledge.UpdateKnowledgeBase(r.Context(), reqCtx, input)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, knowledgeBaseSummaryFromDomain(base), requestIDFromContext(r.Context()))
}

func (s *Server) handleDeleteKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	if err := s.knowledge.DeleteKnowledgeBase(r.Context(), reqCtx, r.PathValue("knowledgeBaseId")); err != nil {
		writeAppError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	input, err := listDocumentsInputFromRequest(r)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	input.KnowledgeBaseID = r.PathValue("knowledgeBaseId")
	result, err := s.knowledge.ListDocuments(r.Context(), reqCtx, input)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writePaginatedJSON(w, http.StatusOK, documentListFromDomain(result), pageInfoFromDomain(result.Page), requestIDFromContext(r.Context()))
}

func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	doc, err := s.knowledge.GetDocument(r.Context(), reqCtx, r.PathValue("documentId"))
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, documentSummaryFromDomain(doc), requestIDFromContext(r.Context()))
}

func (s *Server) handleListChunks(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	input, err := listChunksInputFromRequest(r)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	input.DocumentID = r.PathValue("documentId")
	result, err := s.knowledge.ListChunks(r.Context(), reqCtx, input)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writePaginatedJSON(w, http.StatusOK, documentChunkListFromDomain(result), pageInfoFromDomain(result.Page), requestIDFromContext(r.Context()))
}

func (s *Server) handleCreateIngestionJob(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()

	var payload handoffRequest
	if err := decodeJSONBody(r, &payload); err != nil {
		writeAppError(w, r, err)
		return
	}
	result, err := s.knowledge.CreateIngestionJob(r.Context(), reqCtx, service.HandoffInput{
		KnowledgeBaseID: r.PathValue("knowledgeBaseId"),
		FileID:          payload.FileID,
		Name:            payload.Name,
		ContentType:     payload.ContentType,
		SizeBytes:       payload.SizeBytes,
		Tags:            payload.Tags,
		CreatedBy:       payload.CreatedBy,
		IdempotencyKey:  payload.IdempotencyKey,
	})
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, handoffResponseFromDomain(result), requestIDFromContext(r.Context()))
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	job, err := s.knowledge.GetJob(r.Context(), reqCtx, r.PathValue("jobId"))
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, processingJobSummaryFromDomain(job), requestIDFromContext(r.Context()))
}

func (s *Server) handleRunJobProcessing(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	job, err := s.knowledge.ProcessIngestionJob(r.Context(), reqCtx, r.PathValue("jobId"))
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, processingJobSummaryFromDomain(job), requestIDFromContext(r.Context()))
}

func (s *Server) handleCreateKnowledgeQuery(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()

	var payload knowledgeQueryRequest
	if err := decodeJSONBody(r, &payload); err != nil {
		writeAppError(w, r, err)
		return
	}
	result, err := s.knowledge.CreateKnowledgeQuery(r.Context(), reqCtx, service.KnowledgeQueryInput{
		Query:            payload.Query,
		KnowledgeBaseIDs: payload.KnowledgeBaseIDs,
		TopK:             payload.TopK,
		ScoreThreshold:   payload.ScoreThreshold,
		Tags:             payload.Tags,
		MetadataFilter:   payload.MetadataFilter,
		Rerank:           payload.Rerank,
		RerankTopN:       payload.RerankTopN,
	})
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, knowledgeQueryFromDomain(result), requestIDFromContext(r.Context()))
}

func (s *Server) handleGetRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	cfg, err := s.knowledge.GetRuntimeConfig(r.Context(), reqCtx)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, runtimeConfigFromDomain(cfg), requestIDFromContext(r.Context()))
}

func (s *Server) handleUpdateRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()

	var payload runtimeConfigRequest
	if err := decodeJSONBody(r, &payload); err != nil {
		writeAppError(w, r, err)
		return
	}
	cfg, err := s.knowledge.UpdateRuntimeConfig(r.Context(), reqCtx, service.RuntimeConfigUpdate{
		ParserBackend:        payload.ParserBackend,
		RerankProvider:       payload.RerankProvider,
		RerankModel:          payload.RerankModel,
		RetrievalTopK:        payload.RetrievalTopK,
		ScoreThreshold:       payload.ScoreThreshold,
		MaxConcurrentJobs:    payload.MaxConcurrentJobs,
		ProcessingTimeoutSec: payload.ProcessingTimeoutSec,
		SecretRefs:           payload.SecretRefs,
	})
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, runtimeConfigFromDomain(cfg), requestIDFromContext(r.Context()))
}

func (s *Server) handleCreateKnowledgeBaseJob(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()

	var payload createKnowledgeBaseJobRequest
	if err := decodeJSONBody(r, &payload); err != nil {
		writeAppError(w, r, err)
		return
	}
	if payload.JobType != "" && payload.JobType != string(service.JobTypeReprocess) {
		writeAppError(w, r, service.ValidationError("request validation failed", map[string]string{"jobType": "must be reprocess"}))
		return
	}
	job, err := s.knowledge.CreateReprocessingJob(r.Context(), reqCtx, r.PathValue("knowledgeBaseId"))
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, processingJobSummaryFromDomain(job), requestIDFromContext(r.Context()))
}

func (s *Server) handleGetKnowledgeStats(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.gatewayContext(w, r)
	if !ok {
		return
	}
	stats, err := s.knowledge.GetKnowledgeStats(r.Context(), reqCtx)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, knowledgeStatsFromDomain(stats), requestIDFromContext(r.Context()))
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeAppError(w, r, service.NotFoundError("route not found"))
}

func (s *Server) gatewayContext(w http.ResponseWriter, r *http.Request) (service.RequestContext, bool) {
	reqCtx := service.RequestContext{
		RequestID:      requestIDFromContext(r.Context()),
		UserID:         strings.TrimSpace(r.Header.Get("X-User-Id")),
		Roles:          splitCSV(r.Header.Get("X-User-Roles")),
		Permissions:    splitCSV(r.Header.Get("X-User-Permissions")),
		ForwardedFor:   strings.TrimSpace(r.Header.Get("X-Forwarded-For")),
		ForwardedProto: strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")),
	}
	if reqCtx.UserID == "" {
		writeAppError(w, r, service.UnauthorizedError())
		return service.RequestContext{}, false
	}
	return reqCtx, true
}

func listKnowledgeBasesInputFromRequest(r *http.Request) (service.ListKnowledgeBasesInput, error) {
	query := r.URL.Query()
	page, err := parseOptionalInt(query.Get("page"), "page")
	if err != nil {
		return service.ListKnowledgeBasesInput{}, err
	}
	pageSize, err := parseOptionalInt(query.Get("pageSize"), "pageSize")
	if err != nil {
		return service.ListKnowledgeBasesInput{}, err
	}
	return service.ListKnowledgeBasesInput{
		Page:     page,
		PageSize: pageSize,
		Keyword:  query.Get("keyword"),
		DocType:  query.Get("docType"),
	}, nil
}

func listDocumentsInputFromRequest(r *http.Request) (service.ListDocumentsInput, error) {
	query := r.URL.Query()
	page, err := parseOptionalInt(query.Get("page"), "page")
	if err != nil {
		return service.ListDocumentsInput{}, err
	}
	pageSize, err := parseOptionalInt(query.Get("pageSize"), "pageSize")
	if err != nil {
		return service.ListDocumentsInput{}, err
	}
	return service.ListDocumentsInput{
		Page:     page,
		PageSize: pageSize,
		Status:   query.Get("status"),
	}, nil
}

func listChunksInputFromRequest(r *http.Request) (service.ListChunksInput, error) {
	query := r.URL.Query()
	page, err := parseOptionalInt(query.Get("page"), "page")
	if err != nil {
		return service.ListChunksInput{}, err
	}
	pageSize, err := parseOptionalInt(query.Get("pageSize"), "pageSize")
	if err != nil {
		return service.ListChunksInput{}, err
	}
	return service.ListChunksInput{
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func parseOptionalInt(raw string, field string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, service.ValidationError("request validation failed", map[string]string{field: "must be an integer"})
	}
	return value, nil
}

func decodeJSONBody(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return service.ValidationError("request validation failed", map[string]string{"body": "must be a valid JSON object"})
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return service.ValidationError("request validation failed", map[string]string{"body": "must contain only one JSON object"})
	}
	return nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func newRequestID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "req_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return "req_" + hex.EncodeToString(bytes)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(body)
}
