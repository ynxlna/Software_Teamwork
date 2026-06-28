# Knowledge Service

`services/knowledge` is the Go microservice that owns knowledge ingestion state,
chunks, embeddings, Qdrant indexing, and retrieval coordination. The previous
Python/FastAPI prototype has been removed from this service directory so future
work happens against the repository standard Go service layout.

Frontend callers must not call this service directly. Public routes stay behind
gateway and are documented in `docs/api/gateway.openapi.yaml`.

## Current Scope

Implemented now:

- `GET /healthz`
- `GET /readyz`
- Go service-local module, HTTP server, configuration loading, response
  envelope, error envelope, tests, Dockerfile, and service-local OpenAPI
  baseline.

Next migration slices:

- Knowledge base metadata and DTO alignment.
- Document processing state, chunks, and ingestion jobs.
- File -> Knowledge handoff.
- Qdrant-backed `knowledge-queries` retrieval.
- Gateway proxy and contract tests.

Out of scope for this baseline:

- PostgreSQL persistence.
- Qdrant writes and retrieval.
- File upload ownership.
- Public frontend routing through gateway.

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
| `KNOWLEDGE_STORAGE_BACKEND` | `memory` | Baseline metadata/storage backend. Only `memory` is implemented now. |
| `KNOWLEDGE_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout. |
| `EMBEDDING_PROVIDER` | `local_hashing` | Embedding provider label for readiness and future pipeline wiring. |
| `EMBEDDING_MODEL` | `local_hashing` | Embedding model label. |
| `EMBEDDING_DIMENSION` | `384` | Embedding vector dimension. |
| `QDRANT_COLLECTION` | `knowledge_chunks` | Qdrant collection name for future retrieval slices. |

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
