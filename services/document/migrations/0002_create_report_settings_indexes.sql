-- +goose Up
CREATE TABLE report_settings (
    id text PRIMARY KEY,
    llm_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    default_templates_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    file_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO report_settings (id, llm_json, default_templates_json, file_json)
VALUES (
    'default',
    '{"provider":"ai-gateway"}'::jsonb,
    '{}'::jsonb,
    '{"defaultFormat":"docx","defaultNumberingMode":"global"}'::jsonb
)
ON CONFLICT (id) DO NOTHING;

CREATE INDEX idx_reports_active_created_at ON reports(created_at DESC)
WHERE deleted_at IS NULL AND status <> 'deleted';

CREATE INDEX idx_reports_generated_at ON reports(generated_at DESC)
WHERE generated_at IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX idx_reports_exported_at ON reports(exported_at DESC)
WHERE exported_at IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX idx_report_templates_active_created_at ON report_templates(created_at DESC)
WHERE deleted_at IS NULL;

CREATE INDEX idx_report_materials_active_created_at ON report_materials(created_at DESC)
WHERE deleted_at IS NULL;

CREATE INDEX idx_report_jobs_status_created_at_all ON report_jobs(status, created_at DESC);

CREATE INDEX idx_report_operation_logs_target_created_at ON report_operation_logs(target_type, target_id, created_at DESC);
CREATE INDEX idx_report_operation_logs_operation_created_at ON report_operation_logs(operation_type, created_at DESC);
CREATE INDEX idx_report_operation_logs_request_id_created_at ON report_operation_logs(request_id, created_at DESC);
CREATE INDEX idx_report_operation_logs_source_tool_created_at ON report_operation_logs(request_source, tool_name, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_report_operation_logs_source_tool_created_at;
DROP INDEX IF EXISTS idx_report_operation_logs_request_id_created_at;
DROP INDEX IF EXISTS idx_report_operation_logs_operation_created_at;
DROP INDEX IF EXISTS idx_report_operation_logs_target_created_at;
DROP INDEX IF EXISTS idx_report_jobs_status_created_at_all;
DROP INDEX IF EXISTS idx_report_materials_active_created_at;
DROP INDEX IF EXISTS idx_report_templates_active_created_at;
DROP INDEX IF EXISTS idx_reports_exported_at;
DROP INDEX IF EXISTS idx_reports_generated_at;
DROP INDEX IF EXISTS idx_reports_active_created_at;
DROP TABLE IF EXISTS report_settings;
