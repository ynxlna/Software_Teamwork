package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

type ReadyChecker interface {
	CheckReady(context.Context) error
}

type DocumentService interface {
	ListReportTypes(context.Context, service.RequestContext) ([]service.ReportType, error)
	ListReportTemplates(context.Context, service.RequestContext, service.ReportTemplateListFilter) (service.ReportTemplateListResult, error)
	CreateReportTemplate(context.Context, service.RequestContext, service.CreateReportTemplateInput) (service.ReportTemplate, error)
	GetReportTemplate(context.Context, service.RequestContext, string) (service.ReportTemplate, error)
	UpdateReportTemplate(context.Context, service.RequestContext, service.UpdateReportTemplateInput) (service.ReportTemplate, error)
	DeleteReportTemplate(context.Context, service.RequestContext, string) error
	GetReportTemplateStructure(context.Context, service.RequestContext, string) (service.ReportTemplateStructure, error)
	UpdateReportTemplateStructure(context.Context, service.RequestContext, service.UpdateReportTemplateStructureInput) (service.ReportTemplateStructure, error)
	ListReportMaterials(context.Context, service.RequestContext, service.ReportMaterialListFilter) (service.ReportMaterialListResult, error)
	CreateReportMaterial(context.Context, service.RequestContext, service.CreateReportMaterialInput) (service.ReportMaterial, error)
	GetReportMaterial(context.Context, service.RequestContext, string) (service.ReportMaterial, error)
	DeleteReportMaterial(context.Context, service.RequestContext, string) error
}

type JobSvc interface {
	CreateJob(ctx context.Context, rctx service.RequestContext, input service.CreateJobInput) (service.ReportJob, error)
	GetJob(ctx context.Context, rctx service.RequestContext, id string) (service.ReportJob, error)
	ListJobs(ctx context.Context, rctx service.RequestContext, reportID string) ([]service.ReportJob, error)
	RetryJob(ctx context.Context, rctx service.RequestContext, id, reason string) (service.ReportJobAttempt, error)
	ListAttempts(ctx context.Context, rctx service.RequestContext, jobID string) ([]service.ReportJobAttempt, error)
	ListEvents(ctx context.Context, rctx service.RequestContext, reportID string) ([]service.ReportEvent, error)
}

type AdminSvc interface {
	GetReportSettings(context.Context, service.RequestContext) (service.ReportSettings, error)
	UpdateReportSettings(context.Context, service.RequestContext, service.UpdateReportSettingsInput) (service.ReportSettings, error)
	GetStatisticsOverview(context.Context, service.RequestContext, int) (service.ReportStatisticsOverview, error)
	ListDailyStatistics(context.Context, service.RequestContext, int) ([]service.ReportDailyStatistic, error)
	ListOperationLogs(context.Context, service.RequestContext, service.OperationLogListFilter) (service.OperationLogListResult, error)
}

type ReportFileSvc interface {
	ListReportFiles(ctx context.Context, rctx service.RequestContext, filter service.ReportFileListFilter) (service.ReportFileListResult, error)
	CreateReportFile(ctx context.Context, rctx service.RequestContext, input service.CreateReportFileInput) (service.ReportFile, error)
	GetReportFile(ctx context.Context, rctx service.RequestContext, id string) (service.ReportFile, error)
	ReadReportFileContent(ctx context.Context, rctx service.RequestContext, id string) (service.FileContent, error)
}

const defaultMaxUploadBytes = int64(32 << 20)

type Config struct {
	Logger          *slog.Logger
	ReadyChecker    ReadyChecker
	DocumentService DocumentService
	ReportService   ReportService
	JobSvc          JobSvc
	AdminService    AdminSvc
	ReportFileSvc   ReportFileSvc
	MaxUploadBytes  int64
}

type Server struct {
	logger         *slog.Logger
	readyChecker   ReadyChecker
	documents      DocumentService
	reportService  ReportService
	jobSvc         JobSvc
	adminService   AdminSvc
	reportFileSvc  ReportFileSvc
	maxUploadBytes int64
	mux            *http.ServeMux
}

