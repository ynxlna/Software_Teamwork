# API Contracts

> Contract-first rules for gateway-facing and cross-service HTTP APIs.

---

## Documentation Authority

`docs/` is the source of truth for project contracts. When this Trellis spec and
`docs/` disagree, inspect and follow these files first, then update this spec:

- `docs/architecture/service-boundaries.md`
- `docs/architecture/frontend-backend-contract.md`
- `docs/architecture/technology-decisions.md`
- `docs/services/gateway/api/openapi.yaml`
- `docs/services/<service>/README.md`
- `docs/services/<service>/api/openapi.yaml` or
  `docs/services/<service>/api/*.openapi.yaml`

Do not implement or generate frontend/backend clients from Trellis examples that
contradict the current `docs/` contracts.

## Scenario: Gateway Contract-First API

### 1. Scope / Trigger

- Trigger: any new or changed frontend-facing gateway endpoint, gateway
  response envelope, frontend API client DTO, or cross-service route ownership.
- Applies to `services/gateway/`, browser API clients under `apps/web/`,
  and the domain service that owns the endpoint's business state.

### 2. Signatures

Gateway public endpoints are documented in:

```text
docs/services/gateway/api/openapi.yaml
```

Public routes use these prefixes:

```text
GET /healthz
GET /readyz
/api/v1/**
```

Stable public gateway routes and service-to-service HTTP routes must be
RESTful resource-oriented APIs:

- model paths as resources or collections,
- use HTTP methods for actions,
- use `GET` for reads, `POST` for creation, `PATCH` for partial updates, and
  `DELETE` for deletion,
- do not put action verbs such as `login`, `logout`, `register`, `download`,
  `search`, `generate`, `export`, `retry`, or `revoke` in stable paths,
- model long-running work as resources such as `jobs`, `files`, `sessions`,
  `messages`, `events`, or `queries`.

`/healthz` and `/readyz` are allowed operational exceptions.

Every OpenAPI operation must include:

- `operationId`
- `tags`
- `summary`
- at least one success response
- at least one `4XX` response for user-callable operations
- `x-owner-service` for routes backed by a service boundary

### 3. Contracts

Gateway success envelope:

```json
{
  "data": {},
  "requestId": "req_123"
}
```

Gateway paginated envelope:

```json
{
  "data": [],
  "page": {
    "page": 1,
    "pageSize": 20,
    "total": 100
  },
  "requestId": "req_123"
}
```

