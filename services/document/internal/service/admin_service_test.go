package service

import (
	"context"
	"testing"
	"time"
)

func TestUpdateReportSettingsRequiresAdminAndValidatesReferences(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC)
	repo := newFakeAdminRepository()
	repo.reportTypes["summer_peak_inspection"] = true
	repo.templates["tpl-enabled"] = ReportTemplate{
		ID:           "tpl-enabled",
		ReportType:   "summer_peak_inspection",
		Enabled:      true,
		TemplateName: "enabled",
	}
	profiles := &fakeProfileValidator{
		profiles: map[string]ModelProfileReference{
			"mp-chat": {ID: "mp-chat", Purpose: "chat", Provider: "openai", Model: "gpt-test", Enabled: true, TimeoutSeconds: 45},
		},
	}
	svc := NewAdminService(repo, profiles)
	svc.clock = func() time.Time { return now }

	_, err := svc.UpdateReportSettings(ctx, RequestContext{UserID: "user-1"}, UpdateReportSettingsInput{
		LLM: &ReportSettingsModelConfig{ProfileID: "mp-chat"},
	})
	if code := errorCode(t, err); code != CodeForbidden {
		t.Fatalf("non-admin error code = %q, want %q", code, CodeForbidden)
	}

	_, err = svc.UpdateReportSettings(ctx, RequestContext{UserID: "admin-1", Roles: []string{"admin"}}, UpdateReportSettingsInput{
		LLM: &ReportSettingsModelConfig{ProfileID: "missing-profile"},
	})
	if code := errorCode(t, err); code != CodeValidation {
		t.Fatalf("missing profile error code = %q, want %q", code, CodeValidation)
	}

	_, err = svc.UpdateReportSettings(ctx, RequestContext{UserID: "admin-1", Roles: []string{"admin"}}, UpdateReportSettingsInput{
		DefaultTemplates: &map[string]string{"summer_peak_inspection": "missing-template"},
	})
	if code := errorCode(t, err); code != CodeValidation {
		t.Fatalf("missing template error code = %q, want %q", code, CodeValidation)
	}

	updated, err := svc.UpdateReportSettings(ctx, RequestContext{UserID: "admin-1", Roles: []string{"admin"}, RequestID: "req-settings"}, UpdateReportSettingsInput{
		LLM:              &ReportSettingsModelConfig{ProfileID: "mp-chat"},
		DefaultTemplates: &map[string]string{"summer_peak_inspection": "tpl-enabled"},
		File:             &ReportSettingsFileDefaults{DefaultFormat: "docx", DefaultNumberingMode: "global"},
	})
	if err != nil {
		t.Fatalf("UpdateReportSettings() error = %v", err)
	}
	if updated.LLM.Provider != "ai-gateway" || updated.LLM.Model != "gpt-test" || updated.LLM.TimeoutSeconds != 45 {
		t.Fatalf("settings llm not resolved from profile: %+v", updated.LLM)
	}
	if updated.DefaultTemplates["summer_peak_inspection"] != "tpl-enabled" {
		t.Fatalf("default template map = %+v", updated.DefaultTemplates)
	}
	if !updated.UpdatedAt.Equal(now) {
		t.Fatalf("UpdatedAt = %v, want %v", updated.UpdatedAt, now)
	}
	if len(repo.logs) != 1 || repo.logs[0].OperationType != OperationUpdateReportSettings {
		t.Fatalf("operation logs = %+v, want update_report_settings", repo.logs)
	}
}

func TestAdminReadMethodsRequireAdmin(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAdminRepository()
	svc := NewAdminService(repo, nil)
	reqCtx := RequestContext{UserID: "user-1"}

	if _, err := svc.GetReportSettings(ctx, reqCtx); errorCode(t, err) != CodeForbidden {
		t.Fatalf("GetReportSettings() error = %v, want forbidden", err)
	}
	if _, err := svc.GetStatisticsOverview(ctx, reqCtx, 0); errorCode(t, err) != CodeForbidden {
		t.Fatalf("GetStatisticsOverview() error = %v, want forbidden", err)
	}
	if _, err := svc.ListDailyStatistics(ctx, reqCtx, 0); errorCode(t, err) != CodeForbidden {
		t.Fatalf("ListDailyStatistics() error = %v, want forbidden", err)
	}
	if _, err := svc.ListOperationLogs(ctx, reqCtx, OperationLogListFilter{}); errorCode(t, err) != CodeForbidden {
		t.Fatalf("ListOperationLogs() error = %v, want forbidden", err)
	}
}

