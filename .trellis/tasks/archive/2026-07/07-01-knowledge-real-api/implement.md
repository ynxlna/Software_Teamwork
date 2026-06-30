# Implementation Plan

1. Add typed Knowledge Gateway wrapper module.
2. Move Knowledge hooks/pages to the typed wrapper.
3. Keep `api/admin.ts` compatibility by re-exporting Knowledge functions and removing duplicate generic Knowledge implementations.
4. Add API wrapper unit tests for list/upload/chunks/content/query success and failure behavior.
5. Add Knowledge search page component tests for success state and failed query clearing prior results.
6. Run API generation and frontend quality gates.
7. Commit, push to fork branch, create PR to `develop`, then track CI/review feedback.

## Risk Points

- `KnowledgeQueryRequest` generated fields `topK`, `scoreThreshold`, and `rerank` are required after OpenAPI generation even though the contract gives defaults; UI must provide them.
- `FormData` upload must not force JSON `Content-Type`.
- Query params should remain omitted when undefined.
- Existing unavailable dependencies may block real cross-service smoke; tests should only claim frontend API-boundary coverage.