Gateway error envelope:

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123",
    "fields": {
      "name": "is required"
    }
  }
}
```

Public IDs are strings. Public timestamps use OpenAPI `date-time`.

Gateway must pass request context to downstream services with these headers
when values are available:

| Header | Purpose |
| --- | --- |
| `X-Request-Id` | Correlate frontend request, gateway logs, and downstream logs. |
| `X-User-Id` | Authenticated user identity. |
| `X-User-Roles` | Comma-separated authenticated roles. |
| `X-User-Permissions` | Comma-separated authenticated permissions. |
| `X-Forwarded-For` | Original client address chain. |
| `X-Forwarded-Proto` | Original request protocol. |

### 4. Validation & Error Matrix

| Condition | Public response |
| --- | --- |
| Invalid request shape or field value | `400 validation_error` |
| Missing or invalid authentication | `401 unauthorized` |
| Authenticated caller lacks permission | `403 forbidden` |
| Resource does not exist or is hidden | `404 not_found` |
| State conflict | `409 conflict` |
| Rate or quota exceeded | `429 rate_limited` |
| Downstream service or infrastructure failed | `502 dependency_error` |
| Unexpected gateway failure | `500 internal_error` |

Do not forward raw downstream error bodies, SQL details, object keys, tokens,
prompts, vector payloads, or internal URLs to the frontend.

### 5. Good/Base/Bad Cases

- Good: add a gateway route to `docs/services/gateway/api/openapi.yaml`, mark
  `x-owner-service`, use the standard envelope, and update
  `docs/architecture/service-boundaries.md` if ownership is new.
- Base: proxy a domain-service route through gateway without changing the
  domain response shape, but still normalize errors to the gateway envelope.
- Bad: add a frontend call directly to `services/knowledge` or embed Qdrant,
  MinIO, SQL, prompt, or report-generation logic in gateway.

### 6. Tests Required

When implementation exists:

- Gateway handler tests assert status code, response envelope, and request id.
- Error tests cover validation, auth failure, forbidden, not found, and
  dependency failure where applicable.
- Cross-service client tests use mocked HTTP servers and assert propagated
  context headers.
- Frontend API client tests assert request path, response normalization, and
  error-code mapping.

For documentation-only contract changes:

- Run an OpenAPI linter against `docs/services/gateway/api/openapi.yaml`.
- Parse the YAML and verify `$ref` targets resolve.
- Check route prefix consistency: health routes stay unversioned, public API
  routes use `/api/v1/**`.
- Check stable and placeholder paths follow the RESTful resource-path rule.

### 7. Wrong vs Correct

#### Wrong

```text
frontend -> services/knowledge/search
gateway handler -> Qdrant query -> raw vector payload response
```

#### Correct

```text
frontend -> gateway /api/v1/knowledge-queries
gateway -> knowledge service
knowledge service -> retrieval infrastructure
gateway -> normalized KnowledgeQueryResponse or ErrorResponse
```

## Related Documents

- `docs/services/gateway/README.md`
- `docs/services/gateway/api/openapi.yaml`
- `docs/architecture/service-boundaries.md`
- `docs/architecture/frontend-backend-contract.md`

## Scenario: Internal Service Contract API

### 1. Scope / Trigger

- Trigger: adding or changing an internal service-to-service HTTP API, including
  model invocation APIs owned by `ai-gateway`.
- Applies to `docs/services/<service>/api/openapi.yaml` or
  `docs/services/<service>/api/*.openapi.yaml`,
  `services/<service>/api/openapi.yaml` when implementation exists, service
  interface docs, and matching service-boundary documentation.

### 2. Signatures

Internal service routes use:

```text
GET /healthz
GET /readyz
/internal/v1/**
```

AI Gateway model invocation routes intentionally use OpenAI-compatible paths
inside the internal prefix:

```text
POST /internal/v1/chat/completions
POST /internal/v1/embeddings
POST /internal/v1/rerankings
```

`/internal/v1/rerankings` is an OpenAI-style extension because OpenAI does not
define a native rerank endpoint.

### 3. Contracts

Internal project-owned configuration or metadata APIs use the standard project
envelope:

```json
{ "data": {}, "requestId": "req_123" }
```

```json
{ "error": { "code": "validation_error", "message": "request validation failed", "requestId": "req_123" } }
```

AI Gateway model profile APIs must treat provider credentials as write-only:
requests may include `apiKey` for create/update, but responses, logs, errors,
and frontend-visible gateway admin responses may only expose `apiKeyConfigured`
and non-secret provider/model metadata.

AI Gateway chat completion, embedding, and rerank APIs use OpenAI-compatible or
OpenAI-style request, success response, streaming chunk, function-calling, and
error body shapes.
They must not wrap successful model responses in the project `data/requestId`
envelope. The request id is carried through `X-Request-Id` response headers and
logs.

AI Gateway may pass through and normalize OpenAI-compatible function-calling
fields such as `tools`, `tool_choice`, `parallel_tool_calls`,
`assistant.tool_calls`, `role=tool`, `tool_call_id`, and streaming
`delta.tool_calls`. It must not execute MCP tools or decide domain tool
permissions; the calling domain service, such as `qa`, owns tool policy,
execution, persistence, and public event projection.

Internal services must accept or propagate these headers when available:

| Header | Purpose |
| --- | --- |
| `X-Request-Id` | Correlate public gateway, domain service, AI Gateway, and provider logs. |
| `X-Service-Token` | Authenticate service-to-service calls. |
| `X-Caller-Service` | Identify the calling service, such as `qa`, `knowledge`, or `document`. |
| `X-User-Id` | Audit the user that triggered the model call when applicable. |
| `X-User-Roles` | Audit or quota context. |
| `X-User-Permissions` | Audit or quota context. |

Internal responses and logs must not expose raw API keys, provider bearer
tokens, prompt secrets, raw provider error bodies, storage object keys, vector
payloads, SQL details, or internal URLs.

### 4. Validation & Error Matrix

| Condition | Internal response |
| --- | --- |
| Invalid request shape or field value | `400 validation_error` or OpenAI-style `invalid_request_error` |
| Missing or invalid service credential | `401 unauthorized` or OpenAI-style `authentication_error` |
| Caller service lacks permission | `403 forbidden` or OpenAI-style `permission_error` |
| Profile or resource does not exist | `404 not_found` |
| State or configuration conflict | `409 conflict` |
| Rate or quota exceeded | `429 rate_limited` or OpenAI-style `rate_limit_error` |
| Provider or infrastructure failed | `502 dependency_error` or OpenAI-style `upstream_error` |
| Unexpected service failure | `500 internal_error` |

### 5. Good/Base/Bad Cases

- Good: `qa` calls `ai-gateway` with `POST /internal/v1/chat/completions`,
  passes approved function-calling tool definitions, executes any returned MCP
  tool calls itself, keeps conversation/message/tool-call/citation state in
  `qa`, and stores only sanitized summaries plus the normalized assistant
  response.
- Base: `knowledge` calls `POST /internal/v1/embeddings` and writes the returned
  vectors to its own Qdrant collections without exposing vector payloads to
  gateway responses.
- Bad: public `gateway` directly calls an OpenAI-compatible provider, stores an
  API key, or exposes `/internal/v1/chat/completions` to frontend clients.

### 6. Tests Required

For documentation-only contract changes:

- Parse the affected OpenAPI YAML file.
- Verify all `$ref` targets resolve.
- Verify internal business paths use `/internal/v1/**`, except `/healthz` and
  `/readyz`.
- Verify AI Gateway model invocation operations document OpenAI-compatible
  success, streaming, function-calling, and error shapes.
- Check Markdown links resolve.

When implementation exists:

- Handler tests assert project envelope or OpenAI-compatible response shape as
  appropriate for the endpoint.
- Cross-service client tests assert `X-Request-Id`, `X-Service-Token`, and
  `X-Caller-Service` propagation.
- Sensitive-data tests assert API keys, provider tokens, prompts, raw provider
  errors, and vector payloads are not logged or returned.

### 7. Wrong vs Correct

#### Wrong

```text
frontend -> gateway -> /internal/v1/chat/completions
gateway stores provider API key and streams raw provider chunks
```

#### Correct

```text
frontend -> gateway /api/v1/qa-sessions/{sessionId}/messages
gateway -> qa service
qa service -> ai-gateway /internal/v1/chat/completions
qa owns messages, MCP tool calls, citations, and public SSE event shape
```

## Scenario: Missing Downstream API Contracts

### 1. Scope / Trigger

- Trigger: a downstream service such as `knowledge`, `qa`, `document`, or an
  aggregation surface has not finalized its frontend/backend contract yet.
- Applies to `docs/services/gateway/api/openapi.yaml`,
  `docs/services/gateway/README.md`,
  `docs/architecture/service-boundaries.md`, and
  `docs/architecture/frontend-backend-contract.md`.

### 2. Signatures

Unfinalized endpoints must not be added as active `paths` operations in:

```text
docs/services/gateway/api/openapi.yaml
```

Instead, list them under the OpenAPI root extension:

```yaml
x-missing-contracts:
  - service: gateway
    status: missing
    reason: Management overview aggregation fields are not finalized yet.
    placeholderOperations:
      - GET /api/v1/admin-overview
      - GET /api/v1/admin-metrics
```

### 3. Contracts

Active OpenAPI `paths` represent stable frontend-facing contracts. Missing
placeholder operations are TODO markers only:

- frontend clients must not generate callable methods from placeholders,
- backend services must not treat placeholders as implementation commitments,
- docs may describe expected ownership, but not stable request/response fields.

### 4. Validation & Error Matrix

| Condition | Required handling |
| --- | --- |
| Endpoint request/response shape is finalized | Add an active OpenAPI operation with owner, schemas, and error responses. |
| Endpoint owner is known but shape is not finalized | Keep it in `x-missing-contracts` only. |
| Placeholder overlaps with an active operation | Use method-level placeholders, not broad path globs that hide stable operations. |
| Frontend needs a missing endpoint | First finalize and review the OpenAPI operation, then generate or implement clients. |

### 5. Good/Base/Bad Cases

- Good: keep management overview and cross-service metric aggregation under
  `x-missing-contracts` until request, response, owner, and aggregation source
  fields are finalized.
- Base: keep only management overview or metric aggregation placeholders in
  `x-missing-contracts` until their sources and display fields are finalized.
  Do not mark QA, document, knowledge, auth, or admin runtime configuration
  routes missing once they are active paths in the gateway OpenAPI.
- Bad: add placeholder management overview schemas to OpenAPI active `paths`
  just to reserve routes.

### 6. Tests Required

For documentation-only contract changes:

- Parse `docs/services/gateway/api/openapi.yaml`.
- Verify every active `/api/v1/**` operation has an allowed finalized owner.
- Verify only genuinely unfinalized downstream areas are present in
  `x-missing-contracts`.
- Check broad placeholders do not contradict active operations.
- Check Markdown links resolve.

### 7. Wrong vs Correct

#### Wrong

```text
OpenAPI paths include GET /api/v1/admin-overview with made-up fields even
though management overview aggregation is still listed as missing.
```

#### Correct

```text
x-missing-contracts lists only placeholders such as
GET /api/v1/admin-overview and GET /api/v1/admin-metrics until those contracts are finalized.
```

## Scenario: Domain Service Interface Documents

### 1. Scope / Trigger

- Trigger: adding or changing a service-level interface document such as
  `docs/services/auth/README.md` or `docs/services/file/README.md`.
- Applies when gateway-facing routes depend on an internal domain service
  contract, even if the service code has not been implemented yet.

### 2. Signatures

Service interface documents must list every related gateway route with:

- HTTP method
- gateway path
- authentication requirement
- owner service
- short behavior summary

If an internal service route is proposed, mark it as an internal draft and keep
it separate from the public gateway contract.

### 3. Contracts

Document request and response fields using the same public IDs, timestamps,
envelopes, and error shapes defined in
`docs/services/gateway/api/openapi.yaml`.
Binary success responses, such as file content, may omit the JSON envelope,
but error responses must still use the standard error shape.

### 4. Validation & Error Matrix

For each documented endpoint, separate:

- status codes already declared in OpenAPI,
- future status codes that require an OpenAPI update before frontend reliance.

### 5. Good/Base/Bad Cases

- Good: `docs/services/file/README.md` documents file-owned routes, notes knowledge-owned
  related routes, and calls out that object keys must not reach the frontend.
- Base: a service document summarizes the gateway OpenAPI without adding
  implementation-only behavior.
- Bad: a service document declares a new frontend-facing status code or field
  as stable without updating `docs/services/gateway/api/openapi.yaml`.

### 6. Tests Required

For documentation-only changes:

- Parse `docs/services/gateway/api/openapi.yaml`.
- Verify documented public paths exist in the OpenAPI file.
- Check Markdown links resolve.

When implementation exists, add handler or client tests for the documented
status codes, envelopes, request id propagation, and context headers.

### 7. Wrong vs Correct

#### Wrong

```text
docs/services/file/README.md declares GET /api/v1/files/{id}/download as stable
docs/services/gateway/api/openapi.yaml has no matching public path
```

#### Correct

```text
docs/services/file/README.md references /api/v1/documents/{documentId}/content
docs/services/gateway/api/openapi.yaml owns the same public path and owner-service marker
```

## Scenario: Internal Domain Service APIs

### 1. Scope / Trigger

- Trigger: implementing a domain service HTTP API that gateway or another backend
  service will call directly, even when the public gateway contract is unchanged.
- Applies to `services/<service>/api/openapi.yaml`, `services/<service>/internal/http/`,
  service README files, and matching domain docs such as
  `docs/services/file/README.md`.

### 2. Signatures

Internal domain-service routes must use service-local versioned resource paths:

```text
GET /healthz
GET /readyz
/internal/v1/**
```

Business routes under `/internal/v1/**` must remain RESTful and resource-oriented.
They may be close to public gateway paths, but they are not public frontend
contracts unless the same operation is active in
`docs/services/gateway/api/openapi.yaml`.

### 3. Contracts

Every implemented domain service should document internal API signatures in:

```text
services/<service>/api/openapi.yaml
```

Internal JSON responses use the same envelope and error shapes as gateway:

```json
{ "data": {}, "requestId": "req_123" }
```

```json
{ "error": { "code": "validation_error", "message": "request validation failed", "requestId": "req_123" } }
```

Internal metadata responses may include service-owned integration fields that
are not yet public frontend fields, for example `contentType` or `sizeBytes`
for file-owned metadata. They must not expose storage object keys, bucket names,
internal URLs, SQL details, tokens, credentials, vector payloads, or prompts.

Domain services must accept gateway context headers when present:

| Header | Purpose |
| --- | --- |
| `X-Request-Id` | Correlate gateway, service logs, and downstream calls. |
| `X-User-Id` | Authenticated user identity injected by gateway. |
| `X-User-Roles` | Comma-separated roles injected by gateway. |
| `X-User-Permissions` | Comma-separated permissions injected by gateway. |
| `X-Forwarded-For` | Original client address chain. |
| `X-Forwarded-Proto` | Original request protocol. |

### 4. Validation & Error Matrix

| Condition | Internal response |
| --- | --- |
| Invalid request shape or field value | `400 validation_error` |
| Missing required gateway user context | `401 unauthorized` |
| Authenticated caller lacks permission | `403 forbidden` |
| Resource does not exist, is deleted, or should be hidden | `404 not_found` |
| State conflict | `409 conflict` |
| Infrastructure dependency failed | `502 dependency_error` |
| Unexpected service failure | `500 internal_error` |

### 5. Good/Base/Bad Cases

- Good: file service adds `GET /internal/v1/documents/{documentId}` for
  file-owned metadata and documents it in `services/file/api/openapi.yaml`,
  while public `GET /api/v1/documents/{documentId}` remains knowledge-owned and
  exposes only knowledge document fields.
- Base: gateway proxies an active public route to a matching internal route and
  normalizes any service-owned extra fields before returning to frontend.
- Bad: a domain service adds a public-looking `/api/v1/**` route or exposes raw
  object keys, bucket names, MinIO URLs, SQL errors, prompts, or vector payloads
  in an internal response body.

### 6. Tests Required

When implementation exists:

- Handler tests assert envelope shape, request id propagation, and expected
  status codes for validation, auth context failure, not found, and dependency
  failures where applicable.
- DTO or handler tests assert service-owned integration fields are returned only
  by internal contracts when they are not public gateway fields.
- Content or streaming endpoints assert binary success responses and JSON error
  responses separately.
- Cross-service client tests assert gateway context headers are propagated.

### 7. Wrong vs Correct

#### Wrong

```text
services/file exposes GET /api/v1/documents/{documentId}
response includes objectKey: documents/doc_123
```

#### Correct

```text
services/file exposes GET /internal/v1/documents/{documentId}
response includes contentType and sizeBytes, but no objectKey
public GET /api/v1/documents/{documentId} stays knowledge-owned and does not expose objectKey
```

## Scenario: Document Report Template And Material APIs

### 1. Scope / Trigger

- Trigger: adding or changing Document Service report-type, report-template,
  template-structure, or report-material APIs.
- Applies to `services/document/internal/http`, `services/document/internal/service`,
  `services/document/internal/repository`, `services/document/internal/platform/fileclient`,
  and the matching gateway contract in `docs/services/gateway/api/openapi.yaml`.

### 2. Signatures

Service-local Document routes should mirror the gateway resource paths unless the
team introduces a versioned internal Document API:

- `GET /report-types`
- `GET /report-templates`
- `POST /report-templates` with multipart field `file`, `templateName`,
  `reportType`, and optional `description`
- `GET /report-templates/{reportTemplateId}`
- `PATCH /report-templates/{reportTemplateId}` with optional `templateName`,
  `description`, and `enabled`
- `DELETE /report-templates/{reportTemplateId}`
- `GET /report-templates/{reportTemplateId}/structure`
- `PATCH /report-templates/{reportTemplateId}/structure`
- `GET /report-materials`
- `POST /report-materials` with multipart field `file`, `materialName`,
  `materialType`, optional `category`, `description`, and `tags`
- `GET /report-materials/{materialId}`
- `DELETE /report-materials/{materialId}`

Document calls File Service through:

- `POST /internal/v1/files`
- `DELETE /internal/v1/files/{fileId}` for best-effort cleanup when a Document
  business insert fails after upload

### 3. Contracts

- Gateway-facing responses use `{ data, requestId }`; list responses use
  `{ data, page, requestId }`.
- Public template DTOs may include `id`, `templateName`, `reportType`, `version`,
  `description`, `enabled`, `filename`, `fileSize`, `createdBy`, `createdAt`,
  and `updatedAt`.
- Public material DTOs may include `id`, `materialName`, `materialType`,
  `category`, `description`, `tags`, `enabled`, `filename`, `fileSize`,
  `createdBy`, `createdAt`, and `updatedAt`.
- Template structure follows gateway OpenAPI: `outlineSchema` array and
  `styleConfig` object. Do not expose `materialMappings` unless the gateway
  OpenAPI contract is updated first.
- Document may persist `file_ref` internally, but public responses must not
  expose File Service IDs, `file_ref`, buckets, object keys, internal URLs,
  signed URLs, or storage credentials.
- Template/material deletion should soft-delete business rows with `deleted_at`
  and hide them from list/detail responses.

### 4. Validation & Error Matrix

| Condition | Response/error |
| --- | --- |
| Missing gateway user context | `401 unauthorized` |
| Invalid page, pageSize, enabled, UUID, JSON shape, or multipart body | `400 validation_error` |
| Missing `templateName`, `reportType`, `materialName`, `materialType`, or upload file | `400 validation_error` |
| Template upload is not a DOCX in the first implementation slice | `400 validation_error` |
| Disabled or missing report type on template create | `400 validation_error` |
| Missing or soft-deleted template/material | `404 not_found` |
| File Service upload failure | `502 dependency_error` |
| PostgreSQL query/insert/update failure | `502 dependency_error` unless a typed domain error applies |

### 5. Good/Base/Bad Cases

- Good: handler parses multipart, service validates business fields and calls
  File Service, repository stores `file_ref` plus safe display metadata, and the
  response omits all internal file identifiers.
- Base: template/material rows are soft-deleted and hidden from read APIs while
  historical report references remain intact.
- Bad: returning File Service `id` as a public template/material field, exposing
  object storage details, or calling File Service while holding a database
  transaction.

### 6. Tests Required

- Handler tests for response envelopes, pagination metadata, request ID
  propagation, invalid query parameters, missing upload file, and JSON decode
  errors.
- Service tests or handler fakes for File Service dependency failure mapping,
  required field validation, DOCX validation, and disabled/missing report type.
- Repository integration tests, when `DOCUMENT_TEST_DATABASE_URL` is available,
  for list filters, soft delete hiding, structure JSON round-trip, and tags JSON
  round-trip.
- Response safety tests asserting public bodies do not contain `file_ref`,
  `fileRef`, raw File Service IDs, object keys, buckets, internal URLs, or signed
  URLs.

### 7. Wrong vs Correct

#### Wrong

```text
POST /report-templates -> document stores uploaded bytes itself -> response returns fileRef/fileId
```

#### Correct

```text
POST /report-templates -> document calls file /internal/v1/files -> stores file_ref internally -> response returns only template id and safe display metadata
```

## Scenario: Gateway Redis Session Cache

### 1. Scope / Trigger

- Trigger: adding or changing user creation, session creation, current session
  deletion, current-user behavior, auth middleware, or session identity fields.
- Applies to `services/gateway/`, `services/auth/`,
  `docs/services/auth/README.md`, `docs/services/gateway/README.md`,
  `docs/architecture/frontend-backend-contract.md`, and
  `docs/services/gateway/api/openapi.yaml`.

### 2. Signatures

Public auth routes stay under:

```text
POST /api/v1/users
POST /api/v1/sessions
DELETE /api/v1/sessions/current
GET  /api/v1/users/me
```

Auth success responses must include `data.user` and `data.session`.

### 3. Contracts

`data.user` must include:

- `id`
- `username`
- `roles`
- `permissions`

`data.session` must include:

- `sessionId`
- `accessToken`
- `tokenType`
- `expiresAt`

`accessToken` is an opaque random Bearer token, not a JWT. Gateway, frontend,
and downstream services must not parse claims from it.

Auth stores password credentials with `argon2id` and stores access-token hashes,
not raw access tokens. Gateway Redis cache keys use the token hash and must not
log raw tokens or hashes.

Gateway must store the runtime session in Redis using:

```text
gateway:session:<accessTokenHash>
```

The cached value must include enough fields to inject `X-User-Id`,
`X-User-Roles`, and `X-User-Permissions` without calling auth on every
business request. The Redis TTL must not outlive `data.session.expiresAt`.
Redis is not the durable source of user, role, permission, or session truth.

### 4. Validation & Error Matrix

| Condition | Public response |
| --- | --- |
| Missing bearer credential | `401 unauthorized` |
| Redis session miss, expired session, or malformed cache value | `401 unauthorized` |
| Auth rejects session credentials | `401 unauthorized` |
| Gateway cannot access Redis for an authenticated business request | `502 dependency_error` |
| Auth service or durable auth store is unavailable during user/session operations | `502 dependency_error` |

Do not expose raw tokens, token hashes, Redis keys, session secrets, or auth
internal URLs to frontend responses or logs.

### 5. Good/Base/Bad Cases

- Good: session creation response returns `user` plus `session`; gateway hashes the access
  token for the Redis key, sets TTL from `expiresAt`, and injects downstream
  identity headers from the cache.
- Base: `/api/v1/users/me` reads the Redis session cache and returns `UserResponse`
  without calling auth for every request.
- Bad: gateway stores original access tokens in logs or treats Redis as the
  durable source of permissions.

### 6. Tests Required

When implementation exists:

- Auth handler/client tests assert `SessionResponse` includes `user.permissions`
  and `session`.
- Gateway auth middleware tests cover Redis hit, miss, expired session,
  malformed session, and Redis dependency failure.
- Gateway downstream client tests assert `X-User-Id`, `X-User-Roles`,
  `X-User-Permissions`, and `X-Request-Id` are propagated.
- Current-session deletion tests assert auth invalidation is called and Redis cache is deleted.

For documentation-only changes:

- Parse `docs/services/gateway/api/openapi.yaml`.
- Verify `SessionResponse` requires `user` and `session`.
- Verify docs mention `gateway:session:<accessTokenHash>` and Redis TTL.

### 7. Wrong vs Correct

#### Wrong

```text
gateway receives Authorization: Bearer token
gateway calls auth service on every business request
gateway logs the raw token on failures
```

#### Correct

```text
gateway receives Authorization: Bearer token
gateway hashes token and reads gateway:session:<accessTokenHash>
gateway injects cached user, roles, and permissions into downstream headers
```