func TestUpdateReportSettingsPreservesProfileWhenProfileIDIsOmitted(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAdminRepository()
	repo.settings.LLM = ReportSettingsModelConfig{
		Provider:       "ai-gateway",
		ProfileID:      "mp-current",
		Model:          "gpt-current",
		TimeoutSeconds: 30,
	}
	svc := NewAdminService(repo, nil)

	updated, err := svc.UpdateReportSettings(ctx, RequestContext{UserID: "admin-1", Roles: []string{"admin"}}, UpdateReportSettingsInput{
		LLM: &ReportSettingsModelConfig{TimeoutSeconds: 60},
	})
	if err != nil {
		t.Fatalf("UpdateReportSettings() error = %v", err)
	}
	if updated.LLM.ProfileID != "mp-current" || updated.LLM.Model != "gpt-current" || updated.LLM.TimeoutSeconds != 60 {
		t.Fatalf("updated LLM = %+v, want existing profile with updated timeout", updated.LLM)
	}

	cleared, err := svc.UpdateReportSettings(ctx, RequestContext{UserID: "admin-1", Roles: []string{"admin"}}, UpdateReportSettingsInput{
		LLM: &ReportSettingsModelConfig{ProfileIDSet: true},
	})
	if err != nil {
		t.Fatalf("UpdateReportSettings(clear profile) error = %v", err)
	}
	if cleared.LLM.ProfileID != "" || cleared.LLM.Model != "" || cleared.LLM.TimeoutSeconds != 60 {
		t.Fatalf("cleared LLM = %+v, want empty profile/model with existing timeout", cleared.LLM)
	}
}

func TestListOperationLogsHonorsFiltersAndSanitizesSummary(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAdminRepository()
	repo.logs = []OperationLog{
		{
			ID:              "log-1",
			OperationType:   OperationCreateReport,
			TargetType:      "report",
			TargetID:        "rpt-1",
			RequestID:       "req-1",
			RequestSource:   "mcp",
			ToolName:        "generate_report_outline",
			OperationResult: "succeeded",
			ParameterSummary: map[string]any{
				"reportType": "summer_peak_inspection",
				"prompt":     "must not survive",
				"nested": map[string]any{
					"objectKey": "documents/raw.docx",
					"count":     2,
				},
			},
			CreatedAt: time.Date(2026, 6, 30, 9, 5, 0, 0, time.UTC),
		},
		{
			ID:              "log-2",
			OperationType:   OperationUploadTemplate,
			TargetType:      "template",
			TargetID:        "tpl-1",
			RequestID:       "req-2",
			RequestSource:   "api",
			OperationResult: "succeeded",
			CreatedAt:       time.Date(2026, 6, 30, 9, 6, 0, 0, time.UTC),
		},
	}
	svc := NewAdminService(repo, nil)

	result, err := svc.ListOperationLogs(ctx, RequestContext{UserID: "admin-1", Roles: []string{"admin"}}, OperationLogListFilter{
		Page:          1,
		PageSize:      20,
		TargetType:    "report",
		TargetID:      "rpt-1",
		OperationType: OperationCreateReport,
		RequestID:     "req-1",
		RequestSource: "mcp",
		ToolName:      "generate_report_outline",
	})
	if err != nil {
		t.Fatalf("ListOperationLogs() error = %v", err)
	}
	if result.Page.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != "log-1" {
		t.Fatalf("filtered result = %+v, want only log-1", result)
	}
	summary := result.Items[0].ParameterSummary
	if _, ok := summary["prompt"]; ok {
		t.Fatalf("parameter summary leaked prompt: %+v", summary)
	}
	nested, _ := summary["nested"].(map[string]any)
	if _, ok := nested["objectKey"]; ok || nested["count"] != 2 {
		t.Fatalf("nested summary was not sanitized correctly: %+v", nested)
	}
}

