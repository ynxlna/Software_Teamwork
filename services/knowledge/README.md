# Knowledge Service

`services/knowledge` is the Go microservice that owns knowledge ingestion state,
chunks, embeddings, Qdrant indexing, and retrieval coordination. The previous
Python/FastAPI prototype has been removed from this service directory so future
work happens against the repository standard Go service layout.

Frontend callers must not call this service directly. Public routes stay behind
gateway and are documented in `docs/services/gateway/api/openapi.yaml`.

## Current Scope

Implemented now:

- `GET /healthz`
- `GET /readyz`
- `GET /internal/v1/knowledge-bases`
- `POST /internal/v1/knowledge-bases`
- `GET /internal/v1/knowledge-bases/{knowledgeBaseId}`
- `PATCH /internal/v1/knowledge-bases/{knowledgeBaseId}`
- `DELETE /internal/v1/knowledge-bases/{knowledgeBaseId}`
- `GET /internal/v1/knowledge-bases/{knowledgeBaseId}/documents`
- `POST /internal/v1/knowledge-bases/{knowledgeBaseId}/ingestion-jobs`
- `POST /internal/v1/knowledge-bases/{knowledgeBaseId}/jobs`
- `GET /internal/v1/documents/{documentId}`
- `GET /internal/v1/documents/{documentId}/chunks`
- `GET /internal/v1/jobs/{jobId}`
- `POST /internal/v1/jobs/{jobId}/processing-runs`
- `POST /internal/v1/knowledge-queries`
- `GET /internal/v1/runtime-config`
- `PATCH /internal/v1/runtime-config`
- `GET /internal/v1/knowledge-stats`
- Go service-local module, HTTP server, configuration loading, response
  envelope, error envelope, memory/PostgreSQL repositories, parser/chunker,
  embedding/vector adapters, tests, Dockerfile, and service-local OpenAPI.

Next migration slices:

- Real asynchronous worker/queue execution for ingestion and reprocessing.
- Production embedding provider adapter and rerank provider adapter.
- File service handoff integration after upload.
- Gateway exposure for selected P1 admin endpoints after public OpenAPI review.

Out of scope for this baseline:

- File upload ownership.
- Public frontend exposure for internal runtime config/statistics endpoints.

## Local Run

```bash
cd services/knowledge
go test ./...
go build ./cmd/server
KNOWLEDGE_HTTP_ADDR=:8000 go run ./cmd/server
```

Check the service:

```bash
curl http://localhost:8000/healthz
curl http://localhost:8000/readyz
```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `KNOWLEDGE_HTTP_ADDR` | `:8000` | HTTP listen address. |
| `KNOWLEDGE_SERVICE_VERSION` | `0.3.0` | Service version shown by readiness. |
| `KNOWLEDGE_ENV` | `local` | Runtime environment label. |
| `KNOWLEDGE_STORAGE_BACKEND` | `memory` | Metadata backend. Supported values: `memory`, `postgres`. |
| `DATABASE_URL` | unset | PostgreSQL connection string required when `KNOWLEDGE_STORAGE_BACKEND=postgres`. |
| `FILE_SERVICE_BASE_URL` | unset | Optional File Service base URL used by ingestion pipeline source reads. |
| `KNOWLEDGE_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout. |
| `EMBEDDING_PROVIDER` | `local_hashing` | Embedding provider label for readiness and future pipeline wiring. |
| `EMBEDDING_MODEL` | `local_hashing` | Embedding model label. |
| `EMBEDDING_DIMENSION` | `384` | Embedding vector dimension. |
| `QDRANT_URL` | unset | Optional Qdrant REST base URL. When unset, the service uses an in-memory vector index for local tests. |
| `QDRANT_COLLECTION` | `knowledge_chunks` | Qdrant collection name for vector indexing and retrieval. |

## Response Shape

JSON success responses use:

```json
{
  "data": {},
  "requestId": "req_123"
}
```

JSON errors use:

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123"
  }
}
```

The service must not return SQL details, object keys, raw vectors, prompts,
tokens, API keys, or internal URLs in HTTP responses.

## Contract Notes

Gateway active public Knowledge operations remain:

- `GET /api/v1/knowledge-bases`
- `POST /api/v1/knowledge-bases`
- `GET /api/v1/knowledge-bases/{knowledgeBaseId}`
- `PATCH /api/v1/knowledge-bases/{knowledgeBaseId}`
- `DELETE /api/v1/knowledge-bases/{knowledgeBaseId}`
- `GET /api/v1/knowledge-bases/{knowledgeBaseId}/documents`
- `GET /api/v1/documents/{documentId}`
- `GET /api/v1/documents/{documentId}/chunks`
- `POST /api/v1/knowledge-queries`

Service-to-service implementation routes will live under `/internal/v1/**` as
they are migrated. Do not revive older `/api/v1/knowledge/...` or action-suffix
paths such as `:retry` as stable public API.

## Internal Knowledge Base API

Business routes require gateway-injected user context:

```text
X-User-Id: usr_123
X-Request-Id: req_123
```

Create a knowledge base:

```bash
curl -X POST http://localhost:8000/internal/v1/knowledge-bases \
  -H 'Content-Type: application/json' \
  -H 'X-User-Id: usr_123' \
  -d '{"name":"General","docType":"GENERAL"}'
```

List knowledge bases:

```bash
curl 'http://localhost:8000/internal/v1/knowledge-bases?page=1&pageSize=20&keyword=general&docType=GENERAL' \
  -H 'X-User-Id: usr_123'
```
