package repository

import (
	"os"
	"strings"
	"testing"
)

func TestAdminMigrationIncludesOperationLogFilterIndexes(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/0002_create_report_settings_indexes.sql")
	if err != nil {
		t.Fatalf("read admin migration: %v", err)
	}
	migration := string(raw)
	for _, indexName := range []string{
		"idx_report_operation_logs_target_created_at",
		"idx_report_operation_logs_operation_created_at",
		"idx_report_operation_logs_request_id_created_at",
		"idx_report_operation_logs_source_tool_created_at",
	} {
		if !strings.Contains(migration, indexName) {
			t.Fatalf("migration missing operation log filter index %q", indexName)
		}
	}
}

func TestInitialReportDefaultSeedDocumentsPlaceholdersAndAvoidsSensitiveDefaults(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/0003_seed_initial_report_defaults.sql")
	if err != nil {
		t.Fatalf("read initial report default seed migration: %v", err)
	}
	migration := string(raw)
	for _, required := range []string{
		"summer_peak_inspection",
		"coal_inventory_audit",
		"11111111-1111-4111-8111-111111111101",
		"11111111-1111-4111-8111-111111111102",
		"first-slice-default-docx",
		"needs_decision",
		"services/document/migrations/0003_seed_initial_report_defaults.sql",
		"ON CONFLICT",
		"-- +goose Down",
		"default_templates_json - 'summer_peak_inspection'",
		"default_templates_json - 'coal_inventory_audit'",
		"file_json - 'defaultStyleProfileId'",
		"NOT EXISTS",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("initial report seed missing %q", required)
		}
	}

	lower := strings.ToLower(migration)
	for _, forbidden := range []string{
		"apikey",
		"api_key",
		"authorization",
		"bearer ",
		"file_ref",
		"fileref",
		"object_key",
		"objectkey",
		"minio",
		"secret",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("initial report seed contains forbidden sensitive marker %q", forbidden)
		}
	}
}
