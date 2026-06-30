package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

func (s *Server) requireAdminService(w http.ResponseWriter, r *http.Request) bool {
	if s.adminService != nil {
		return true
	}
	writeError(w, r, service.NewError(service.CodeDependency, "admin service is not configured", nil))
	return false
}

func (s *Server) handleGetReportSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminService(w, r) {
		return
	}
	settings, err := s.adminService.GetReportSettings(r.Context(), s.requestContext(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, reportSettingsFromDomain(settings))
}

func (s *Server) handleUpdateReportSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminService(w, r) {
		return
	}
	var payload reportSettingsPatchRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	updated, err := s.adminService.UpdateReportSettings(r.Context(), s.requestContext(r), updateSettingsInputFromRequest(payload))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, updatedAtResponse{UpdatedAt: updated.UpdatedAt})
}

func (s *Server) handleGetReportStatisticsOverview(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminService(w, r) {
		return
	}
	overview, err := s.adminService.GetStatisticsOverview(r.Context(), s.requestContext(r), service.DefaultReportStatisticsDays)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, overviewFromDomain(overview))
}

func (s *Server) handleListReportDailyStatistics(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminService(w, r) {
		return
	}
	days, err := parsePositiveIntQuery(r, "days")
	if err != nil {
		writeError(w, r, err)
		return
	}
	items, err := s.adminService.ListDailyStatistics(r.Context(), s.requestContext(r), days)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := make([]reportDailyStatisticResponse, len(items))
	for i, item := range items {
		out[i] = dailyStatisticFromDomain(item)
	}
	writeData(w, r, http.StatusOK, out)
}

