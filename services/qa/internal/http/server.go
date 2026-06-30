package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/http/middleware"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
)

var streamHeartbeatInterval = 15 * time.Second

type QAService interface {
	CreateConversation(context.Context, string, string) (service.Conversation, error)
	ListConversations(context.Context, string, service.ConversationListOptions) (service.Page[service.Conversation], error)
	GetConversation(context.Context, string, string) (service.Conversation, error)
	UpdateConversation(context.Context, string, string, string, string) (service.Conversation, error)
	DeleteConversation(context.Context, string, string) error
	ListMessages(context.Context, string, string, service.MessageListOptions) (service.Page[service.Message], error)
	Ask(context.Context, string, string, service.AskInput, service.ProgressObserver) (service.AskResult, error)
}

type ResourceService interface {
	GetResponseRun(context.Context, string, string) (service.ResponseRun, error)
	CancelResponseRun(context.Context, string, string) (service.ResponseRun, error)
	ListStreamEvents(context.Context, string, string, string, int) ([]service.StreamEvent, error)
	ListMessageCitations(context.Context, string, string) ([]service.Citation, error)
	GetCitation(context.Context, string, string) (service.Citation, error)
	LookupCitations(context.Context, string, []string) ([]service.Citation, error)
	ListToolCalls(context.Context, string, string) ([]service.AgentToolCall, error)
	GetActiveQAConfigVersion(context.Context) (service.QAConfigVersion, error)
	CreateQAConfigVersion(context.Context, string, service.CreateQAConfigVersionInput) (service.QAConfigVersion, error)
	GetActiveLLMConfigVersion(context.Context) (service.LLMConfigVersion, error)
	CreateLLMConfigVersion(context.Context, string, service.CreateLLMConfigVersionInput) (service.LLMConfigVersion, error)
	TestLLMConnection(context.Context, string, service.LLMProfileTestInput) (service.LLMProfileTestResult, error)
	CreateRetrievalTestRun(context.Context, string, service.RetrievalTestInput) (service.RetrievalTestRun, error)
	GetRetrievalTestRun(context.Context, string, string) (service.RetrievalTestRun, error)
	GetMetricsOverview(context.Context, int) (service.MetricsOverview, error)
	GetMetricsTrend(context.Context, int) (service.MetricsTrend, error)
	GetTopQueries(context.Context, int, int) ([]service.TopQuery, error)
	GetIntentDistribution(context.Context, int) ([]service.IntentDistribution, error)
}

type SettingsService interface {
	GetSettings(context.Context) (service.QASettings, error)
	UpdateSettings(context.Context, string, string, service.UpdateQASettingsInput) (service.QASettings, error)
	ListMCPServers(context.Context) ([]service.MCPServer, error)
	CreateMCPServer(context.Context, string, string, service.MCPServerInput) (service.MCPServer, error)
	UpdateMCPServer(context.Context, string, string, string, service.MCPServerPatch) (service.MCPServer, error)
	DeleteMCPServer(context.Context, string, string, string) error
	TestLLMConnection(context.Context, service.LLMConnectionTestInput) (service.LLMConnectionTestResult, error)
	TestMCPConnection(context.Context, service.MCPConnectionTestInput) (service.MCPConnectionTestResult, error)
}

type Config struct {
	MaxRequestBytes int64
	Logger          *slog.Logger
	Ready           func(context.Context) error
	AdminUserIDs    []string
	SettingsOpen    bool
	ServiceToken    string
}

type Server struct {
	qa              QAService
	settings        SettingsService
	resources       ResourceService
	maxRequestBytes int64
	logger          *slog.Logger
	ready           func(context.Context) error
	adminUserIDs    map[string]struct{}
	settingsOpen    bool
	serviceToken    string
	mux             *http.ServeMux
}

