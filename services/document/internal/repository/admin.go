package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
	"github.com/jackc/pgx/v5"
)

const reportSettingsID = "default"

type reportSettingsModelJSON struct {
	Provider       string `json:"provider,omitempty"`
	ProfileID      string `json:"profileId,omitempty"`
	Model          string `json:"model,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}

type reportSettingsFileJSON struct {
	DefaultFormat         string         `json:"defaultFormat,omitempty"`
	DefaultNumberingMode  string         `json:"defaultNumberingMode,omitempty"`
	DefaultStyleProfileID string         `json:"defaultStyleProfileId,omitempty"`
	Extra                 map[string]any `json:"extra,omitempty"`
}

func (r *PostgresRepository) GetReportSettings(ctx context.Context) (service.ReportSettings, error) {
	var llmRaw, templatesRaw, fileRaw []byte
	var updatedAt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT llm_json, default_templates_json, file_json, updated_at
		FROM report_settings
		WHERE id = $1`, reportSettingsID).Scan(&llmRaw, &templatesRaw, &fileRaw, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return defaultReportSettings(), nil
		}
		return service.ReportSettings{}, fmt.Errorf("get report settings: %w", err)
	}
	settings, err := decodeReportSettings(llmRaw, templatesRaw, fileRaw, updatedAt)
	if err != nil {
		return service.ReportSettings{}, err
	}
	return settings, nil
}

func (r *PostgresRepository) SaveReportSettings(ctx context.Context, settings service.ReportSettings) (service.ReportSettings, error) {
	if settings.UpdatedAt.IsZero() {
		settings.UpdatedAt = time.Now().UTC()
	}
	llmRaw, templatesRaw, fileRaw, err := encodeReportSettings(settings)
	if err != nil {
		return service.ReportSettings{}, err
	}
	var savedLLMRaw, savedTemplatesRaw, savedFileRaw []byte
	var savedUpdatedAt time.Time
	err = r.db.QueryRow(ctx, `
		INSERT INTO report_settings (id, llm_json, default_templates_json, file_json, updated_at)
		VALUES ($1, $2::jsonb, $3::jsonb, $4::jsonb, $5)
		ON CONFLICT (id) DO UPDATE SET
			llm_json = EXCLUDED.llm_json,
			default_templates_json = EXCLUDED.default_templates_json,
			file_json = EXCLUDED.file_json,
			updated_at = EXCLUDED.updated_at
		RETURNING llm_json, default_templates_json, file_json, updated_at`,
		reportSettingsID, string(llmRaw), string(templatesRaw), string(fileRaw), settings.UpdatedAt,
	).Scan(&savedLLMRaw, &savedTemplatesRaw, &savedFileRaw, &savedUpdatedAt)
	if err != nil {
		return service.ReportSettings{}, fmt.Errorf("save report settings: %w", err)
	}
	return decodeReportSettings(savedLLMRaw, savedTemplatesRaw, savedFileRaw, savedUpdatedAt)
}