func (s *Server) handleListReportOperationLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminService(w, r) {
		return
	}
	page, pageSize, err := parsePage(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	q := r.URL.Query()
	result, err := s.adminService.ListOperationLogs(r.Context(), s.requestContext(r), service.OperationLogListFilter{
		Page:          page,
		PageSize:      pageSize,
		TargetType:    strings.TrimSpace(q.Get("targetType")),
		TargetID:      strings.TrimSpace(q.Get("targetId")),
		OperationType: strings.TrimSpace(q.Get("operationType")),
		RequestID:     strings.TrimSpace(q.Get("requestId")),
		RequestSource: strings.TrimSpace(q.Get("requestSource")),
		ToolName:      strings.TrimSpace(q.Get("toolName")),
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	items := make([]operationLogResponse, len(result.Items))
	for i, item := range result.Items {
		items[i] = operationLogFromDomain(item)
	}
	writePage(w, r, http.StatusOK, items, result.Page)
}

type reportSettingsPatchRequest struct {
	LLM              *reportSettingsModelRequest `json:"llm"`
	DefaultTemplates *map[string]string          `json:"defaultTemplates"`
	File             *reportSettingsFileRequest  `json:"file"`
}

type reportSettingsModelRequest struct {
	Provider       string  `json:"provider"`
	ProfileID      *string `json:"profileId"`
	Model          string  `json:"model"`
	TimeoutSeconds int     `json:"timeoutSeconds"`
}

type reportSettingsFileRequest struct {
	DefaultFormat         string         `json:"defaultFormat"`
	DefaultNumberingMode  string         `json:"defaultNumberingMode"`
	DefaultStyleProfileID *string        `json:"defaultStyleProfileId"`
	Extra                 map[string]any `json:"-"`
}

type reportSettingsResponse struct {
	LLM              reportSettingsModelResponse `json:"llm"`
	DefaultTemplates map[string]string           `json:"defaultTemplates"`
	File             reportSettingsFileResponse  `json:"file"`
}

type reportSettingsModelResponse struct {
	Provider       string `json:"provider,omitempty"`
	ProfileID      string `json:"profileId,omitempty"`
	Model          string `json:"model,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}

type reportSettingsFileResponse struct {
	DefaultFormat         string         `json:"defaultFormat,omitempty"`
	DefaultNumberingMode  string         `json:"defaultNumberingMode,omitempty"`
	DefaultStyleProfileID string         `json:"defaultStyleProfileId,omitempty"`
	Extra                 map[string]any `json:"-"`
}

type updatedAtResponse struct {
	UpdatedAt time.Time `json:"updatedAt"`
}

type reportStatisticsOverviewResponse struct {
	ReportCount     int            `json:"reportCount"`
	TemplateCount   int            `json:"templateCount"`
	MaterialCount   int            `json:"materialCount"`
	JobStatusCounts map[string]int `json:"jobStatusCounts,omitempty"`
	RecentDays      int            `json:"recentDays,omitempty"`
}

type reportDailyStatisticResponse struct {
	Date           string `json:"date"`
	ReportType     string `json:"reportType,omitempty"`
	CreatedCount   int    `json:"createdCount"`
	GeneratedCount int    `json:"generatedCount"`
	FailedCount    int    `json:"failedCount"`
	ExportedCount  int    `json:"exportedCount"`
}

type operationLogResponse struct {
	ID               string         `json:"id"`
	OperatorID       string         `json:"operatorId,omitempty"`
	OperatorName     string         `json:"operatorName,omitempty"`
	OperationType    string         `json:"operationType"`
	TargetType       string         `json:"targetType"`
	TargetID         string         `json:"targetId"`
	RequestID        string         `json:"requestId,omitempty"`
	RequestSource    string         `json:"requestSource,omitempty"`
	ToolName         string         `json:"toolName,omitempty"`
	ParameterSummary map[string]any `json:"parameterSummary,omitempty"`
	OperationResult  string         `json:"operationResult"`
	ErrorMessage     string         `json:"errorMessage,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"createdAt"`
}

func updateSettingsInputFromRequest(payload reportSettingsPatchRequest) service.UpdateReportSettingsInput {
	input := service.UpdateReportSettingsInput{DefaultTemplates: payload.DefaultTemplates}
	if payload.LLM != nil {
		input.LLM = &service.ReportSettingsModelConfig{
			Provider:       payload.LLM.Provider,
			Model:          payload.LLM.Model,
			TimeoutSeconds: payload.LLM.TimeoutSeconds,
		}
		if payload.LLM.ProfileID != nil {
			input.LLM.ProfileID = *payload.LLM.ProfileID
			input.LLM.ProfileIDSet = true
		}
	}
	if payload.File != nil {
		input.File = &service.ReportSettingsFileDefaults{
			DefaultFormat:        payload.File.DefaultFormat,
			DefaultNumberingMode: payload.File.DefaultNumberingMode,
			Extra:                payload.File.Extra,
		}
		if payload.File.DefaultStyleProfileID != nil {
			input.File.DefaultStyleProfileID = *payload.File.DefaultStyleProfileID
			input.File.DefaultStyleProfileIDSet = true
		}
	}
	return input
}

func reportSettingsFromDomain(value service.ReportSettings) reportSettingsResponse {
	defaultTemplates := value.DefaultTemplates
	if defaultTemplates == nil {
		defaultTemplates = map[string]string{}
	}
	return reportSettingsResponse{
		LLM: reportSettingsModelResponse{
			Provider:       value.LLM.Provider,
			ProfileID:      value.LLM.ProfileID,
			Model:          value.LLM.Model,
			TimeoutSeconds: value.LLM.TimeoutSeconds,
		},
		DefaultTemplates: defaultTemplates,
		File: reportSettingsFileResponse{
			DefaultFormat:         value.File.DefaultFormat,
			DefaultNumberingMode:  value.File.DefaultNumberingMode,
			DefaultStyleProfileID: value.File.DefaultStyleProfileID,
			Extra:                 value.File.Extra,
		},
	}
}

func overviewFromDomain(value service.ReportStatisticsOverview) reportStatisticsOverviewResponse {
	return reportStatisticsOverviewResponse{
		ReportCount:     value.ReportCount,
		TemplateCount:   value.TemplateCount,
		MaterialCount:   value.MaterialCount,
		JobStatusCounts: value.JobStatusCounts,
		RecentDays:      value.RecentDays,
	}
}

func dailyStatisticFromDomain(value service.ReportDailyStatistic) reportDailyStatisticResponse {
	return reportDailyStatisticResponse{
		Date:           value.Date,
		ReportType:     value.ReportType,
		CreatedCount:   value.CreatedCount,
		GeneratedCount: value.GeneratedCount,
		FailedCount:    value.FailedCount,
		ExportedCount:  value.ExportedCount,
	}
}

func operationLogFromDomain(value service.OperationLog) operationLogResponse {
	return operationLogResponse{
		ID:               value.ID,
		OperatorID:       value.OperatorID,
		OperatorName:     value.OperatorName,
		OperationType:    value.OperationType,
		TargetType:       value.TargetType,
		TargetID:         value.TargetID,
		RequestID:        value.RequestID,
		RequestSource:    value.RequestSource,
		ToolName:         value.ToolName,
		ParameterSummary: value.ParameterSummary,
		OperationResult:  value.OperationResult,
		ErrorMessage:     value.ErrorMessage,
		Metadata:         value.Metadata,
		CreatedAt:        value.CreatedAt,
	}
}