func TestStatisticsOverviewAndDailyAreBounded(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAdminRepository()
	repo.overview = ReportStatisticsOverview{
		ReportCount:     2,
		TemplateCount:   3,
		MaterialCount:   4,
		JobStatusCounts: map[string]int{"succeeded": 1, "failed": 1},
		RecentDays:      30,
	}
	repo.daily = []ReportDailyStatistic{
		{Date: "2026-06-30", ReportType: "summer_peak_inspection", CreatedCount: 1, GeneratedCount: 1},
	}
	svc := NewAdminService(repo, nil)
	reqCtx := RequestContext{UserID: "admin-1", Roles: []string{"admin"}}

	overview, err := svc.GetStatisticsOverview(ctx, reqCtx, 0)
	if err != nil {
		t.Fatalf("GetStatisticsOverview() error = %v", err)
	}
	if overview.RecentDays != 30 || overview.JobStatusCounts["failed"] != 1 {
		t.Fatalf("overview = %+v", overview)
	}

	_, err = svc.ListDailyStatistics(ctx, reqCtx, 367)
	if code := errorCode(t, err); code != CodeValidation {
		t.Fatalf("days=367 error code = %q, want %q", code, CodeValidation)
	}

	daily, err := svc.ListDailyStatistics(ctx, reqCtx, 7)
	if err != nil {
		t.Fatalf("ListDailyStatistics() error = %v", err)
	}
	if repo.lastDailyDays != 7 || len(daily) != 1 {
		t.Fatalf("daily days/items = %d/%d, want 7/1", repo.lastDailyDays, len(daily))
	}
}

type fakeAdminRepository struct {
	settings      ReportSettings
	reportTypes   map[string]bool
	templates     map[string]ReportTemplate
	overview      ReportStatisticsOverview
	daily         []ReportDailyStatistic
	logs          []OperationLog
	lastDailyDays int
}

func newFakeAdminRepository() *fakeAdminRepository {
	return &fakeAdminRepository{
		settings: ReportSettings{
			LLM:              ReportSettingsModelConfig{Provider: "ai-gateway"},
			DefaultTemplates: map[string]string{},
			File:             ReportSettingsFileDefaults{DefaultFormat: "docx", DefaultNumberingMode: "global"},
		},
		reportTypes: map[string]bool{},
		templates:   map[string]ReportTemplate{},
	}
}

func (r *fakeAdminRepository) GetReportSettings(context.Context) (ReportSettings, error) {
	return r.settings, nil
}

func (r *fakeAdminRepository) SaveReportSettings(_ context.Context, settings ReportSettings) (ReportSettings, error) {
	r.settings = settings
	return settings, nil
}

func (r *fakeAdminRepository) ReportTypeExists(_ context.Context, code string) (bool, error) {
	return r.reportTypes[code], nil
}

func (r *fakeAdminRepository) FindReportTemplateByID(_ context.Context, id string) (ReportTemplate, error) {
	template, ok := r.templates[id]
	if !ok {
		return ReportTemplate{}, NewError(CodeNotFound, "report template not found", nil)
	}
	return template, nil
}

func (r *fakeAdminRepository) GetReportStatisticsOverview(context.Context, int) (ReportStatisticsOverview, error) {
	return r.overview, nil
}

func (r *fakeAdminRepository) ListReportDailyStatistics(_ context.Context, days int) ([]ReportDailyStatistic, error) {
	r.lastDailyDays = days
	return r.daily, nil
}

func (r *fakeAdminRepository) ListOperationLogs(_ context.Context, filter OperationLogListFilter) (OperationLogListResult, error) {
	items := make([]OperationLog, 0, len(r.logs))
	for _, log := range r.logs {
		if filter.TargetType != "" && log.TargetType != filter.TargetType {
			continue
		}
		if filter.TargetID != "" && log.TargetID != filter.TargetID {
			continue
		}
		if filter.OperationType != "" && log.OperationType != filter.OperationType {
			continue
		}
		if filter.RequestID != "" && log.RequestID != filter.RequestID {
			continue
		}
		if filter.RequestSource != "" && log.RequestSource != filter.RequestSource {
			continue
		}
		if filter.ToolName != "" && log.ToolName != filter.ToolName {
			continue
		}
		items = append(items, log)
	}
	return OperationLogListResult{Items: items, Page: PageMeta{Page: filter.Page, PageSize: filter.PageSize, Total: len(items)}}, nil
}

func (r *fakeAdminRepository) CreateOperationLog(_ context.Context, log OperationLog) (OperationLog, error) {
	r.logs = append(r.logs, log)
	return log, nil
}

type fakeProfileValidator struct {
	profiles map[string]ModelProfileReference
}

func (v *fakeProfileValidator) GetModelProfile(_ context.Context, _ RequestContext, id string) (ModelProfileReference, error) {
	profile, ok := v.profiles[id]
	if !ok {
		return ModelProfileReference{}, NewError(CodeNotFound, "model profile not found", nil)
	}
	return profile, nil
}
