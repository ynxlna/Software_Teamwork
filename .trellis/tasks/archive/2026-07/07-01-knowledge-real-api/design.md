# Design

## API Boundary

Create `apps/web/src/api/knowledge.ts` as the Knowledge-specific frontend API module.

- Import generated `paths` and `components` from `apps/web/src/api/generated/gateway.ts`.
- Derive request and response payload types from OpenAPI operations with helper aliases.
- Keep Gateway envelope handling in `apps/web/src/api/client.ts`.
- Keep generated code untouched.
- Re-export Knowledge functions from `apps/web/src/api/admin.ts` only for compatibility; new Knowledge callers should import `@/api/knowledge`.

## Data Flow

UI page -> Knowledge React Query hook -> `api/knowledge.ts` operation wrapper -> shared Gateway client -> `/api/v1/**` Gateway path -> Knowledge owner service through Gateway.

Binary original document content uses `gatewayFileRequest` because OpenAPI returns `application/octet-stream`, not a JSON envelope.

## Error Flow

The shared client maps Gateway error envelopes into `ApiError`. Knowledge UI calls `getGatewayCapabilityIssue` to classify:

- `401/unauthorized`: auth expired.
- `403/forbidden`: permission denied.
- `501/not_implemented`: active contract, backend workflow not ready.
- `502/dependency_error`: downstream dependency failure.
- Other `ApiError`: generic Gateway failure with request ID if present.

## Test Strategy

- API wrapper tests stub `fetch` at the browser API boundary and verify URL/method/body/envelope behavior.
- Search page test mocks `@/api/knowledge` so the component is tested at the feature API boundary, not by mocking generated OpenAPI internals.
- Existing capability helper tests remain pure unit coverage.

## Trade-offs

- Use operation-derived type aliases in the wrapper instead of introducing a generated runtime client. The repo already has a shared fetch wrapper and OpenAPI type generation, so this keeps the change scoped while adding compile-time drift checks.
- Keep existing page composition and UI patterns to avoid unrelated frontend redesign.
