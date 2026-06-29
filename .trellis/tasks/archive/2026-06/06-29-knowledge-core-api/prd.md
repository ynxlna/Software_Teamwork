# A-09 Knowledge Core API

## Goal

Rebuild `services/knowledge` from a cleared service directory and implement the
P0 Knowledge foundation API required by issue #81: knowledge base CRUD, document
status listing/detail, durable PostgreSQL-backed metadata, and docs-compatible
response/error behavior. This gives frontend/admin/gateway callers a practical
Knowledge Management backend before full parsing, embedding, and retrieval are
implemented.

## What I Already Know

- GitHub issue: <https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/81>
- Issue title: `[A-09] Knowledge 知识库与文档状态基础 API`
- Issue assignee: `L-1ngg`
- Labels: `L1nggTeam`, `backend`, `service:knowledge`
- Suggested branch: `L1nggTeam/feat/knowledge-core-api`
- Current working branch after alignment: `L1nggTeam/feat/knowledge-core-api`
- `services/knowledge` has been intentionally cleared and currently only keeps
  `.gitkeep`.
- Public API authority is `docs/services/gateway/api/openapi.yaml`.
- Knowledge docs are authority for service boundary and data model intent, but
  `docs/services/knowledge/docs/implementation.md` contains stale references to
  the removed implementation and must not be copied blindly.

## Requirements

- Recreate a runnable Go service under `services/knowledge` using the existing
  project backend service shape, with `services/knowledge/go.mod` declaring
  `go 1.25.0`.
- Use the documented backend persistence stack as the implementation baseline:
  PostgreSQL with `pgx` + `sqlc`, migrations with `goose`, and no ORM.
- Implement service-local operational endpoints:
  - `GET /healthz`
  - `GET /readyz`
- Implement service-local internal Knowledge Base APIs aligned with gateway
  public contract:
  - `GET /internal/v1/knowledge-bases`
  - `POST /internal/v1/knowledge-bases`
  - `GET /internal/v1/knowledge-bases/{knowledgeBaseId}`
  - `PATCH /internal/v1/knowledge-bases/{knowledgeBaseId}`
  - `DELETE /internal/v1/knowledge-bases/{knowledgeBaseId}`
- Implement service-local internal document state APIs:
  - `GET /internal/v1/knowledge-bases/{knowledgeBaseId}/documents`
  - `GET /internal/v1/documents/{documentId}`
- Persist Knowledge Base metadata, Knowledge Document metadata/status, and
  enough processing/job trace state for provenance in PostgreSQL.
- Preserve and expose `DocumentStatus` values:
  - `uploaded`
  - `parsing`
  - `chunking`
  - `embedding`
  - `ready`
  - `failed`
- Support list pagination with `page`, `pageSize`, and `total`.
- Preserve request ID behavior:
  - read `X-Request-Id` when present;
  - generate one when missing;
  - return it in response headers and JSON envelopes.
- Apply basic user context filtering from gateway headers:
  - require `X-User-Id` for business endpoints;
  - read `X-User-Roles` and `X-User-Permissions`;
  - list/detail reads return resources created by the caller, plus broader
    resources only when the caller has an explicit admin role or knowledge read
    permission;
  - create/update/delete require an admin role or explicit knowledge write
    permission;
  - hidden resources return `404 not_found` rather than leaking existence;
  - authenticated but unauthorized mutations return `403 forbidden`.
- Preserve failure/provenance fields for document status:
  - `errorCode`
  - `errorMessage`
  - `parserBackend`
  - `jobId`
  - `createdBy`
  - `createdAt`
  - `updatedAt`
  - `chunkCount`
- Use soft delete for knowledge bases and documents.
- Deleting a knowledge base must not hard-drop future chunks/index records
  directly. It should mark owned metadata deleted and leave a cleanup path
  through processing/job state or a documented follow-up cleanup mechanism.
- Include a `document_chunks` schema in the first migration as a provenance and
  cleanup anchor, even though chunk write/read APIs are out of scope for A-09.
  This keeps delete semantics aligned with the docs without implementing the
  full parser/chunker/vector pipeline.
- Recreate service-local docs/contracts that match the implemented MVP:
  - `services/knowledge/README.md`
  - `services/knowledge/api/openapi.yaml`
  - `services/knowledge/migrations/0001_*.sql`
  - `services/knowledge/sqlc.yaml`
  - `services/knowledge/internal/repository/queries/*.sql`

## Acceptance Criteria

- [x] `services/knowledge` is a runnable Go 1.25.0 service-local module.
- [x] `go test ./...` passes from `services/knowledge`.
- [x] `go build ./cmd/server` passes from `services/knowledge`.
- [x] `GET /healthz` and `GET /readyz` return standard `{ data, requestId }`
  envelopes.
- [x] Knowledge base list/create/detail/update/delete routes return schemas
  aligned with `docs/services/gateway/api/openapi.yaml`.
- [x] Document list/detail routes return document status and provenance fields
  aligned with `DocumentSummary`.
- [x] Missing `X-User-Id` returns `401 unauthorized`.
- [x] Missing write permission for create/update/delete returns `403 forbidden`.
- [x] Invalid request bodies/query params return `400 validation_error` with
  field details where useful.
