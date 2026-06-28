# Knowledge Go 微服务迁移设计

## Architecture

`services/knowledge` 正式实现采用 service-local Go module，遵循 `.trellis/spec/backend/directory-structure.md`：

```text
services/knowledge/
├── go.mod
├── go.sum
├── cmd/server/main.go
├── internal/
│   ├── config/
│   ├── http/
│   ├── service/
│   ├── repository/
│   └── platform/
├── api/openapi.yaml
├── migrations/
├── Dockerfile
└── README.md
```

现有 Python/FastAPI 原型只作为迁移参考。Go 服务的 HTTP 契约、DTO、错误码和边界以 gateway OpenAPI、`docs/services/knowledge.md` 和 backend spec 为准。

## Boundaries

- Gateway owns public `/api/v1/**` entrypoint, response normalization and auth context propagation.
- Knowledge owns knowledge bases, document processing state, chunks, embeddings, Qdrant points and retrieval queries.
- File owns raw upload, original object lifecycle, source content reads and file-owned metadata updates.
- QA and Document consume Knowledge retrieval through HTTP contracts and do not read/write Qdrant directly.

## Data Flow

1. Gateway or File receives document upload through the file-owned public path.
2. File persists original file metadata/object.
3. File or gateway calls a Knowledge internal handoff resource with file reference and knowledge base context.
4. Knowledge creates an ingestion job and document processing record.
5. Worker/service parses content, chunks text, embeds chunks and writes Qdrant points.
6. Knowledge persists final document/chunk status and exposes document detail/chunk/query APIs to gateway.

## Compatibility Notes

- Keep stable public paths as `/api/v1/knowledge-bases`, `/api/v1/documents/{documentId}`, `/api/v1/documents/{documentId}/chunks`, `/api/v1/knowledge-queries`.
- Do not revive older `/api/v1/knowledge/...` or action suffix paths as stable public API.
- If Python parsing code remains temporarily useful, call it through an adapter/worker boundary rather than exposing Python service contracts to frontend or gateway as final API.

## Risks

- Python prototype currently contains the most complete local ingestion logic; migrating to Go may temporarily reduce capability. Mitigation: migrate by vertical slices and keep reference behavior documented.
- Gateway and File handoff are not fully implemented yet. Mitigation: start with internal HTTP resource contracts and memory/PostgreSQL-friendly ports so wiring can evolve.
- Qdrant and embedding provider setup can slow MVP. Mitigation: keep platform clients behind service-owned interfaces and allow a deterministic local embedding adapter for tests.