func (r *PostgresRepository) CreateOperationLog(ctx context.Context, log service.OperationLog) (service.OperationLog, error) {
	if strings.TrimSpace(log.ID) == "" {
		log.ID = newUUIDString()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(log.OperationResult) == "" {
		log.OperationResult = service.OperationResultSucceeded
	}
	id, err := parseUUID(log.ID)
	if err != nil {
		return service.OperationLog{}, service.ValidationError(map[string]string{"id": "must be a valid UUID"})
	}
	parameterSummary, err := marshalMapObject(log.ParameterSummary)
	if err != nil {
		return service.OperationLog{}, fmt.Errorf("encode operation log parameter summary: %w", err)
	}
	metadata, err := marshalMapObject(log.Metadata)
	if err != nil {
		return service.OperationLog{}, fmt.Errorf("encode operation log metadata: %w", err)
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO report_operation_logs (
			id, operator_id, operator_name, operation_type, target_type, target_id,
			request_id, request_source, tool_name, parameter_summary_json,
			operation_result, error_message, metadata_json, created_at
		)
		VALUES (
			$1, NULLIF($2, ''), NULLIF($3, ''), $4, $5, $6,
			NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), $10::jsonb,
			$11, NULLIF($12, ''), $13::jsonb, $14
		)
		RETURNING
			id::text, COALESCE(operator_id, ''), COALESCE(operator_name, ''),
			operation_type, target_type, target_id, COALESCE(request_id, ''),
			COALESCE(request_source, ''), COALESCE(tool_name, ''),
			parameter_summary_json, operation_result, COALESCE(error_message, ''),
			metadata_json, created_at`,
		id,
		log.OperatorID,
		log.OperatorName,
		log.OperationType,
		log.TargetType,
		log.TargetID,
		log.RequestID,
		log.RequestSource,
		log.ToolName,
		string(parameterSummary),
		log.OperationResult,
		log.ErrorMessage,
		string(metadata),
		log.CreatedAt,
	)
	created, err := scanOperationLog(row)
	if err != nil {
		if isUniqueViolation(err) {
			return service.OperationLog{}, service.NewError(service.CodeConflict, "operation log already exists", err)
		}
		return service.OperationLog{}, fmt.Errorf("insert operation log: %w", err)
	}
	return created, nil
}

func (r *PostgresRepository) ListOperationLogs(ctx context.Context, filter service.OperationLogListFilter) (service.OperationLogListResult, error) {
	page, pageSize := normalizeRepositoryPage(filter.Page, filter.PageSize)
	where := []string{"1 = 1"}
	args := []any{}
	addFilter := func(column, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		args = append(args, strings.TrimSpace(value))
		where = append(where, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	addFilter("target_type", filter.TargetType)
	addFilter("target_id", filter.TargetID)
	addFilter("operation_type", filter.OperationType)
	addFilter("request_id", filter.RequestID)
	addFilter("request_source", filter.RequestSource)
	addFilter("tool_name", filter.ToolName)

	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM report_operation_logs WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return service.OperationLogListResult{}, fmt.Errorf("count operation logs: %w", err)
	}

	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	query := fmt.Sprintf(`
		SELECT
			id::text, COALESCE(operator_id, ''), COALESCE(operator_name, ''),
			operation_type, target_type, target_id, COALESCE(request_id, ''),
			COALESCE(request_source, ''), COALESCE(tool_name, ''),
			parameter_summary_json, operation_result, COALESCE(error_message, ''),
			metadata_json, created_at
		FROM report_operation_logs
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d`, whereSQL, len(queryArgs)-1, len(queryArgs))
	rows, err := r.db.Query(ctx, query, queryArgs...)
	if err != nil {
		return service.OperationLogListResult{}, fmt.Errorf("list operation logs: %w", err)
	}
	defer rows.Close()

	items := []service.OperationLog{}
	for rows.Next() {
		item, err := scanOperationLog(rows)
		if err != nil {
			return service.OperationLogListResult{}, fmt.Errorf("scan operation log: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return service.OperationLogListResult{}, fmt.Errorf("iterate operation logs: %w", err)
	}
	return service.OperationLogListResult{
		Items: items,
		Page:  service.PageMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

func (r *PostgresRepository) GetReportStatisticsOverview(ctx context.Context, recentDays int) (service.ReportStatisticsOverview, error) {
	overview := service.ReportStatisticsOverview{
		JobStatusCounts: map[string]int{},
		RecentDays:      recentDays,
	}
	if err := r.db.QueryRow(ctx, `
		SELECT count(*)
		FROM reports
		WHERE deleted_at IS NULL AND status <> 'deleted'`).Scan(&overview.ReportCount); err != nil {
		return service.ReportStatisticsOverview{}, fmt.Errorf("count reports: %w", err)
	}
	if err := r.db.QueryRow(ctx, `
		SELECT count(*)
		FROM report_templates
		WHERE deleted_at IS NULL`).Scan(&overview.TemplateCount); err != nil {
		return service.ReportStatisticsOverview{}, fmt.Errorf("count report templates: %w", err)
	}
	if err := r.db.QueryRow(ctx, `
		SELECT count(*)
		FROM report_materials
		WHERE deleted_at IS NULL`).Scan(&overview.MaterialCount); err != nil {
		return service.ReportStatisticsOverview{}, fmt.Errorf("count report materials: %w", err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT status, count(*)
		FROM report_jobs
		GROUP BY status
		ORDER BY status`)
	if err != nil {
		return service.ReportStatisticsOverview{}, fmt.Errorf("count report jobs by status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return service.ReportStatisticsOverview{}, fmt.Errorf("scan report job status count: %w", err)
		}
		overview.JobStatusCounts[status] = count
	}
	if err := rows.Err(); err != nil {
		return service.ReportStatisticsOverview{}, fmt.Errorf("iterate report job status counts: %w", err)
	}
	return overview, nil
}