- [x] Not found, deleted, or hidden knowledge bases/documents return
  `404 not_found`.
- [x] Pagination includes `page`, `pageSize`, and `total`.
- [x] Soft-deleted knowledge bases are excluded from list/detail/document list.
- [x] Soft-deleted documents are excluded from list/detail.
- [x] Database migration creates required tables, constraints, and indexes,
  including `document_chunks` as the future chunk/index cleanup anchor.
- [x] Repository implementation uses `pgx` + `sqlc` for PostgreSQL access; no
  ORM is introduced.
- [x] Handler/service/repository tests cover success and error paths.
- [x] `git diff --check` passes.

## Definition Of Done

- Tests added or updated for HTTP handlers, service rules, and repository
  behavior.
- Service docs and service-local OpenAPI match implemented behavior.
- Migration is present and reviewed for source-of-truth/provenance needs.
- Validation commands and skipped checks are reported clearly.
- PR description can list completed scope, verification commands, risks, and
  `Closes #81`.

## Out Of Scope

- Gateway proxy implementation unless explicitly requested later.
- Frontend pages or generated frontend API client updates.
- Direct browser-facing API routes outside gateway.
- Full multipart upload flow.
- Full parser/chunker pipeline.
- Embedding generation.
- Qdrant collection/index implementation.
- Retrieval query execution.
- LLM answer generation.
- Agent/MCP orchestration.
- Report generation integration.
- Direct `POST /documents`; document creation remains a later upload/handoff
  workflow.

## Technical Approach

### Service Shape

Recreate the standard service-local structure:

```text
services/knowledge/
├── go.mod
├── sqlc.yaml
├── cmd/server/main.go
├── internal/config/
├── internal/http/
├── internal/service/
├── internal/repository/
│   ├── queries/
│   └── sqlc/
├── migrations/
├── api/openapi.yaml
└── README.md
```

`go.mod` must declare `go 1.25.0`. This is an explicit Knowledge-service
decision to keep the future RAG MCP server path straightforward, even though
older service docs and `services/file` currently use Go 1.22-era patterns.

Follow the existing `services/file` style where it matches project specs:

- standard `net/http` `ServeMux`
- `X-Request-Id` propagation
- `X-User-Id` gateway context extraction
- project JSON envelope
- app error classification
- local handler tests with `httptest`

### Core Domain Model

Use source-of-truth PostgreSQL tables:

- `knowledge_bases`
  - `id`, `name`, `description`, `doc_type`
  - `chunk_strategy`, `retrieval_strategy`
  - `created_by`, `created_at`, `updated_at`, `deleted_at`
- `knowledge_documents`
  - `id`, `knowledge_base_id`, `file_ref`
  - `name`, `content_type`, `size_bytes`
  - `status`, `error_code`, `error_message`
  - `tags`, `parser_backend`, `current_job_id`
  - `created_by`, `created_at`, `updated_at`, `deleted_at`
- `processing_jobs`
  - `id`, `knowledge_base_id`, `document_id`
  - `job_type`, `status`, `current_stage`, `progress_percent`
  - `message`, `error_code`, `error_message`
  - `attempts`, `max_attempts`, `started_at`, `finished_at`
  - `created_at`, `updated_at`
- `document_chunks`
  - `id`, `knowledge_base_id`, `document_id`, `chunk_index`
  - `section_path`, `content`, `token_count`, `chunk_type`
  - `qdrant_point_id`, `embedding_provider`, `embedding_model`,
    `embedding_dimension`
  - `metadata`, `created_at`

`document_chunks` is created now to preserve the documented data model and
delete cleanup path. A-09 should not implement parser/chunker writes or chunk
read APIs unless they become explicitly required later.

### PostgreSQL Access

Use the documented `pgx` + `sqlc` shape:

- `services/knowledge/sqlc.yaml`
- SQL query files under `internal/repository/queries/`
- generated package under `internal/repository/sqlc/`
- repository adapter wraps generated queries and returns service domain structs
- handlers and service use cases do not import generated SQL row types directly

### Access Filtering

MVP user-context filtering:

- business endpoints require `X-User-Id`;
- create/update/delete require an admin role or explicit knowledge write
  permission from `X-User-Roles` / `X-User-Permissions`;
- create stores `created_by`;
- read operations return caller-created resources by default;
- read operations can include broader resources only for callers with an admin
  role or explicit knowledge read permission;
- hidden/deleted resources map to `not_found`;
- authenticated callers lacking mutation rights receive `forbidden`.

This stays within issue #81's "basic access filtering" while following the
docs' role/permission context model instead of a pure owner-only shortcut.

### Delete Semantics

Use soft delete:

- Knowledge base delete sets `knowledge_bases.deleted_at`.
- Documents under a deleted knowledge base are excluded from reads.
- Existing/future chunks remain associated with their document rows and are not
  hard-deleted by the metadata route.
- Document delete is not part of issue #81, but the schema and service design
  must leave room for document soft delete and cleanup jobs.
- No route should hard-delete chunks or vectors in a way that bypasses future
  cleanup state.