func NewServer(qa QAService, settings SettingsService, resources ResourceService, cfg Config) (*Server, error) {
	if qa == nil || settings == nil || resources == nil || strings.TrimSpace(cfg.ServiceToken) == "" {
		return nil, errors.New("QA, settings, resource services and service token are required")
	}
	if cfg.MaxRequestBytes <= 0 {
		cfg.MaxRequestBytes = 1 << 20
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	admins := make(map[string]struct{}, len(cfg.AdminUserIDs))
	for _, id := range cfg.AdminUserIDs {
		admins[id] = struct{}{}
	}
	s := &Server{
		qa: qa, settings: settings, resources: resources, maxRequestBytes: cfg.MaxRequestBytes,
		logger: cfg.Logger, ready: cfg.Ready, adminUserIDs: admins,
		settingsOpen: cfg.SettingsOpen, serviceToken: cfg.ServiceToken, mux: http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /readyz", s.handleReady)
	s.mux.HandleFunc("POST /internal/v1/qa-sessions", s.handleCreateConversation)
	s.mux.HandleFunc("GET /internal/v1/qa-sessions", s.handleListConversations)
	s.mux.HandleFunc("GET /internal/v1/qa-sessions/{sessionId}", s.handleGetConversation)
	s.mux.HandleFunc("PATCH /internal/v1/qa-sessions/{sessionId}", s.handleUpdateConversation)
	s.mux.HandleFunc("DELETE /internal/v1/qa-sessions/{sessionId}", s.handleDeleteConversation)
	s.mux.HandleFunc("GET /internal/v1/qa-sessions/{sessionId}/messages", s.handleListMessages)
	s.mux.HandleFunc("POST /internal/v1/qa-sessions/{sessionId}/messages", s.handleAsk)
	s.registerResourceRoutes()
	s.mux.HandleFunc("/", s.handleNotFound)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	middleware.RequestLog(s.logger, http.HandlerFunc(s.dispatch)).ServeHTTP(w, r)
}

func (s *Server) dispatch(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.RequestIDFromContext(r.Context())
	ctx := service.WithRequestID(r.Context(), requestID)
	if roles := strings.TrimSpace(r.Header.Get("X-User-Roles")); roles != "" {
		ctx = service.WithUserRoles(ctx, roles)
	}
	if perms := strings.TrimSpace(r.Header.Get("X-User-Permissions")); perms != "" {
		ctx = service.WithUserPermissions(ctx, perms)
	}
	r = r.WithContext(ctx)
	if strings.HasPrefix(r.URL.Path, "/internal/v1/") && !secureTokenEqual(r.Header.Get("X-Service-Token"), s.serviceToken) {
		writeError(w, r, service.NewError(service.CodeUnauthorized, "service authentication required", nil))
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			s.logger.ErrorContext(ctx, "http panic recovered",
				"service", "qa",
				"request_id", requestID,
				"operation", "http_request",
				"status", "failed",
			)
			writeError(w, r, service.NewError(service.CodeInternal, "internal server error", nil))
		}
	}()
	s.mux.ServeHTTP(w, r)
}

func secureTokenEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	var result byte
	for index := range left {
		result |= left[index] ^ right[index]
	}
	return result == 0
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeData(w, r, http.StatusOK, map[string]string{"service": "qa", "status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.ready != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.ready(ctx); err != nil {
			writeError(w, r, service.NewError(service.CodeDependency, "service is not ready", err))
			return
		}
	}
	writeData(w, r, http.StatusOK, map[string]string{"service": "qa", "status": "ready"})
}

func (s *Server) handleCreateConversation(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}
	var payload struct {
		Title string `json:"title"`
	}
	if r.ContentLength != 0 {
		if err := s.decodeJSON(w, r, &payload); err != nil {
			writeError(w, r, err)
			return
		}
	}
	conversation, err := s.qa.CreateConversation(r.Context(), userID, payload.Title)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, conversation)
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}
	options, err := conversationListOptions(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.qa.ListConversations(r.Context(), userID, options)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writePage(w, r, http.StatusOK, result)
}

func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}
	conversation, err := s.qa.GetConversation(r.Context(), userID, r.PathValue("sessionId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, conversation)
}

func (s *Server) handleUpdateConversation(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}
	var payload struct {
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	if err := s.decodeJSON(w, r, &payload); err != nil {
		writeError(w, r, err)
		return
	}
	conversation, err := s.qa.UpdateConversation(r.Context(), userID, r.PathValue("sessionId"), payload.Title, payload.Status)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, conversation)
}

func (s *Server) handleDeleteConversation(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}
	if err := s.qa.DeleteConversation(r.Context(), userID, r.PathValue("sessionId")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}
	options, err := messageListOptions(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.qa.ListMessages(r.Context(), userID, r.PathValue("sessionId"), options)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writePage(w, r, http.StatusOK, result)
}

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	if acceptsSSE(r) {
		s.handleAskStream(w, r)
		return
	}
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}
	var input service.AskInput
	if err := s.decodeJSON(w, r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.qa.Ask(r.Context(), userID, r.PathValue("sessionId"), input, nil)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, result)
}

func (s *Server) handleAskStream(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}
	var input service.AskInput
	if err := s.decodeJSON(w, r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, service.NewError(service.CodeInternal, "streaming is unavailable", nil))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	var streamMu sync.Mutex
	sentError := false
	writeStream := func(event string, sequence int, payload any) {
		streamMu.Lock()
		defer streamMu.Unlock()
		writeSSE(w, flusher, event, sequence, payload)
	}
	stopHeartbeat := startStreamHeartbeat(r.Context(), writeStream)
	defer stopHeartbeat()

	observe := func(event service.ProgressEvent) {
		streamMu.Lock()
		defer streamMu.Unlock()
		if event.Type == "error" {
			sentError = true
		}
		writeSSE(w, flusher, event.Type, event.Sequence, event.Payload)
	}
	_, err := s.qa.Ask(r.Context(), userID, r.PathValue("sessionId"), input, observe)
	if err != nil {
		appErr, ok := service.Classify(err)
		if !ok {
			appErr = service.NewError(service.CodeInternal, "answer generation failed", err)
		}
		streamMu.Lock()
		defer streamMu.Unlock()
		if !sentError {
			writeSSE(w, flusher, "error", 0, map[string]any{"code": appErr.Code, "message": appErr.Message, "requestId": requestIDFromContext(r.Context())})
		}
		return
	}
}