func (r *PostgresRepository) ListReportDailyStatistics(ctx context.Context, days int) ([]service.ReportDailyStatistic, error) {
	rows, err := r.db.Query(ctx, `
		WITH bounds AS (
			SELECT
				(CURRENT_DATE - (($1::int - 1) * INTERVAL '1 day'))::date AS start_date,
				(CURRENT_DATE + INTERVAL '1 day')::date AS end_date
		),
		daily AS (
			SELECT created_at::date AS stat_date, report_type, count(*)::int AS created_count, 0::int AS generated_count, 0::int AS failed_count, 0::int AS exported_count
			FROM reports, bounds
			WHERE deleted_at IS NULL
				AND status <> 'deleted'
				AND created_at >= bounds.start_date
				AND created_at < bounds.end_date
			GROUP BY created_at::date, report_type
			UNION ALL
			SELECT generated_at::date AS stat_date, report_type, 0::int, count(*)::int, 0::int, 0::int
			FROM reports, bounds
			WHERE deleted_at IS NULL
				AND generated_at IS NOT NULL
				AND generated_at >= bounds.start_date
				AND generated_at < bounds.end_date
			GROUP BY generated_at::date, report_type
			UNION ALL
			SELECT exported_at::date AS stat_date, report_type, 0::int, 0::int, 0::int, count(*)::int
			FROM reports, bounds
			WHERE deleted_at IS NULL
				AND exported_at IS NOT NULL
				AND exported_at >= bounds.start_date
				AND exported_at < bounds.end_date
			GROUP BY exported_at::date, report_type
			UNION ALL
			SELECT COALESCE(j.finished_at, j.created_at)::date AS stat_date, r.report_type, 0::int, 0::int, count(*)::int, 0::int
			FROM report_jobs j
			JOIN reports r ON r.id = j.report_id
			CROSS JOIN bounds
			WHERE r.deleted_at IS NULL
				AND r.status <> 'deleted'
				AND j.status = 'failed'
				AND COALESCE(j.finished_at, j.created_at) >= bounds.start_date
				AND COALESCE(j.finished_at, j.created_at) < bounds.end_date
			GROUP BY COALESCE(j.finished_at, j.created_at)::date, r.report_type
		)
		SELECT
			to_char(stat_date, 'YYYY-MM-DD') AS date,
			report_type,
			sum(created_count)::int,
			sum(generated_count)::int,
			sum(failed_count)::int,
			sum(exported_count)::int
		FROM daily
		GROUP BY stat_date, report_type
		ORDER BY stat_date DESC, report_type ASC`, days)
	if err != nil {
		return nil, fmt.Errorf("list report daily statistics: %w", err)
	}
	defer rows.Close()

	items := []service.ReportDailyStatistic{}
	for rows.Next() {
		var item service.ReportDailyStatistic
		if err := rows.Scan(
			&item.Date,
			&item.ReportType,
			&item.CreatedCount,
			&item.GeneratedCount,
			&item.FailedCount,
			&item.ExportedCount,
		); err != nil {
			return nil, fmt.Errorf("scan report daily statistic: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate report daily statistics: %w", err)
	}
	return items, nil
}

func encodeReportSettings(settings service.ReportSettings) ([]byte, []byte, []byte, error) {
	llmRaw, err := json.Marshal(reportSettingsModelJSON{
		Provider:       settings.LLM.Provider,
		ProfileID:      settings.LLM.ProfileID,
		Model:          settings.LLM.Model,
		TimeoutSeconds: settings.LLM.TimeoutSeconds,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encode settings llm: %w", err)
	}
	defaultTemplates := settings.DefaultTemplates
	if defaultTemplates == nil {
		defaultTemplates = map[string]string{}
	}
	templatesRaw, err := json.Marshal(defaultTemplates)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encode settings default templates: %w", err)
	}
	fileRaw, err := json.Marshal(reportSettingsFileJSON{
		DefaultFormat:         settings.File.DefaultFormat,
		DefaultNumberingMode:  settings.File.DefaultNumberingMode,
		DefaultStyleProfileID: settings.File.DefaultStyleProfileID,
		Extra:                 settings.File.Extra,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encode settings file defaults: %w", err)
	}
	return llmRaw, templatesRaw, fileRaw, nil
}

func decodeReportSettings(llmRaw, templatesRaw, fileRaw []byte, updatedAt time.Time) (service.ReportSettings, error) {
	settings := defaultReportSettings()
	settings.UpdatedAt = updatedAt
	var llm reportSettingsModelJSON
	if len(llmRaw) > 0 {
		if err := json.Unmarshal(llmRaw, &llm); err != nil {
			return service.ReportSettings{}, fmt.Errorf("decode settings llm: %w", err)
		}
		settings.LLM = service.ReportSettingsModelConfig{
			Provider:       llm.Provider,
			ProfileID:      llm.ProfileID,
			Model:          llm.Model,
			TimeoutSeconds: llm.TimeoutSeconds,
		}
	}
	if len(templatesRaw) > 0 {
		if err := json.Unmarshal(templatesRaw, &settings.DefaultTemplates); err != nil {
			return service.ReportSettings{}, fmt.Errorf("decode settings default templates: %w", err)
		}
	}
	var file reportSettingsFileJSON
	if len(fileRaw) > 0 {
		if err := json.Unmarshal(fileRaw, &file); err != nil {
			return service.ReportSettings{}, fmt.Errorf("decode settings file defaults: %w", err)
		}
		settings.File = service.ReportSettingsFileDefaults{
			DefaultFormat:         file.DefaultFormat,
			DefaultNumberingMode:  file.DefaultNumberingMode,
			DefaultStyleProfileID: file.DefaultStyleProfileID,
			Extra:                 file.Extra,
		}
	}
	if settings.LLM.Provider == "" {
		settings.LLM.Provider = service.DefaultReportSettingsProvider
	}
	if settings.DefaultTemplates == nil {
		settings.DefaultTemplates = map[string]string{}
	}
	if settings.File.DefaultFormat == "" {
		settings.File.DefaultFormat = service.DefaultReportSettingsFormat
	}
	if settings.File.DefaultNumberingMode == "" {
		settings.File.DefaultNumberingMode = service.DefaultReportNumberingMode
	}
	if settings.File.Extra == nil {
		settings.File.Extra = map[string]any{}
	}
	return settings, nil
}

func defaultReportSettings() service.ReportSettings {
	return service.ReportSettings{
		LLM: service.ReportSettingsModelConfig{
			Provider: service.DefaultReportSettingsProvider,
		},
		DefaultTemplates: map[string]string{},
		File: service.ReportSettingsFileDefaults{
			DefaultFormat:        service.DefaultReportSettingsFormat,
			DefaultNumberingMode: service.DefaultReportNumberingMode,
			Extra:                map[string]any{},
		},
	}
}

func scanOperationLog(row scanner) (service.OperationLog, error) {
	var value service.OperationLog
	var parameterSummaryRaw, metadataRaw []byte
	if err := row.Scan(
		&value.ID,
		&value.OperatorID,
		&value.OperatorName,
		&value.OperationType,
		&value.TargetType,
		&value.TargetID,
		&value.RequestID,
		&value.RequestSource,
		&value.ToolName,
		&parameterSummaryRaw,
		&value.OperationResult,
		&value.ErrorMessage,
		&metadataRaw,
		&value.CreatedAt,
	); err != nil {
		return service.OperationLog{}, err
	}
	value.ParameterSummary = unmarshalMapObject(parameterSummaryRaw)
	value.Metadata = unmarshalMapObject(metadataRaw)
	return value, nil
}

func marshalMapObject(value map[string]any) ([]byte, error) {
	if value == nil {
		value = map[string]any{}
	}
	return json.Marshal(value)
}

func unmarshalMapObject(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	value := map[string]any{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]any{}
	}
	return value
}

func normalizeRepositoryPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = service.DefaultPage
	}
	if pageSize <= 0 {
		pageSize = service.DefaultPageSize
	}
	if pageSize > service.MaxPageSize {
		pageSize = service.MaxPageSize
	}
	return page, pageSize
}
