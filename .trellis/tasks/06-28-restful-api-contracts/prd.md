# Make API Contracts RESTful

## Goal

Convert the current gateway-facing and service-boundary API contracts to
RESTful resource-oriented style, and record the restriction that all
frontend-facing and service-to-service HTTP communication must use RESTful API
design unless an explicit exception is documented.

## What I Already Know

* User requires all service communication to comply with RESTful API standards.
* Current active OpenAPI uses action-style auth paths such as
  `/api/v1/auth/login`, `/api/v1/auth/logout`, and `/api/v1/auth/register`.
* Current active OpenAPI uses action-style file download path
  `/api/v1/documents/{documentId}/download`.
* Downstream `knowledge`, `qa`, `document`, and `admin` contracts are currently
  missing placeholders and should also use RESTful placeholder operations.

## Assumptions

* RESTful here means resource nouns in paths, HTTP methods carrying action
  semantics, no verbs such as `login`, `logout`, `register`, `download`,
  `search`, `generate`, `export`, or `retry` in stable paths.
* Health probes `/healthz` and `/readyz` are infrastructure exceptions.
* Streaming and job-like behavior should still use resource-oriented endpoints,
  e.g. sessions/messages/events, jobs, files, or content subresources when
  contracts are later finalized.

## Requirements

* Replace action-style active auth paths with resource-style paths:
  * `POST /api/v1/users` for registration.
  * `POST /api/v1/sessions` for login/session creation.
  * `DELETE /api/v1/sessions/current` for logout/session deletion.
  * `GET /api/v1/users/me` for current user.
* Replace file download action path with `GET /api/v1/documents/{documentId}/content`.
* Update gateway, auth, file, frontend-backend, service-boundary, and README docs
  to use the new resource-style paths.
* Update missing downstream placeholders to avoid action-style names such as
  `/search` and `/reports/{id}/export`.
* Update API contract specs to require RESTful design for both public gateway
  and internal service-to-service HTTP APIs.

## Acceptance Criteria

* [x] Active OpenAPI paths contain no action words: `login`, `logout`,
  `register`, or `download`.
* [x] Active OpenAPI paths use resource nouns and HTTP methods for behavior.
* [x] Missing downstream placeholders use RESTful resource-style paths.
* [x] Docs and specs consistently state the RESTful API restriction.
* [x] OpenAPI parses, `$ref` targets resolve, Redocly lint passes, and local
  Markdown links resolve.

## Definition of Done

* Docs and OpenAPI updated.
* Relevant Trellis API contract spec updated.
* Checks pass.
* Changes committed, archived, journaled, pushed to the existing PR.

## Out of Scope

* Designing full knowledge/QA/document/admin contracts.
* Implementing backend services or frontend clients.
* Changing runtime behavior.

## Technical Notes

* Related files: `docs/api/gateway.openapi.yaml`, `docs/gateway.md`,
  `docs/auth.md`, `docs/file.md`, `docs/frontend-backend-contract.md`,
  `docs/service-boundaries.md`, `.trellis/spec/backend/api-contracts.md`.
