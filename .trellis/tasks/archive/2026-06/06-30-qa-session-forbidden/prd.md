# QA 403 And Active API Authorization Consistency

## Goal

Complete issue #157 by making QA ownership behavior explicit and testable across session and session-owned resource APIs. A live session owned by another authenticated user returns `403 forbidden` for direct session detail, update, and delete operations, while missing, deleted, or hidden child resources return the existing OpenAPI `404 not_found` response without leaking data.

## Requirements

- Keep `qa` as the authorization owner and derive identity only from Gateway-provided `X-User-Id`.
- Return `403 forbidden` when an authenticated non-owner accesses a live QA session through detail, update, or delete.
- Return `404 not_found` for missing or soft-deleted sessions.
- Keep message, response-run, event, tool-call, and citation reads owner-filtered.
- For child resources whose public OpenAPI exposes only `404`, return `404` for missing or non-owned parents instead of an empty success response.
- Preserve `409 conflict` for an owner cancelling a response run that exists but is no longer cancellable; return `404` for a missing or non-owned run.
- Add handler, service, and repository coverage for the authorization/error mapping.
- Align QA README wording with the Gateway OpenAPI behavior. Change Gateway OpenAPI only if inspection finds a remaining contract mismatch.

## Acceptance Criteria

- [x] Non-owner `GET`, `PATCH`, and `DELETE` on `/qa-sessions/{sessionId}` return `403 forbidden`.
- [x] Missing and soft-deleted direct sessions return `404 not_found`.
- [x] Non-owned message/run/citation resources return no protected data and follow their documented `404` behavior.
- [x] A non-owner run cancellation does not return `409`; an owner cancelling a terminal run still does.
- [x] No administrator cross-user bypass is introduced.
- [x] `go test ./...`, `go build ./cmd/server`, `go build ./cmd/agent`, and `git diff --check` pass from `services/qa`/repository root as applicable.

## Definition Of Done

- Tests cover the changed handler/service/repository behavior.
- Public error envelopes remain `{ "error": { "code", "message", "requestId" } }`.
- Documentation describes the 403-versus-404 boundary.
- No unrelated modules or contracts are changed.

## Technical Approach

- Reuse the existing `conversationAccessError` path for direct session operations.
- Add narrow ownership/existence checks only when a collection query returns no rows, preserving valid empty collections for owned resources.
- Distinguish response-run authorization failure from run-state conflict before mapping cancellation errors.
- Keep the ownership probe parameterized inside the existing resource repository boundary; no schema or generated sqlc contract changes are needed.

## Decision (ADR-lite)

**Context:** The Gateway contract explicitly exposes `403` for direct QA session detail/update/delete, but child resource operations generally expose only `404` to hide inaccessible resources.

**Decision:** Preserve that split. Direct access to a known live session is explicit `403`; session-owned child resources remain owner-filtered and hidden with `404` when missing or inaccessible.

**Consequences:** Frontend can render a permission-denied state for direct session navigation without gaining a cross-user administration path. Child-resource IDs do not disclose whether another user's data exists.

## Out Of Scope

- Administrator or support-user cross-account QA access.
- Gateway identity-header injection implementation from parallel issue #153.
- Frontend UI changes.
- New authentication or authorization libraries.
- Unrelated QA Agent, retrieval, or model behavior.

## Technical Notes

- Authoritative contract: `docs/services/gateway/api/openapi.yaml`.
- Service behavior: `docs/services/qa/README.md`.
- Relevant code: `services/qa/internal/http`, `services/qa/internal/service`, and `services/qa/internal/repository`.
- Issue dependencies #149 and #156 are closed; downstream #162 is closed; parallel route work #153 remains open.
- `private/doc-update-tasks-20260629.md` is referenced by the issue but is not present on current `develop`; public `docs/` and current code provide the available authority.