func startStreamHeartbeat(ctx context.Context, write func(string, int, any)) func() {
	if streamHeartbeatInterval <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(streamHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				write("heartbeat", 0, map[string]any{"status": "alive"})
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}

func acceptsSSE(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream")
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, service.NewError(service.CodeNotFound, "route not found", nil))
}

func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxRequestBytes)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return service.ValidationError(map[string]string{"body": "must be a valid JSON object"})
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return service.ValidationError(map[string]string{"body": "must contain only one JSON object"})
	}
	return nil
}

func userIDFromRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID := strings.TrimSpace(r.Header.Get("X-User-Id"))
	if userID == "" {
		writeError(w, r, service.NewError(service.CodeUnauthorized, "authentication required", nil))
		return "", false
	}
	return userID, true
}

func pagination(r *http.Request, defaultPageSize int) (int, int, error) {
	page, pageSize := 1, defaultPageSize
	var err error
	if raw := r.URL.Query().Get("page"); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil || page <= 0 {
			return 0, 0, service.ValidationError(map[string]string{"page": "must be a positive integer"})
		}
	}
	if raw := r.URL.Query().Get("pageSize"); raw != "" {
		pageSize, err = strconv.Atoi(raw)
		if err != nil || pageSize <= 0 || pageSize > 100 {
			return 0, 0, service.ValidationError(map[string]string{"pageSize": "must be between 1 and 100"})
		}
	}
	return page, pageSize, nil
}

func conversationListOptions(r *http.Request) (service.ConversationListOptions, error) {
	page, pageSize, err := pagination(r, 20)
	if err != nil {
		return service.ConversationListOptions{}, err
	}
	return service.ConversationListOptions{
		Page:     page,
		PageSize: pageSize,
		Status:   r.URL.Query().Get("status"),
		Sort:     r.URL.Query().Get("sort"),
	}, nil
}

func messageListOptions(r *http.Request) (service.MessageListOptions, error) {
	page, pageSize, err := pagination(r, 50)
	if err != nil {
		return service.MessageListOptions{}, err
	}
	includeThinking, err := boolQuery(r, "includeThinking", true)
	if err != nil {
		return service.MessageListOptions{}, err
	}
	includeCitations, err := boolQuery(r, "includeCitations", true)
	if err != nil {
		return service.MessageListOptions{}, err
	}
	return service.MessageListOptions{
		Page:             page,
		PageSize:         pageSize,
		IncludeThinking:  includeThinking,
		IncludeCitations: includeCitations,
	}, nil
}

func boolQuery(r *http.Request, name string, defaultValue bool) (bool, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, service.ValidationError(map[string]string{name: "must be a boolean"})
	}
	return value, nil
}

func requestIDFromContext(ctx context.Context) string {
	return service.RequestIDFromContext(ctx)
}

type errorEnvelope struct {
	Error struct {
		Code      service.Code      `json:"code"`
		Message   string            `json:"message"`
		RequestID string            `json:"requestId"`
		Fields    map[string]string `json:"fields,omitempty"`
	} `json:"error"`
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
	case service.CodeUnsupportedIntent:
		return http.StatusUnprocessableEntity
	case service.CodeDependency:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

type successEnvelope struct {
	Data      any    `json:"data"`
	RequestID string `json:"requestId"`
}
type pageEnvelope struct {
	Data any `json:"data"`
	Page struct {
		Page     int `json:"page"`
		PageSize int `json:"pageSize"`
		Total    int `json:"total"`
	} `json:"page"`
	RequestID string `json:"requestId"`
}

func writeData(w http.ResponseWriter, r *http.Request, status int, value any) {
	requestID := ""
	if r != nil {
		requestID = requestIDFromContext(r.Context())
	}
	writeJSON(w, status, successEnvelope{Data: value, RequestID: requestID})
}
func writePage[T any](w http.ResponseWriter, r *http.Request, status int, value service.Page[T]) {
	body := pageEnvelope{Data: value.Items, RequestID: requestIDFromContext(r.Context())}
	body.Page.Page = value.Page
	body.Page.PageSize = value.PageSize
	body.Page.Total = value.Total
	writeJSON(w, status, body)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeSSE(w io.Writer, flusher http.Flusher, event string, sequence int, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		return
	}
	if sequence > 0 {
		_, _ = fmt.Fprintf(w, "event: %s\nid: %d\ndata: %s\n\n", event, sequence, payload)
	} else {
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	}
	flusher.Flush()
}
