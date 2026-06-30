# F-017 Knowledge Frontend Real Gateway API

## Goal

Implement issue #303: move the Knowledge frontend production path onto the active public Gateway API for document lifecycle, original content/chunks, and `knowledge-queries` retrieval, with visible loading, empty, error, permission, and not-ready states.

## Confirmed Facts

- Issue: https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/303.
- PR target must be `Sakayori-Iroha-168/Software_Teamwork:develop`.
- Frontend source lives under `apps/web/src/`.
- Gateway public OpenAPI source is `docs/services/gateway/api/public.openapi.yaml`.
- Generated frontend types live at `apps/web/src/api/generated/gateway.ts` and are produced by `bun run --cwd apps/web api:generate`.
- Active Knowledge Gateway paths are present in OpenAPI and `apps/web/src/api/active-paths.ts`:
  - `GET/POST /api/v1/knowledge-bases`
  - `GET/PATCH/DELETE /api/v1/knowledge-bases/{knowledgeBaseId}`
  - `GET/POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents`
  - `GET/PATCH/DELETE /api/v1/documents/{documentId}`
  - `GET /api/v1/documents/{documentId}/chunks`
  - `GET /api/v1/documents/{documentId}/content`
  - `POST /api/v1/knowledge-queries`
- Existing UI already has pages for Knowledge base management, documents, chunks, and search; the remaining gap is to make the API boundary explicitly operation-typed and test drift/error handling.

## Requirements

1. Knowledge API calls must use the public Gateway transport only; browser code must not call Knowledge internal services or inactive action-style search paths.
2. Knowledge API wrappers must be typed from generated OpenAPI `paths`/schemas, not hand-maintained DTO duplicates.
3. Knowledge base detail, document list, upload, status refresh, original content download, chunks, and `knowledge-queries` retrieval must use these wrappers.
4. Active Gateway route failures must surface visible UI states. `403`, `401`, `501/not_implemented`, and `502/dependency_error` must remain distinguishable.
5. Production UI must not render mock, fake, sample, silent fallback, or stale success data for active Knowledge routes.
6. Add focused tests that prove:
   - Knowledge wrappers call Gateway paths with the expected method, query, envelope, and FormData behavior.
   - Failed retrieval clears stale success data and shows the Gateway capability error.
   - Existing capability classifier covers readiness, dependency, permission, and missing request ID cases.

## Out Of Scope

- Changing Knowledge or Gateway backend contracts.
- Implementing QA RAG end-to-end acceptance; issue #304 owns that.
- Treating API-boundary mocks as cross-service smoke.
- Adding real upload progress events when the Gateway transport only exposes request completion.

## Acceptance Criteria

- [x] Frontend Knowledge-related production requests go through Gateway typed client/wrappers.
- [x] Document upload, document status/list, document content/chunks, and `knowledge-queries` have success and failure coverage.
- [x] Active Knowledge routes do not use mock/fallback data.
- [x] Capability-not-ready, permission-denied, unauthorized, dependency, and generic API failures have understandable UI/error states.
- [x] `bun run --cwd apps/web check` passes.
- [x] Relevant frontend tests pass.
- [x] `bun run --cwd apps/web build` and `git diff --check` pass before PR.

## Verification Evidence

- `bun run --cwd apps/web api:generate` and `git diff --exit-code -- apps/web/src/api/generated/gateway.ts` passed.
- `bun run --cwd apps/web test:unit -- src/api/knowledge.test.ts src/pages/knowledge/search/page.test.tsx src/features/knowledge/capability.test.ts` passed.
- `bun run --cwd apps/web test:unit` passed: 15 files, 38 tests.
- `bun run --cwd apps/web check` passed.
- `bun run --cwd apps/web build` passed; Vite reported only the existing large bundle warning.
- `bun run --cwd apps/web test:e2e` passed: 6 tests.
- `git diff --check` passed.

## Validation Plan

- `bun run --cwd apps/web api:generate`
- `bun run --cwd apps/web test:unit -- src/api/knowledge.test.ts src/pages/knowledge/search/page.test.tsx src/features/knowledge/capability.test.ts`
- `bun run --cwd apps/web check`
- `bun run --cwd apps/web build`
- `git diff --check`
