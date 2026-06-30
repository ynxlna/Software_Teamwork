-- +goose Up
INSERT INTO report_types (code, name, description, enabled)
VALUES
    ('summer_peak_inspection', '迎峰度夏检查报告', '迎峰度夏检查报告', true),
    ('coal_inventory_audit', '煤库存审计报告', '煤库存审计报告', true)
ON CONFLICT (code) DO NOTHING;

INSERT INTO report_templates (
    id,
    template_name,
    report_type,
    version,
    filename,
    file_size,
    structure_json,
    style_config_json,
    description,
    enabled,
    created_by
)
VALUES
    (
        '11111111-1111-4111-8111-111111111101',
        '迎峰度夏检查报告占位模板',
        'summer_peak_inspection',
        1,
        'placeholder-summer-peak-inspection.docx',
        0,
        '{
            "templateStatus": "needs_decision",
            "dependency": "Formal DOCX template file is pending.",
            "placeholderTemplate": true,
            "importPath": "services/document/migrations/0003_seed_initial_report_defaults.sql",
            "outline": [
                {"title": "检查概况", "level": 1},
                {"title": "风险与问题", "level": 1},
                {"title": "整改建议", "level": 1}
            ]
        }'::jsonb,
        '{
            "styleProfileId": "first-slice-default-docx",
            "templateStatus": "needs_decision",
            "defaultFormat": "docx"
        }'::jsonb,
        'Needs Decision: formal DOCX template file is pending. Placeholder seeded from services/document/migrations/0003_seed_initial_report_defaults.sql.',
        true,
        'system'
    ),
    (
        '11111111-1111-4111-8111-111111111102',
        '煤库存审计报告占位模板',
        'coal_inventory_audit',
        1,
        'placeholder-coal-inventory-audit.docx',
        0,
        '{
            "templateStatus": "needs_decision",
            "dependency": "Formal DOCX template file is pending.",
            "placeholderTemplate": true,
            "importPath": "services/document/migrations/0003_seed_initial_report_defaults.sql",
            "outline": [
                {"title": "审计概况", "level": 1},
                {"title": "库存核查", "level": 1},
                {"title": "审计结论", "level": 1}
            ]
        }'::jsonb,
        '{
            "styleProfileId": "first-slice-default-docx",
            "templateStatus": "needs_decision",
            "defaultFormat": "docx"
        }'::jsonb,
        'Needs Decision: formal DOCX template file is pending. Placeholder seeded from services/document/migrations/0003_seed_initial_report_defaults.sql.',
        true,
        'system'
    )
ON CONFLICT (id) DO NOTHING;

INSERT INTO report_settings (id, llm_json, default_templates_json, file_json)
VALUES (
    'default',
    '{"provider":"ai-gateway"}'::jsonb,
    '{
        "summer_peak_inspection": "11111111-1111-4111-8111-111111111101",
        "coal_inventory_audit": "11111111-1111-4111-8111-111111111102"
    }'::jsonb,
    '{
        "defaultFormat": "docx",
        "defaultNumberingMode": "global",
        "defaultStyleProfileId": "first-slice-default-docx"
    }'::jsonb
)
ON CONFLICT (id) DO NOTHING;

UPDATE report_settings
SET
    llm_json = '{"provider":"ai-gateway"}'::jsonb || llm_json,
    default_templates_json = '{
        "summer_peak_inspection": "11111111-1111-4111-8111-111111111101",
        "coal_inventory_audit": "11111111-1111-4111-8111-111111111102"
    }'::jsonb || default_templates_json,
    file_json = '{
        "defaultFormat": "docx",
        "defaultNumberingMode": "global",
        "defaultStyleProfileId": "first-slice-default-docx"
    }'::jsonb || file_json,
    updated_at = now()
WHERE id = 'default'
  AND (
      NOT llm_json ? 'provider'
      OR NOT default_templates_json ? 'summer_peak_inspection'
      OR NOT default_templates_json ? 'coal_inventory_audit'
      OR NOT file_json ? 'defaultFormat'
      OR NOT file_json ? 'defaultNumberingMode'
      OR NOT file_json ? 'defaultStyleProfileId'
  );

-- +goose Down
UPDATE report_settings
SET
    default_templates_json = default_templates_json - 'summer_peak_inspection',
    updated_at = now()
WHERE id = 'default'
  AND default_templates_json ->> 'summer_peak_inspection' = '11111111-1111-4111-8111-111111111101';

UPDATE report_settings
SET
    default_templates_json = default_templates_json - 'coal_inventory_audit',
    updated_at = now()
WHERE id = 'default'
  AND default_templates_json ->> 'coal_inventory_audit' = '11111111-1111-4111-8111-111111111102';

UPDATE report_settings
SET
    file_json = file_json - 'defaultStyleProfileId',
    updated_at = now()
WHERE id = 'default'
  AND file_json ->> 'defaultStyleProfileId' = 'first-slice-default-docx';

DELETE FROM report_templates template
WHERE template.id IN (
      '11111111-1111-4111-8111-111111111101',
      '11111111-1111-4111-8111-111111111102'
  )
  AND template.created_by = 'system'
  AND template.structure_json ->> 'placeholderTemplate' = 'true'
  AND NOT EXISTS (
      SELECT 1
      FROM reports report
      WHERE report.template_id = template.id
  )
  AND NOT EXISTS (
      SELECT 1
      FROM report_template_materials link
      WHERE link.template_id = template.id
  )
  AND NOT EXISTS (
      SELECT 1
      FROM report_types report_type
      WHERE report_type.default_template_id = template.id
  );
