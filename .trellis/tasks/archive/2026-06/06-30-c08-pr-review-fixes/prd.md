# C-08 PR Review Fixes

## Background

PR #238 implements Document C-08 report settings, statistics, and operation
logs. Codex PR Review found that management read endpoints currently require
only gateway user context, while `PATCH /report-settings` requires admin.

The affected service methods are:

- `GetReportSettings`
- `GetStatisticsOverview`
- `ListDailyStatistics`
- `ListOperationLogs`

Gateway proxies these routes and passes user roles; Document service must keep
the service-layer authorization guard because these are admin/audit surfaces.

## Goal

Make report settings, statistics, and operation-log read endpoints require
`admin` or `super_admin`, matching the write endpoint and the gateway contract.

## Acceptance Criteria

- Non-admin callers receive `403 forbidden` from:
  - `GET /report-settings`
  - `GET /report-statistics/overview`
  - `GET /report-statistics/daily`
  - `GET /report-operation-logs`
- Admin callers still receive the existing successful responses.
- Service tests cover non-admin rejection for all management read methods.
- Handler tests use admin role for successful management reads.
- Document service checks pass:
  - `go test ./... -count=1`
  - `go build ./cmd/server`
  - `git diff --check`

## Non-Goals

- Do not change gateway OpenAPI.
- Do not change frontend behavior.
- Do not add unrelated filters or new management endpoints.
