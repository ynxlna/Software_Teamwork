# B-02 QA 会话与消息资源 API

## Source

- GitHub issue: https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/88
- Team: JerryTeam
- Module: qa
- Branch: `JerryTeam/feat/qa-session-message-api`
- Base branch: `develop`

## Authority

Follow `docs/` when contracts or code disagree:

- `docs/services/qa/README.md`
- `docs/services/qa/docs/data-models.md`
- `docs/services/gateway/api/openapi.yaml`
- `docs/architecture/service-boundaries.md`
- `docs/architecture/technology-decisions.md`

## Goal

Implement the QA-owned session and message resource API so the frontend can
create/list/read/update/delete QA sessions and list/create messages for a
current user.

## In Scope

Implement and verify these gateway-facing QA contracts through the QA service
internal framework:

1. `POST /api/v1/qa-sessions`
2. `GET /api/v1/qa-sessions`
3. `GET /api/v1/qa-sessions/{sessionId}`
4. `PATCH /api/v1/qa-sessions/{sessionId}`
5. `DELETE /api/v1/qa-sessions/{sessionId}`
6. `GET /api/v1/qa-sessions/{sessionId}/messages`
7. `POST /api/v1/qa-sessions/{sessionId}/messages`

The QA service exposes internal service paths under `/internal/v1/**`; public
`/api/v1/**` routing is owned by gateway.

## Functional Requirements

- Use `X-User-Id` as the authoritative current-user context.
- List sessions only for the current user.
- Create sessions with status `active` and a stable `QASession` DTO.
- Return session details only to the owning user.
- Update session title and `active` / `archived` status.
- Soft-delete sessions so they no longer appear in session lists.
- Distinguish cross-user access from missing/deleted resources:
  - other user's resource: `403 forbidden`
  - missing or soft-deleted resource: `404 not_found`
- List messages for a session using stable `sequenceNo` ordering.
- Support message pagination and stable query parameters:
  - `page`
  - `pageSize`
  - `includeThinking`
  - `includeCitations`
- Create user messages and persist:
  - message row
  - `sequenceNo`
  - status
  - content block
  - basic response run / assistant placeholder structure when required by the existing framework.
- Keep response envelopes aligned with project contracts:
  - success: `{ data, requestId }`
  - pagination: `{ data, page, requestId }`
  - error: `{ error: { code, message, requestId, fields? } }`

## Non Goals

- Do not implement the full Agent Run, LLM orchestration, MCP tool loop, final answer generation, or complete SSE behavior beyond what the current framework already supports.
- Do not bypass gateway/auth ownership; QA only trusts gateway-injected context headers.
- Do not expose raw SQL errors, prompts, chain-of-thought, MCP raw arguments/results, object keys, internal URLs, provider errors, or secrets.

## Acceptance Criteria

- OpenAPI-facing schema and QA service responses stay aligned with docs.
- Error envelope and request id are consistent.
- Soft-deleted sessions are not returned by list APIs.
- Tests cover access to another user's session returning `403`.
- Tests cover message pagination and `includeThinking` / `includeCitations` stability.
- `go test ./...` and `go build ./cmd/server` pass from `services/qa`.

## Validation Plan

- Run `gofmt` for touched Go files.
- Run `go test ./...` in `services/qa`.
- Run `go build -buildvcs=false ./cmd/server` in `services/qa`.
- Run `git diff --check`.