### Configuration

Use envconfig-style structured config in `internal/config`:

- `KNOWLEDGE_HTTP_ADDR`
- `KNOWLEDGE_SERVICE_VERSION`
- `KNOWLEDGE_ENV`
- `DATABASE_URL`
- `KNOWLEDGE_SHUTDOWN_TIMEOUT`

PostgreSQL is the authoritative runtime backend for this task. A memory
repository may exist only as a test helper for service/handler unit tests, not
as the production delivery path or a substitute for the PostgreSQL repository.

## Implementation Plan

1. Recreate service scaffold.
   - `go.mod` with `go 1.25.0`
   - `cmd/server/main.go`
   - config loader
   - health/readiness handlers
   - README/API skeleton
2. Build domain and service layer.
   - Knowledge Base CRUD use cases
   - Document list/detail use cases
   - validation and error classification
   - role/permission-aware request context filtering
3. Build repositories and migration.
   - `sqlc.yaml`, query files, and generated sqlc package
   - PostgreSQL repository adapter and migration
   - memory repository only as a test helper if useful
   - transaction helper if multi-table operations require it
4. Build HTTP layer.
   - DTOs matching gateway OpenAPI names/shape
   - pagination parsing
   - JSON decoding with unknown-field rejection
   - standard envelopes/errors/request ID
5. Add tests.
   - handler tests
   - service tests
   - repository tests
   - optional PostgreSQL integration tests gated by env if no local DB is
     guaranteed
6. Update docs/contracts.
   - service README
   - service-local OpenAPI
   - notes about intentionally deferred upload/parser/vector/retrieval
7. Validate.
   - `git diff --check`
   - `cd services/knowledge && go test ./...`
   - `cd services/knowledge && go build ./cmd/server`

## Decision (ADR-lite)

**Context**: Knowledge was intentionally cleared to avoid old schema and code
misleading the new RAG/Knowledge foundation implementation. Issue #81 is a
P0 foundation task, but explicitly excludes full parsing, embedding, and Qdrant
retrieval.

**Decision**: Rebuild only the Knowledge metadata/status vertical slice first:
service scaffold, PostgreSQL-backed knowledge base, document status, processing
job, and document chunk schema, `pgx` + `sqlc` repository access, basic
role/permission-aware filtering, soft delete, and docs-compatible HTTP
contracts.

**Consequences**: The service becomes runnable and useful for frontend/admin
state display quickly. Upload/pipeline/vector/retrieval work remains cleanly
separated for follow-up tasks. Some existing Knowledge docs that describe the
removed implementation must be treated as design intent until updated by this
task.

## Research References

- [`research/docs-contract-review.md`](research/docs-contract-review.md) -
  issue #81 and docs contract alignment notes.

## Technical Notes

- A-09 depends on upstream work noted in the issue as S-01 and A-02.
- Branch metadata has been set to `L1nggTeam/feat/knowledge-core-api`, PR base
  to `develop`, and scope to `services/knowledge`.
- Current Git branch is `L1nggTeam/feat/knowledge-core-api`.
- `gh` CLI was unavailable locally; issue content was fetched through GitHub's
  public API endpoint with `curl`.
- Do not resurrect old `services/knowledge` files from before the clear commit
  unless a specific piece is intentionally reintroduced after review.
- Knowledge intentionally uses `go 1.25.0` for this new service module to keep
  the future RAG MCP server integration path open. If the local toolchain cannot
  build Go 1.25.0 during implementation, report the toolchain mismatch instead
  of silently downgrading the module version.

## Implementation Result

- Rebuilt `services/knowledge` as a Go 1.25.0 service-local module.
- Implemented operational routes, Knowledge Base CRUD routes, document list, and
  document detail.
- Added standard request ID propagation, success/page/error envelopes, JSON body
  validation, pagination validation, and gateway user context extraction.
- Implemented `knowledge:read` / `knowledge:write` plus admin-role aware access
  filtering without inventing extra permission strings.
- Added PostgreSQL migration for `knowledge_bases`, `knowledge_documents`,
  `processing_jobs`, and `document_chunks`.
- Added `sqlc.yaml`, SQL query files, generated `internal/repository/sqlc`
  code, and a `pgxpool` PostgreSQL repository adapter.
- Added memory repository only as a handler/service/repository test helper.
- Updated `docs/architecture/technology-decisions.md` with the Knowledge
  service-level `go 1.25.0` deviation.

## Validation Result

Commands run:

```bash
cd services/knowledge && sqlc generate
cd services/knowledge && go test ./...
cd services/knowledge && go build ./cmd/server
cd services/knowledge && go build -o /tmp/knowledge-server ./cmd/server
git diff --check
python3 ./.trellis/scripts/task.py validate .trellis/tasks/06-29-knowledge-core-api
```

Notes:

- The environment initially had no `go` on `PATH`, so Go 1.25.0 was installed
  under `/home/l1ngg/.cache/codex-tools/go1.25.0` for validation.
- `proxy.golang.org` timed out during dependency download; validation succeeded
  with `GOPROXY=https://goproxy.cn,direct` and
  `GOSUMDB=sum.golang.google.cn`.
