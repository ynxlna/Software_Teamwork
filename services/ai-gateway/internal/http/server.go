package httpapi

import (
	"context"
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

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/ai-gateway/internal/middleware"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/ai-gateway/internal/service"
)

type ModelProfileService interface {
	ListModelProfiles(context.Context, service.ListModelProfilesFilter) ([]service.ModelProfile, error)
	GetModelProfile(context.Context, string) (service.ModelProfile, error)
	CreateModelProfile(context.Context, service.RequestContext, service.CreateModelProfileInput) (service.ModelProfile, error)
	UpdateModelProfile(context.Context, service.RequestContext, service.UpdateModelProfileInput) (service.ModelProfile, error)
	DeleteModelProfile(context.Context, service.RequestContext, string) error
	CheckReady(context.Context) (service.Readiness, error)
}

type Config struct {
	Logger          *slog.Logger
	Profiles        ModelProfileService
	Authenticator   *middleware.ServiceTokenAuthenticator
	MaxRequestBytes int64
}

type Server struct {
	logger          *slog.Logger
	profiles        ModelProfileService
	authenticator   *middleware.ServiceTokenAuthenticator
	maxRequestBytes int64
	mux             *http.ServeMux
}

const defaultMaxRequestBytes = int64(1 << 20)

func NewServer(cfg Config) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.MaxRequestBytes <= 0 {
		cfg.MaxRequestBytes = defaultMaxRequestBytes
	}
	server := &Server{
		logger:          cfg.Logger,
		profiles:        cfg.Profiles,
		authenticator:   cfg.Authenticator,
		maxRequestBytes: cfg.MaxRequestBytes,
		mux:             http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /readyz", s.handleReady)
	s.mux.HandleFunc("GET /internal/v1/model-profiles", s.handleListModelProfiles)
	s.mux.HandleFunc("POST /internal/v1/model-profiles", s.handleCreateModelProfile)
	s.mux.HandleFunc("GET /internal/v1/model-profiles/{profileId}", s.handleGetModelProfile)
	s.mux.HandleFunc("PATCH /internal/v1/model-profiles/{profileId}", s.handleUpdateModelProfile)
	s.mux.HandleFunc("DELETE /internal/v1/model-profiles/{profileId}", s.handleDeleteModelProfile)
	s.mux.HandleFunc("POST /internal/v1/chat/completions", s.handleModelInvocationNotImplemented)
	s.mux.HandleFunc("POST /internal/v1/embeddings", s.handleModelInvocationNotImplemented)
	s.mux.HandleFunc("POST /internal/v1/rerankings", s.handleModelInvocationNotImplemented)
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
			s.logger.ErrorContext(ctx, "http panic recovered", "service", "ai-gateway", "request_id", requestID, "operation", "http_request")
			if recorder.status == 0 {
				writeError(recorder, r, service.NewError(service.CodeInternal, "internal server error", nil))
			}
		}
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		if status >= http.StatusInternalServerError {
			s.logger.ErrorContext(ctx, "http request failed", "service", "ai-gateway", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "status", status, "duration_ms", time.Since(startedAt).Milliseconds())
		}
	}()

	s.mux.ServeHTTP(recorder, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeData(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.profiles == nil {
		writeError(w, r, service.DependencyError("profile service is not configured", nil))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	ready, err := s.profiles.CheckReady(ctx)
	if err != nil {
		writeError(w, r, err)
		return
	}
	status := http.StatusOK
	if ready.Status != "ok" {
		status = http.StatusServiceUnavailable
	}
	writeData(w, r, status, ready)
}

func (s *Server) handleListModelProfiles(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeInternal(w, r) || !s.requireProfiles(w, r) {
		return
	}
	filter, ok := parseListFilter(w, r)
	if !ok {
		return
	}
	items, err := s.profiles.ListModelProfiles(r.Context(), filter)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, profilesFromDomain(items))
}

func (s *Server) handleCreateModelProfile(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.internalContext(w, r)
	if !ok || !s.requireProfiles(w, r) {
		return
	}
	var payload createModelProfileRequest
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	created, err := s.profiles.CreateModelProfile(r.Context(), reqCtx, createInputFromRequest(payload))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, profileFromDomain(created))
}

