package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

func TestReportSettingsHandlersUseGatewayEnvelope(t *testing.T) {
	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	admin := &fakeAdminService{
		getSettings: func(_ context.Context, reqCtx service.RequestContext) (service.ReportSettings, error) {
			if !reqCtx.IsAdmin() {
				t.Fatalf("GET settings request context roles = %+v, want admin", reqCtx.Roles)
			}
			return service.ReportSettings{
				LLM: service.ReportSettingsModelConfig{
					Provider:       "ai-gateway",
					ProfileID:      "mp-chat",
					Model:          "gpt-test",
					TimeoutSeconds: 45,
				},
				DefaultTemplates: map[string]string{"summer_peak_inspection": "tpl-1"},
				File:             service.ReportSettingsFileDefaults{DefaultFormat: "docx", DefaultNumberingMode: "global"},
				UpdatedAt:        now,
			}, nil
		},
		updateSettings: func(ctx context.Context, reqCtx service.RequestContext, input service.UpdateReportSettingsInput) (service.ReportSettings, error) {
			if reqCtx.UserID != "admin-1" {
				t.Fatalf("UserID = %q, want admin-1", reqCtx.UserID)
			}
			if input.LLM == nil || input.LLM.ProfileID != "mp-chat-updated" {
				t.Fatalf("unexpected update input: %+v", input)
			}
			return service.ReportSettings{UpdatedAt: now}, nil
		},
	}
	server := NewServer(Config{AdminService: admin})

	getReq := httptest.NewRequest(http.MethodGet, "/report-settings", nil)
	getReq.Header.Set("X-User-Id", "admin-1")
	getReq.Header.Set("X-User-Roles", "admin")
	getReq.Header.Set("X-Request-Id", "req-get-settings")
	getRec := httptest.NewRecorder()
	server.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200: %s", getRec.Code, getRec.Body.String())
	}
	var getBody struct {
		Data struct {
			LLM struct {
				Provider       string `json:"provider"`
				ProfileID      string `json:"profileId"`
				Model          string `json:"model"`
				TimeoutSeconds int    `json:"timeoutSeconds"`
			} `json:"llm"`
			DefaultTemplates map[string]string `json:"defaultTemplates"`
			File             struct {
				DefaultFormat        string `json:"defaultFormat"`
				DefaultNumberingMode string `json:"defaultNumberingMode"`
			} `json:"file"`
		} `json:"data"`
		RequestID string `json:"requestId"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getBody); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if getBody.RequestID != "req-get-settings" || getBody.Data.LLM.ProfileID != "mp-chat" || getBody.Data.DefaultTemplates["summer_peak_inspection"] != "tpl-1" {
		t.Fatalf("unexpected GET response: %+v", getBody)
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/report-settings", strings.NewReader(`{
		"llm": {"profileId": "mp-chat-updated"},
		"defaultTemplates": {"summer_peak_inspection": "tpl-2"},
		"file": {"defaultFormat": "docx", "defaultNumberingMode": "global"}
	}`))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set("X-User-Id", "admin-1")
	patchReq.Header.Set("X-User-Roles", "admin")
	patchReq.Header.Set("X-Request-Id", "req-patch-settings")
	patchRec := httptest.NewRecorder()
	server.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d, want 200: %s", patchRec.Code, patchRec.Body.String())
	}
	if !strings.Contains(patchRec.Body.String(), `"updatedAt"`) || strings.Contains(patchRec.Body.String(), "mp-chat-updated") {
		t.Fatalf("PATCH response should only return updatedAt, got %s", patchRec.Body.String())
	}
}

func TestUpdateSettingsInputDistinguishesProfileIDPresence(t *testing.T) {
	var omitted reportSettingsPatchRequest
	if err := json.Unmarshal([]byte(`{"llm":{"timeoutSeconds":60}}`), &omitted); err != nil {
		t.Fatalf("decode omitted profile patch: %v", err)
	}
	omittedInput := updateSettingsInputFromRequest(omitted)
	if omittedInput.LLM == nil || omittedInput.LLM.ProfileIDSet {
		t.Fatalf("omitted profile input = %+v, want profileId not set", omittedInput.LLM)
	}

	var cleared reportSettingsPatchRequest
	if err := json.Unmarshal([]byte(`{"llm":{"profileId":""}}`), &cleared); err != nil {
		t.Fatalf("decode clear profile patch: %v", err)
	}
	clearedInput := updateSettingsInputFromRequest(cleared)
	if clearedInput.LLM == nil || !clearedInput.LLM.ProfileIDSet || clearedInput.LLM.ProfileID != "" {
		t.Fatalf("cleared profile input = %+v, want explicit empty profileId", clearedInput.LLM)
	}
}

func TestStatisticsAndOperationLogHandlers(t *testing.T) {
	admin := &fakeAdminService{
		getOverview: func(_ context.Context, reqCtx service.RequestContext, recentDays int) (service.ReportStatisticsOverview, error) {
			if !reqCtx.IsAdmin() {
				t.Fatalf("overview request context roles = %+v, want admin", reqCtx.Roles)
			}
			if recentDays != 30 {
				t.Fatalf("overview recentDays = %d, want 30", recentDays)
			}
			return service.ReportStatisticsOverview{
				ReportCount:     2,
				TemplateCount:   1,
				MaterialCount:   3,
				JobStatusCounts: map[string]int{"succeeded": 1},
				RecentDays:      30,
			}, nil
		},
		listDaily: func(_ context.Context, reqCtx service.RequestContext, days int) ([]service.ReportDailyStatistic, error) {
			if !reqCtx.IsAdmin() {
				t.Fatalf("daily request context roles = %+v, want admin", reqCtx.Roles)
			}
			if days != 7 {
				t.Fatalf("daily days = %d, want 7", days)
			}
			return []service.ReportDailyStatistic{{Date: "2026-06-30", CreatedCount: 1, GeneratedCount: 1}}, nil
		},
		listLogs: func(_ context.Context, reqCtx service.RequestContext, filter service.OperationLogListFilter) (service.OperationLogListResult, error) {
			if !reqCtx.IsAdmin() {
				t.Fatalf("logs request context roles = %+v, want admin", reqCtx.Roles)
			}
			if filter.Page != 2 || filter.PageSize != 5 || filter.TargetType != "report" || filter.RequestSource != "mcp" {
				t.Fatalf("unexpected log filter: %+v", filter)
			}
			return service.OperationLogListResult{
				Items: []service.OperationLog{{
					ID:               "log-1",
					OperationType:    service.OperationCreateReport,
					TargetType:       "report",
					TargetID:         "rpt-1",
					RequestID:        "req-1",
					RequestSource:    "mcp",
					ToolName:         "generate_report_outline",
					ParameterSummary: map[string]any{"reportType": "summer_peak_inspection"},
					OperationResult:  "succeeded",
					Metadata:         map[string]any{"sectionCount": 3},
					CreatedAt:        time.Date(2026, 6, 30, 11, 0, 0, 0, time.UTC),
				}},
				Page: service.PageMeta{Page: 2, PageSize: 5, Total: 9},
			}, nil
		},
	}
	server := NewServer(Config{AdminService: admin})

	overviewReq := httptest.NewRequest(http.MethodGet, "/report-statistics/overview", nil)
	overviewReq.Header.Set("X-User-Id", "admin-1")
	overviewReq.Header.Set("X-User-Roles", "admin")
	overviewRec := httptest.NewRecorder()
	server.ServeHTTP(overviewRec, overviewReq)
	if overviewRec.Code != http.StatusOK || !strings.Contains(overviewRec.Body.String(), `"reportCount":2`) {
		t.Fatalf("overview response = %d %s", overviewRec.Code, overviewRec.Body.String())
	}

	dailyReq := httptest.NewRequest(http.MethodGet, "/report-statistics/daily?days=7", nil)
	dailyReq.Header.Set("X-User-Id", "admin-1")
	dailyReq.Header.Set("X-User-Roles", "admin")
	dailyRec := httptest.NewRecorder()
	server.ServeHTTP(dailyRec, dailyReq)
	if dailyRec.Code != http.StatusOK || !strings.Contains(dailyRec.Body.String(), `"createdCount":1`) {
		t.Fatalf("daily response = %d %s", dailyRec.Code, dailyRec.Body.String())
	}

	logReq := httptest.NewRequest(http.MethodGet, "/report-operation-logs?page=2&pageSize=5&targetType=report&requestSource=mcp", nil)
	logReq.Header.Set("X-User-Id", "admin-1")
	logReq.Header.Set("X-User-Roles", "admin")
	logRec := httptest.NewRecorder()
	server.ServeHTTP(logRec, logReq)
	if logRec.Code != http.StatusOK {
		t.Fatalf("logs status = %d, want 200: %s", logRec.Code, logRec.Body.String())
	}
	if !strings.Contains(logRec.Body.String(), `"page":2`) || !strings.Contains(logRec.Body.String(), `"parameterSummary"`) {
		t.Fatalf("logs response missing page or summary: %s", logRec.Body.String())
	}
}

type fakeAdminService struct {
	getSettings    func(context.Context, service.RequestContext) (service.ReportSettings, error)
	updateSettings func(context.Context, service.RequestContext, service.UpdateReportSettingsInput) (service.ReportSettings, error)
	getOverview    func(context.Context, service.RequestContext, int) (service.ReportStatisticsOverview, error)
	listDaily      func(context.Context, service.RequestContext, int) ([]service.ReportDailyStatistic, error)
	listLogs       func(context.Context, service.RequestContext, service.OperationLogListFilter) (service.OperationLogListResult, error)
}

func (f *fakeAdminService) GetReportSettings(ctx context.Context, reqCtx service.RequestContext) (service.ReportSettings, error) {
	if f.getSettings == nil {
		return service.ReportSettings{}, service.NewError(service.CodeInternal, "fake method not configured", nil)
	}
	return f.getSettings(ctx, reqCtx)
}

func (f *fakeAdminService) UpdateReportSettings(ctx context.Context, reqCtx service.RequestContext, input service.UpdateReportSettingsInput) (service.ReportSettings, error) {
	if f.updateSettings == nil {
		return service.ReportSettings{}, service.NewError(service.CodeInternal, "fake method not configured", nil)
	}
	return f.updateSettings(ctx, reqCtx, input)
}

func (f *fakeAdminService) GetStatisticsOverview(ctx context.Context, reqCtx service.RequestContext, recentDays int) (service.ReportStatisticsOverview, error) {
	if f.getOverview == nil {
		return service.ReportStatisticsOverview{}, service.NewError(service.CodeInternal, "fake method not configured", nil)
	}
	return f.getOverview(ctx, reqCtx, recentDays)
}

func (f *fakeAdminService) ListDailyStatistics(ctx context.Context, reqCtx service.RequestContext, days int) ([]service.ReportDailyStatistic, error) {
	if f.listDaily == nil {
		return nil, service.NewError(service.CodeInternal, "fake method not configured", nil)
	}
	return f.listDaily(ctx, reqCtx, days)
}

func (f *fakeAdminService) ListOperationLogs(ctx context.Context, reqCtx service.RequestContext, filter service.OperationLogListFilter) (service.OperationLogListResult, error) {
	if f.listLogs == nil {
		return service.OperationLogListResult{}, service.NewError(service.CodeInternal, "fake method not configured", nil)
	}
	return f.listLogs(ctx, reqCtx, filter)
}
