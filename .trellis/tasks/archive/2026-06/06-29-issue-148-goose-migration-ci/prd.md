# Complete Issue 148 Goose Migration CI

## Goal

Unify service migration documentation and CI around `goose@v3.27.1` so every landed Go service with SQL migrations can prove its migrations apply cleanly against PostgreSQL.

## Requirements

- Add a GitHub Actions migration gate for services that own SQL migrations under `services/<service>/migrations/*.sql`.
- Use `github.com/pressly/goose/v3/cmd/goose@v3.27.1` consistently in CI and README migration commands.
- Validate the landed migration services currently present in the repository: `auth`, `document`, `file`, `knowledge`, and `qa`.
- Use an ephemeral PostgreSQL service in CI for migration apply validation.
- Keep migration checks service-local; do not create a repository-root Go module or shared migration runner.
- Update service README files that mention goose without the fixed version or without an apply command.
- Confirm migration filenames use ordered numeric prefixes.

## Acceptance Criteria

- [x] CI can apply migrations for every service with `services/<service>/migrations/*.sql`.
- [x] README and CI references use goose `v3.27.1` consistently.
- [x] Migration filenames use ordered prefixes such as `0001_*`, `0002_*`.
- [x] Existing Go service checks still pass where affected.
- [x] PR body lists validation results and links `Closes #148`.

## Definition of Done

- Migration apply validation passes locally or any local environment limitation is clearly reported.
- `git diff --check` passes.
- Trellis task is archived after work commits.
- Branch is pushed to the fork and a PR targets upstream `develop`.

## Technical Approach

Add a dedicated GitHub Actions workflow for migration validation, with a matrix over landed services that have SQL migration files. Each matrix job runs with PostgreSQL `16-alpine`, installs/runs `goose@v3.27.1`, and applies migrations from the service directory against the temporary database.

Use `go run github.com/pressly/goose/v3/cmd/goose@v3.27.1` rather than relying on a preinstalled binary, so CI and README both pin the same version.

## Decision (ADR-lite)

**Context**: The repository has multiple independent Go modules and service-local migrations. A root migration runner would couple service boundaries and require extra tooling.

**Decision**: Use a service matrix in GitHub Actions and run goose from each service directory with a service-local migration path.

**Consequences**: Adding a future database-backed service requires adding that service to the matrix. The workflow remains explicit and easy to review, and it matches the service-local module layout.

## Out of Scope

- Creating new business schema beyond minimal fixes required for existing migrations to apply.
- Adding rollback/down migration CI unless already required by a service.
- Introducing sqlc generation, repository integration tests, or a shared migration abstraction.
- Changing services without SQL migration files.

## Technical Notes

- Issue: <https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/148>
- Authority: `docs/architecture/technology-decisions.md`.
- Referenced issue authority `private/doc-update-tasks-20260629.md` is not present in this checkout.
- Dependency issue #147 is merged into `develop`; current Go service baseline is `go 1.25.0`.
- Current services with SQL migrations at task start: `auth`, `document`, `file`, `knowledge`, `qa`.
## Validation

- `go test ./...` passed in `services/auth`, `services/document`, `services/file`, `services/gateway`, `services/knowledge`, and `services/qa`.
- `go build ./cmd/server` passed in all six landed Go services.
- `go build ./cmd/agent` passed in `services/qa`.
- `go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -version` returned `goose version: v3.27.1`.
- Static migration filename check passed for all SQL files under `services/*/migrations/`.
- Static goose annotation check passed for all SQL files under `services/*/migrations/`.
- `git diff --check` passed.
- Local Docker is unavailable (`docker` command not found), so PostgreSQL apply validation is expected to run in GitHub Actions via `.github/workflows/go-migrations.yml`.