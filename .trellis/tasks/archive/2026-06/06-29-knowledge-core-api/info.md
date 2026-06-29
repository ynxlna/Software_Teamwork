# Technical Design: A-09 Knowledge Core API

## Design Target

Build a narrow Knowledge metadata/status service that is independently runnable
and testable. The service must support Knowledge Base CRUD and document status
reads now, while preserving clean extension points for later upload, parsing,
chunking, embedding, Qdrant indexing, and retrieval.

## Route Plan

Service-local routes under `services/knowledge`:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/healthz` | liveness |
| `GET` | `/readyz` | readiness/config summary |
| `GET` | `/internal/v1/knowledge-bases` | paginated list |
| `POST` | `/internal/v1/knowledge-bases` | create |
| `GET` | `/internal/v1/knowledge-bases/{knowledgeBaseId}` | detail |
| `PATCH` | `/internal/v1/knowledge-bases/{knowledgeBaseId}` | metadata/strategy update |
| `DELETE` | `/internal/v1/knowledge-bases/{knowledgeBaseId}` | soft delete |
| `GET` | `/internal/v1/knowledge-bases/{knowledgeBaseId}/documents` | paginated document status list |
| `GET` | `/internal/v1/documents/{documentId}` | document processing detail |

These mirror the active gateway paths in `docs/services/gateway/api/openapi.yaml`
while staying internal-service shaped.

## Package Layout

```text
services/knowledge/
├── go.mod
├── sqlc.yaml
├── cmd/server/main.go
├── internal/config/config.go
├── internal/http/context.go
├── internal/http/knowledge_base_dto.go
├── internal/http/document_dto.go
├── internal/http/paginated.go
├── internal/http/response.go
├── internal/http/server.go
├── internal/http/server_test.go
├── internal/service/context.go
├── internal/service/errors.go
├── internal/service/knowledge_base.go
├── internal/service/document.go
├── internal/service/status.go
├── internal/service/*_test.go
├── internal/repository/postgres.go
├── internal/repository/memory.go
├── internal/repository/queries/knowledge.sql
├── internal/repository/sqlc/
├── internal/repository/*_test.go
├── migrations/0001_create_knowledge_core_tables.sql
├── api/openapi.yaml
└── README.md
```

`go.mod` must declare `go 1.25.0`. This is a deliberate per-service choice for
Knowledge so the later RAG MCP server work does not start from an older module
baseline. Existing Go 1.22-era service patterns remain useful for structure,
but they do not override this module version decision.

`platform/` is not needed in A-09 because no external File/Redis/Qdrant/AI
client is implemented in this slice.

## Domain Types

### KnowledgeBase

Fields:

- `ID`
- `Name`
- `Description`
- `DocType`
- `ChunkStrategy`
- `RetrievalStrategy`
- `DocumentCount`
- `ChunkCount`
- `CreatedBy`
- `CreatedAt`
- `UpdatedAt`
- `DeletedAt`

`DocumentCount` and `ChunkCount` should be derived in repository queries rather
than stored as counters unless implementation discovers a strong reason.

### KnowledgeDocument

Fields:

- `ID`
- `KnowledgeBaseID`
- `FileRef`
- `Name`
- `ContentType`
- `SizeBytes`
- `Status`
- `ErrorCode`
- `ErrorMessage`
- `ChunkCount`
- `Tags`
- `ParserBackend`
- `CreatedBy`
- `CurrentJobID`
- `CreatedAt`
- `UpdatedAt`
- `DeletedAt`

### ProcessingJob

MVP stores processing/job trace state for future upload and reprocessing flows:

- `ID`
- `KnowledgeBaseID`
- `DocumentID`
- `JobType`
- `Status`
- `CurrentStage`
- `ProgressPercent`
- `Message`
- `ErrorCode`
- `ErrorMessage`
- `Attempts`
- `MaxAttempts`
- `StartedAt`
- `FinishedAt`
- `CreatedAt`
- `UpdatedAt`

No worker is required in A-09.

### DocumentChunk

A-09 does not implement chunk creation, chunk listing, embedding, or Qdrant
indexing. It still creates the `document_chunks` table because the docs define
chunks as part of the Knowledge-owned data model and issue #81 requires delete
not to bypass future chunk/index cleanup.

Fields:

- `ID`
- `KnowledgeBaseID`
- `DocumentID`
- `ChunkIndex`
- `SectionPath`
- `Content`
- `TokenCount`
- `ChunkType`
- `QdrantPointID`
- `EmbeddingProvider`
- `EmbeddingModel`
- `EmbeddingDimension`
- `Metadata`
- `CreatedAt`

## PostgreSQL Migration

Create these tables:

### `knowledge_bases`

Columns:

- `id text primary key`
- `name text not null`
- `description text not null default ''`
- `doc_type text not null default 'GENERAL'`
- `chunk_strategy jsonb not null default '{}'::jsonb`
- `retrieval_strategy jsonb not null default '{}'::jsonb`
- `created_by text not null`
- `created_at timestamptz not null default now()`
- `updated_at timestamptz not null default now()`
- `deleted_at timestamptz`

Indexes:

- `idx_knowledge_bases_created_by_created_at`
- `idx_knowledge_bases_deleted_at`
- `idx_knowledge_bases_doc_type`

### `knowledge_documents`

Columns:

- `id text primary key`
- `knowledge_base_id text not null references knowledge_bases(id)`
- `file_ref text`
- `name text not null`
- `content_type text`
- `size_bytes bigint`
- `status text not null`
- `error_code text`
- `error_message text`
- `tags jsonb not null default '[]'::jsonb`
- `parser_backend text`
- `current_job_id text`
- `created_by text not null`
- `created_at timestamptz not null default now()`
- `updated_at timestamptz not null default now()`
- `deleted_at timestamptz`

Indexes:

- `idx_knowledge_documents_kb_created_at`
- `idx_knowledge_documents_created_by_created_at`
- `idx_knowledge_documents_status`
- `idx_knowledge_documents_deleted_at`
- `idx_knowledge_documents_current_job_id`

### `processing_jobs`

Columns:

- `id text primary key`
- `knowledge_base_id text not null references knowledge_bases(id)`
- `document_id text references knowledge_documents(id)`
- `job_type text not null`
- `status text not null`
- `current_stage text`
- `progress_percent integer not null default 0`
- `message text`
- `error_code text`
- `error_message text`
- `attempts integer not null default 0`
- `max_attempts integer not null default 3`
- `started_at timestamptz`
- `finished_at timestamptz`
- `created_at timestamptz not null default now()`
- `updated_at timestamptz not null default now()`

Indexes:

- `idx_processing_jobs_kb_created_at`
- `idx_processing_jobs_document_id`
- `idx_processing_jobs_status_created_at`

### `document_chunks`

Columns:

- `id text primary key`
- `knowledge_base_id text not null references knowledge_bases(id)`
- `document_id text not null references knowledge_documents(id)`
- `chunk_index integer not null`
- `section_path text`
- `content text not null default ''`
- `token_count integer`
- `chunk_type text`
- `qdrant_point_id text`
- `embedding_provider text`
- `embedding_model text`
- `embedding_dimension integer`
- `metadata jsonb not null default '{}'::jsonb`
- `created_at timestamptz not null default now()`

Constraints and indexes:

- `uniq_document_chunks_document_id_chunk_index`
- `idx_document_chunks_document_id_chunk_index`
- `idx_document_chunks_knowledge_base_id`
- `idx_document_chunks_qdrant_point_id`

The table exists for provenance, future retrieval hydration, and cleanup
coordination. A-09 should not write real chunks unless a test seed needs them to
verify delete/read exclusion behavior.

## PostgreSQL Access Plan

Use `pgx` + `sqlc` as required by the project technology baseline:

- `sqlc.yaml` at `services/knowledge/sqlc.yaml`.
- query files under `internal/repository/queries/`.
- generated code under `internal/repository/sqlc/`.
- repository adapters wrap generated query methods and return domain structs.
- service and HTTP layers must not import generated row types.
- SQL must use explicit column lists and parameter binding.

No ORM should be introduced.

## Repository Boundary

Repository methods should return domain structs, not raw DB rows:

- `CreateKnowledgeBase`
- `ListKnowledgeBases`
- `GetKnowledgeBase`
- `UpdateKnowledgeBase`
- `SoftDeleteKnowledgeBase`
- `ListDocumentsByKnowledgeBase`
- `GetDocument`

Delete needs transaction safety:

- mark the knowledge base deleted;
- exclude documents under deleted knowledge bases from reads;
- mark documents under the deleted knowledge base as deleted in the same
  transaction, or otherwise guarantee document reads join against non-deleted
  knowledge bases;
- leave `document_chunks` rows available for a future cleanup job instead of
  hard-deleting them in the metadata route.

Do not perform external calls in repository transactions.

## Service Rules

Validation:

- ID path values must be non-empty.
- `name` is required for create and cannot be blank when patched.
- `description` defaults to empty string.
- `docType` defaults to `GENERAL`.
- `page >= 1`.
- `pageSize` uses gateway-compatible defaults and maximums.
- document `status` filter must be one of the public `DocumentStatus` values.

Access:

- require `X-User-Id` on business routes;
- parse `X-User-Roles` and `X-User-Permissions`;
- create/update/delete require an admin role or explicit knowledge write
  permission;
- create resources with `CreatedBy = X-User-Id`;
- read operations return caller-created resources by default;
- read operations may include broader resources only with an admin role or
  explicit knowledge read permission;
- hidden/deleted resources return `not_found`;
- authenticated callers lacking mutation permission receive `forbidden`.

Status:

- public document statuses are exactly `uploaded`, `parsing`, `chunking`,
  `embedding`, `ready`, `failed`;
- internal job stages can use richer values later, but should not leak as
  `DocumentStatus`.

## HTTP Layer

Patterns:

- use `http.ServeMux` method patterns;
- set and propagate `X-Request-Id`;
- recover panics at the HTTP boundary;
- decode JSON with `DisallowUnknownFields`;
- reject multiple JSON objects in one request body;
- return standard success, paginated, and error envelopes;
- do not return SQL errors, object keys, stack traces, full document content, or
  internal URLs.

DTOs should match gateway schemas:

- `CreateKnowledgeBaseRequest`
- `UpdateKnowledgeBaseRequest`
- `KnowledgeBaseSummary`
- `DocumentSummary`
- `PageInfo`

## Testing Plan

Handler tests:

- health/readiness envelope and request ID;
- create/list/get/update/delete knowledge base;
- list documents by knowledge base;
- get document detail;
- missing `X-User-Id`;
- invalid pagination;
- invalid JSON body;
- not found/hidden/deleted resources.

Service tests:

- defaults and validation;
- role/permission-aware access filtering;
- soft delete exclusion;
- document status enum validation;
- pagination boundaries.

Repository tests:

- memory repository coverage only as a service/handler test helper;
- PostgreSQL/sqlc repository unit or integration coverage where available;
- PostgreSQL integration tests gated by an env var if local DB is unavailable;
- migration should be syntactically reviewable and explicit.

Validation commands:

```bash
git diff --check
cd services/knowledge && go test ./...
cd services/knowledge && go build ./cmd/server
```

## Implementation Notes

- Start from the empty `services/knowledge/.gitkeep` directory.
- Remove `.gitkeep` once real files exist.
- Create `go.mod` with `go 1.25.0`; if the local Go toolchain cannot validate
  that version during implementation, record the toolchain mismatch rather than
  downgrading the service.
- Prefer copying proven patterns from `services/file` only after checking they
  still match the Knowledge docs and backend specs.
- Do not revive the old removed Knowledge implementation wholesale.
- The branch decision is resolved: implementation should happen on
  `L1nggTeam/feat/knowledge-core-api`.
- Implementation has started and the task is now `in_progress`.
- Real `sqlc generate` succeeded after simplifying the knowledge-base update
  query to an `:execrows` write followed by repository-level detail reload.
