# Docs Contract Review For A-09 Knowledge Core API

## Sources Reviewed

- GitHub issue #81: `[A-09] Knowledge 知识库与文档状态基础 API`
- `docs/services/knowledge/README.md`
- `docs/services/knowledge/docs/api-contract.md`
- `docs/services/knowledge/docs/data-models.md`
- `docs/services/knowledge/docs/implementation.md`
- `docs/services/gateway/api/openapi.yaml`
- `docs/architecture/service-boundaries.md`
- `docs/architecture/frontend-backend-contract.md`
- `docs/architecture/technology-decisions.md`
- `.trellis/spec/backend/*.md`
- `services/file/**` for current Go service style

## Issue #81 Summary

Issue #81 is task `A-09`, owned by `L1nggTeam`, priority `P0`, module
`knowledge`, labels `backend` and `service:knowledge`, assigned to `L-1ngg`.

Required scope:

- Implement knowledge base list, create, detail, update, and delete.
- Implement document list within a knowledge base.
- Implement `GET /documents/{documentId}` processing detail.
- Persist and expose `DocumentStatus`: `uploaded`, `parsing`, `chunking`,
  `embedding`, `ready`, `failed`.
- Apply basic access filtering from user context.
- Preserve failure reason, statistics fields, and timestamps.

Explicitly out of scope:

- Full parsing pipeline.
- Embedding generation.
- Qdrant retrieval.

Acceptance from issue:

- `go test ./...` passes inside `services/knowledge`.
- Pagination, error envelope, and request ID follow docs.
- Delete does not bypass chunks/index cleanup flow.

## Contract Conclusions

### Public Gateway Contract

`docs/services/gateway/api/openapi.yaml` is the public contract authority. The
service-local implementation should expose equivalent `/internal/v1/**` routes
for gateway to proxy later.

MVP public routes that must have service-local counterparts:

- `GET /api/v1/knowledge-bases`
- `POST /api/v1/knowledge-bases`
- `GET /api/v1/knowledge-bases/{knowledgeBaseId}`
- `PATCH /api/v1/knowledge-bases/{knowledgeBaseId}`
- `DELETE /api/v1/knowledge-bases/{knowledgeBaseId}`
- `GET /api/v1/knowledge-bases/{knowledgeBaseId}/documents`
- `GET /api/v1/documents/{documentId}`

The gateway contract also documents upload, document metadata update/delete,
chunks, content, and retrieval routes. Those are not required by issue #81
except that delete must not make future chunk/index cleanup impossible.

### Response Envelope

Project-owned JSON success responses use:

```json
{ "data": {}, "requestId": "req_123" }
```

Paginated responses use:

```json
{
  "data": [],
  "page": { "page": 1, "pageSize": 20, "total": 0 },
  "requestId": "req_123"
}
```

Errors use:

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123",
    "fields": {}
  }
}
```

`204 No Content` returns no JSON body.

### Authentication Context

Gateway injects:

- `X-Request-Id`
- `X-User-Id`
- `X-User-Roles`
- `X-User-Permissions`
- `X-Forwarded-For`
- `X-Forwarded-Proto`

Service-local business routes should require `X-User-Id`. Missing user context
returns `401 unauthorized`. The docs expose `X-User-Roles` and
`X-User-Permissions`, so A-09 should not be pure owner-only filtering. The MVP
should:

- parse roles and permissions from gateway headers;
- allow reads for caller-created resources by default;
- allow broader reads only for an admin role or explicit knowledge read
  permission;
- require an admin role or explicit knowledge write permission for
  create/update/delete;
- return `404 not_found` for hidden resources and `403 forbidden` for
  authenticated callers attempting unauthorized mutations.

### Data Model

Docs define a broader future model:

- `knowledge_bases`
- `knowledge_documents`
- `processing_jobs`
- `document_chunks`

Issue #81 only needs durable metadata and status, but delete must not bypass
future chunks/index cleanup. Recommended MVP tables:

- `knowledge_bases`
- `knowledge_documents`
- `processing_jobs`
- `document_chunks`

`document_chunks` should be created in the first migration as a provenance and
cleanup anchor. A-09 should not implement chunk writing/listing, embedding, or
Qdrant indexing; the table exists to keep the data model and delete cleanup path
aligned with docs.

Recommended soft delete fields:

- `deleted_at` on `knowledge_bases`
- `deleted_at` on `knowledge_documents`

Soft delete avoids bypassing future chunk/index cleanup and keeps retrieval
from returning deleted resources after future features are added.

### Technology Choices

Docs prefer:

- Go 1.22-era service-local module patterns in the current baseline docs.
- `net/http` / `http.ServeMux`.
- `log/slog`.
- envconfig-style config loading.
- PostgreSQL via `pgx` + `sqlc`.
- migrations via `goose`.

For Knowledge specifically, the user has explicitly chosen `go 1.25.0` so the
service starts from a newer module baseline for later RAG MCP server work. Treat
this as a per-service deviation from older docs, not as a docs contradiction to
resolve by downgrading.

Current `services/file` is simpler and uses hand-written repository code with
memory storage only. Because A-09 explicitly requires PostgreSQL as the
authority source and the docs require `pgx` + `sqlc`, Knowledge should include
`sqlc.yaml`, query files, generated sqlc code, a PostgreSQL repository adapter,
and a migration from the start. A memory repository can still be useful for
service and handler tests, but it must not be the runtime delivery path.

### Important Stale Docs

`docs/services/knowledge/docs/implementation.md` describes a previous Go
implementation that has now been intentionally cleared from `services/knowledge`.
Treat it as architecture intent and historical reference, not as current code.

The new implementation must rebuild `services/knowledge` from an empty service
directory while following the current docs and backend spec.

## Feasible MVP Approach

Recommended approach: rebuild a narrow but complete Knowledge metadata service.

1. Recreate the Go service scaffold:
   - `go.mod` declaring `go 1.25.0`, `cmd/server/main.go`,
     `internal/config`, `internal/http`, `internal/service`,
     `internal/repository`, `migrations`, `api/openapi.yaml`, `README.md`.
2. Implement service-local internal routes matching the A-09 public contract:
   - health/readiness
   - knowledge base CRUD
   - document list by knowledge base
   - document detail by ID
3. Persist metadata in PostgreSQL:
   - knowledge bases
   - knowledge documents
   - processing jobs for status traceability
   - document chunks as the future cleanup/hydration anchor
4. Add `pgx` + `sqlc` repository access with generated query code and a thin
   domain adapter.
5. Add memory repository only as a unit/handler test helper if useful.
6. Add migration and repository tests where local PostgreSQL test tooling is
   available; otherwise keep repository unit coverage around memory adapter and
   document the integration-test gate.

## Risks And Follow-Ups

- Branch alignment has been resolved: implementation branch is
  `L1nggTeam/feat/knowledge-core-api`.
- `services/knowledge` currently only contains `.gitkeep`; all service files
  need to be recreated.
- Gateway proxy implementation is not part of A-09 unless later requested.
- Complete upload, parsing, chunking, embedding, Qdrant, and retrieval should be
  separate follow-up tasks.
- Local validation may require a Go 1.25.0-capable toolchain. If unavailable,
  implementation should report the environment gap instead of changing the
  planned module version.
