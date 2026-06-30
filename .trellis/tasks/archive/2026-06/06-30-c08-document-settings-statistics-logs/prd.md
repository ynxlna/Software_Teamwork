# C-08 Document Report Settings / Statistics / Operation Logs

## Background

Redo the work from the closed PR #221 on top of the latest `upstream/develop`.
This task implements issue #104 for `services/document`: report settings,
statistics, and operation log read/write support.

Current develop keeps these routes scaffolded as `not_implemented`:

- `GET /report-settings`
- `PATCH /report-settings`
- `GET /report-statistics/overview`
- `GET /report-statistics/daily`
- `GET /report-operation-logs`

The feature must align with gateway OpenAPI, Document service docs, service
boundaries, and the review feedback from the previous PR.

## Goals

- Persist and expose report generation settings.
- Allow only admin or super admin callers to update report settings.
- Validate default template references against enabled, non-deleted templates
  and matching report types.
- Validate AI Gateway profile references through a Document-owned client when a
  profile id is provided.
- Provide statistics overview and daily trend queries aligned with gateway
  schema.
- Provide operation log query with all documented filters.
- Record operation logs from real mutation paths, not only expose a query API.
- Keep operation logs secret-safe: no prompt text, object keys, signed URLs,
  provider keys, raw file references, or full request bodies.

## Non-Goals

- Do not implement user authentication. Gateway injects caller context.
- Do not implement frontend pages.
- Do not implement model profile CRUD. AI Gateway owns model profiles.
- Do not implement real AI generation, DOCX generation, or report file content.
- Do not change gateway contracts unless a direct mismatch is discovered and
  documented.

## Acceptance Criteria

- `GET /report-settings` returns settings using the project envelope.
- `PATCH /report-settings` rejects non-admin callers with `403 forbidden`.
- `PATCH /report-settings` validates:
  - `llm.profileId` exists when non-empty,
  - `defaultTemplates` is a report-type to template-id map,
  - each report type exists and is enabled,
  - each template exists, is enabled, not deleted, and matches the map key,
  - file defaults use supported format and numbering mode values.
- Statistics overview includes at least report count, template count,
  material count, job status counts, and recent-day daily trend data.
- Daily statistics supports bounded `days` query, defaults to 30, and avoids
  unbounded full scans.
- Operation log list supports pagination and filters for target type, target id,
  operation type, request id, request source, and tool name.
- Mutation paths for templates, materials, reports, outlines/sections, jobs, and
  job status transitions record sanitized operation logs where the Document
  service owns the write.
- Route coverage tests no longer expect these C-08 routes to return
  `not_implemented`.
- `services/document` passes `go test ./...`, `go build ./cmd/server`, and
  `git diff --check`.

## Test Plan

Follow TDD:

- Add failing service tests for settings authorization and validation.
- Add failing service tests for statistics and operation log filter behavior.
- Add failing tests showing mutation paths create sanitized operation logs.
- Add failing handler tests for settings/statistics/logs response shapes and
  query parsing.
- Implement minimal production code until tests pass.
- Run the full Document service test/build checks before commit and PR.

## Notes From Prior PR Review

- Do not store `defaultTemplateId` as a single value; use
  `defaultTemplates` map keyed by report type.
- Do not leave operation logs as query-only; add production write paths.
- Keep schema and model names aligned with existing
  `report_operation_logs.parameter_summary_json` and `metadata_json`.
- Honor every operation-log query filter.
- Exclude soft-deleted reports/templates/materials from counts and validation.
- Use separate count queries for paginated log results.
- Make `llm.profileId` clearing semantics explicit.
- Do not add invalid sqlc query files.