func NewServer(cfg Config) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.MaxUploadBytes <= 0 {
		cfg.MaxUploadBytes = defaultMaxUploadBytes
	}
	server := &Server{
		logger:         cfg.Logger,
		readyChecker:   cfg.ReadyChecker,
		documents:      cfg.DocumentService,
		reportService:  cfg.ReportService,
		jobSvc:         cfg.JobSvc,
		adminService:   cfg.AdminService,
		reportFileSvc:  cfg.ReportFileSvc,
		maxUploadBytes: cfg.MaxUploadBytes,
		mux:            http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /readyz", s.handleReady)
	s.mux.HandleFunc("GET /report-types", s.handleListReportTypes)
	s.mux.HandleFunc("GET /report-templates", s.handleListReportTemplates)
	s.mux.HandleFunc("POST /report-templates", s.handleCreateReportTemplate)
	s.mux.HandleFunc("GET /report-templates/{reportTemplateId}", s.handleGetReportTemplate)
	s.mux.HandleFunc("PATCH /report-templates/{reportTemplateId}", s.handleUpdateReportTemplate)
	s.mux.HandleFunc("DELETE /report-templates/{reportTemplateId}", s.handleDeleteReportTemplate)
	s.mux.HandleFunc("GET /report-templates/{reportTemplateId}/structure", s.handleGetReportTemplateStructure)
	s.mux.HandleFunc("PATCH /report-templates/{reportTemplateId}/structure", s.handleUpdateReportTemplateStructure)
	s.mux.HandleFunc("GET /report-materials", s.handleListReportMaterials)
	s.mux.HandleFunc("POST /report-materials", s.handleCreateReportMaterial)
	s.mux.HandleFunc("GET /report-materials/{materialId}", s.handleGetReportMaterial)
	s.mux.HandleFunc("DELETE /report-materials/{materialId}", s.handleDeleteReportMaterial)
	s.registerReportRoutes()
	// C-04: jobs, attempts, events.
	s.mux.HandleFunc("GET /reports/{reportId}/jobs", s.handleListJobs)
	s.mux.HandleFunc("POST /reports/{reportId}/jobs", s.handleCreateJob)
	s.mux.HandleFunc("GET /report-jobs/{jobId}", s.handleGetJob)
	s.mux.HandleFunc("GET /report-jobs/{jobId}/attempts", s.handleListAttempts)
	s.mux.HandleFunc("POST /report-jobs/{jobId}/attempts", s.handleRetryJob)
	s.mux.HandleFunc("GET /reports/{reportId}/events", s.handleListEvents)
	s.mux.HandleFunc("GET /report-files", s.handleListReportFiles)
	s.mux.HandleFunc("POST /report-files", s.handleCreateReportFile)
	s.mux.HandleFunc("GET /report-files/{reportFileId}", s.handleGetReportFile)
	s.mux.HandleFunc("GET /report-files/{reportFileId}/content", s.handleGetReportFileContent)
	s.mux.HandleFunc("GET /report-statistics/overview", s.handleGetReportStatisticsOverview)
	s.mux.HandleFunc("GET /report-statistics/daily", s.handleListReportDailyStatistics)
	s.mux.HandleFunc("GET /report-operation-logs", s.handleListReportOperationLogs)
	s.mux.HandleFunc("GET /report-settings", s.handleGetReportSettings)
	s.mux.HandleFunc("PATCH /report-settings", s.handleUpdateReportSettings)
	s.mux.HandleFunc("/", s.handleNotFound)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
	if requestID == "" {
		requestID = newRequestID()
	}
	ctx := context.WithValue(r.Context(), requestIDKey{}, requestID)
	r = r.WithContext(ctx)
	w.Header().Set("X-Request-Id", requestID)

	recorder := &statusRecorder{ResponseWriter: w}
	startedAt := time.Now()
	defer func() {
		if recovered := recover(); recovered != nil {
			s.logger.ErrorContext(ctx, "http panic recovered", "service", "document", "request_id", requestID, "operation", "http_request")
			if recorder.status == 0 {
				writeError(recorder, r, service.NewError(service.CodeInternal, "internal server error", nil))
			}
		}
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		if status >= http.StatusInternalServerError {
			s.logger.ErrorContext(ctx, "http request failed", "service", "document", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "status", status, "duration_ms", time.Since(startedAt).Milliseconds())
		}
	}()

	s.mux.ServeHTTP(recorder, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeData(w, r, http.StatusOK, map[string]string{"service": "document", "status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.readyChecker != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.readyChecker.CheckReady(ctx); err != nil {
			writeError(w, r, service.NewError(service.CodeDependency, "service is not ready", err))
			return
		}
	}
	writeData(w, r, http.StatusOK, map[string]string{"service": "document", "status": "ready"})
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, service.NewError(service.CodeNotFound, "route not found", nil))
}

type requestIDKey struct{}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

type successEnvelope struct {
	Data      any    `json:"data"`
	RequestID string `json:"requestId"`
}

type pagedEnvelope struct {
	Data      any          `json:"data"`
	Page      pageResponse `json:"page"`
	RequestID string       `json:"requestId"`
}

type errorEnvelope struct {
	Error struct {
		Code      service.Code      `json:"code"`
		Message   string            `json:"message"`
		RequestID string            `json:"requestId"`
		Fields    map[string]string `json:"fields,omitempty"`
	} `json:"error"`
}

func writeData(w http.ResponseWriter, r *http.Request, status int, value any) {
	writeJSON(w, status, successEnvelope{Data: value, RequestID: requestIDFromContext(r.Context())})
}

func writePage(w http.ResponseWriter, r *http.Request, status int, value any, page service.PageMeta) {
	writeJSON(w, status, pagedEnvelope{Data: value, Page: pageFromDomain(page), RequestID: requestIDFromContext(r.Context())})
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	appErr, ok := service.Classify(err)
	if !ok {
		appErr = service.NewError(service.CodeInternal, "internal server error", err)
	}
	var payload errorEnvelope
	payload.Error.Code = appErr.Code
	payload.Error.Message = appErr.Message
	payload.Error.RequestID = requestIDFromContext(r.Context())
	payload.Error.Fields = appErr.Fields
	writeJSON(w, statusForCode(appErr.Code), payload)
}

func statusForCode(code service.Code) int {
	switch code {
	case service.CodeValidation:
		return http.StatusBadRequest
	case service.CodeUnauthorized:
		return http.StatusUnauthorized
	case service.CodeForbidden:
		return http.StatusForbidden
	case service.CodeNotFound:
		return http.StatusNotFound
	case service.CodeConflict:
		return http.StatusConflict
	case service.CodeRateLimited:
		return http.StatusTooManyRequests
	case service.CodeDependency:
		return http.StatusBadGateway
	case service.CodeNotImplemented:
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func newRequestID() string {
	data := make([]byte, 8)
	if _, err := rand.Read(data); err != nil {
		return "req_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return "req_" + hex.EncodeToString(data)
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