func (s *Server) handleGetModelProfile(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeInternal(w, r) || !s.requireProfiles(w, r) {
		return
	}
	profile, err := s.profiles.GetModelProfile(r.Context(), r.PathValue("profileId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, profileFromDomain(profile))
}

func (s *Server) handleUpdateModelProfile(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.internalContext(w, r)
	if !ok || !s.requireProfiles(w, r) {
		return
	}
	var payload updateModelProfileRequest
	if !s.decodeJSON(w, r, &payload) {
		return
	}
	updated, err := s.profiles.UpdateModelProfile(r.Context(), reqCtx, updateInputFromRequest(r.PathValue("profileId"), payload))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, profileFromDomain(updated))
}

func (s *Server) handleDeleteModelProfile(w http.ResponseWriter, r *http.Request) {
	reqCtx, ok := s.internalContext(w, r)
	if !ok || !s.requireProfiles(w, r) {
		return
	}
	if err := s.profiles.DeleteModelProfile(r.Context(), reqCtx, r.PathValue("profileId")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleModelInvocationNotImplemented(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeModelInvocation(w, r) {
		return
	}
	writeOpenAIError(w, http.StatusNotImplemented, "model invocation is not implemented", "not_implemented_error", "not_implemented")
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, service.NotFoundError("route not found", nil))
}

func (s *Server) internalContext(w http.ResponseWriter, r *http.Request) (service.RequestContext, bool) {
	if !s.authorizeInternal(w, r) {
		return service.RequestContext{}, false
	}
	reqCtx := service.RequestContext{
		RequestID:     requestIDFromContext(r.Context()),
		CallerService: strings.TrimSpace(r.Header.Get("X-Caller-Service")),
		UserID:        strings.TrimSpace(r.Header.Get("X-User-Id")),
	}
	return reqCtx, true
}

func (s *Server) authorizeInternal(w http.ResponseWriter, r *http.Request) bool {
	if s.authenticator == nil || !s.authenticator.Authenticate(r.Header.Get("X-Service-Token")) {
		writeError(w, r, service.UnauthorizedError())
		return false
	}
	callerService := strings.TrimSpace(r.Header.Get("X-Caller-Service"))
	if callerService == "" {
		writeError(w, r, service.UnauthorizedError())
		return false
	}
	if !isAllowedCallerService(callerService) {
		writeError(w, r, service.NewError(service.CodeForbidden, "caller service is not allowed", nil))
		return false
	}
	return true
}

func (s *Server) authorizeModelInvocation(w http.ResponseWriter, r *http.Request) bool {
	if s.authenticator == nil || !s.authenticator.Authenticate(r.Header.Get("X-Service-Token")) {
		writeOpenAIError(w, http.StatusUnauthorized, "authentication is required", "authentication_error", "unauthorized")
		return false
	}
	callerService := strings.TrimSpace(r.Header.Get("X-Caller-Service"))
	if callerService == "" {
		writeOpenAIError(w, http.StatusUnauthorized, "authentication is required", "authentication_error", "unauthorized")
		return false
	}
	if !isAllowedCallerService(callerService) {
		writeOpenAIError(w, http.StatusForbidden, "caller service is not allowed", "permission_error", "forbidden")
		return false
	}
	return true
}

func isAllowedCallerService(callerService string) bool {
	switch callerService {
	case "gateway", "qa", "knowledge", "document", "auth", "file":
		return true
	default:
		return false
	}
}

func (s *Server) requireProfiles(w http.ResponseWriter, r *http.Request) bool {
	if s.profiles != nil {
		return true
	}
	writeError(w, r, service.DependencyError("profile service is not configured", nil))
	return false
}

func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, s.maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, r, service.ValidationError(map[string]string{"body": "must be a valid JSON object"}))
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, r, service.ValidationError(map[string]string{"body": "must contain only one JSON object"}))
		return false
	}
	return true
}

func parseListFilter(w http.ResponseWriter, r *http.Request) (service.ListModelProfilesFilter, bool) {
	var filter service.ListModelProfilesFilter
	if raw := strings.TrimSpace(r.URL.Query().Get("purpose")); raw != "" {
		purpose := service.Purpose(raw)
		filter.Purpose = &purpose
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("enabled")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, r, service.ValidationError(map[string]string{"enabled": "must be a boolean"}))
			return service.ListModelProfilesFilter{}, false
		}
		filter.Enabled = &value
	}
	return filter, true
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

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type openAIErrorEnvelope struct {
	Error openAIErrorBody `json:"error"`
}

type errorBody struct {
	Code      service.Code      `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"requestId"`
	Fields    map[string]string `json:"fields,omitempty"`
}

type openAIErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

func writeData(w http.ResponseWriter, r *http.Request, status int, value any) {
	writeJSON(w, status, successEnvelope{Data: value, RequestID: requestIDFromContext(r.Context())})
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	appErr, ok := service.Classify(err)
	if !ok {
		appErr = service.NewError(service.CodeInternal, "internal server error", err)
	}
	writeJSON(w, statusForCode(appErr.Code), errorEnvelope{Error: errorBody{
		Code:      appErr.Code,
		Message:   appErr.Message,
		RequestID: requestIDFromContext(r.Context()),
		Fields:    appErr.Fields,
	}})
}

func writeOpenAIError(w http.ResponseWriter, status int, message, errorType, code string) {
	writeJSON(w, status, openAIErrorEnvelope{Error: openAIErrorBody{
		Message: message,
		Type:    errorType,
		Code:    code,
	}})
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
