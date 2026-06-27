# Gateway Patterns and Contract-First Planning

## Topic

What should the gateway service own in this early microservice architecture, and what documents help frontend and backend teams work in parallel?

## Sources

* Microsoft Azure Architecture Center, "API gateways" - https://learn.microsoft.com/en-us/azure/architecture/microservices/design/gateway
* Microservices.io, "Pattern: API Gateway / Backends for Frontends" - https://microservices.io/patterns/apigateway.html
* OpenAPI Specification v3.2.0 - https://spec.openapis.org/oas/v3.2.0.html
* Microsoft Azure Architecture Center, "Backends for Frontends pattern" - https://learn.microsoft.com/en-us/azure/architecture/patterns/backends-for-frontends

## Findings

API gateway guidance consistently separates three responsibilities:

* Routing: expose one client-facing backend entry point and route requests to internal services.
* Aggregation: combine multiple internal service calls into one client-friendly endpoint when a workflow needs data from several services.
* Offloading: centralize cross-cutting behavior such as authentication checks, request correlation, logging, rate limiting, retries, and protocol/header transformations.

The Microservices.io API Gateway pattern emphasizes a single entry point for clients and warns that gateway logic can become a new source of complexity. Microsoft guidance similarly frames the gateway as useful for routing, aggregation, and cross-cutting concerns, but notes that gateway hops and fan-out add latency and reliability considerations.

The OpenAPI Specification is useful for early parallel development because it provides a language-agnostic, machine-readable HTTP API contract. Once gateway endpoints and downstream service endpoints are documented in OpenAPI, frontend work can proceed against mocks while backend teams implement service-local APIs independently.

The Backends-for-Frontends pattern is relevant only if the project later has multiple client types with substantially different API needs. For the current repository, there is only one React frontend, so one gateway API is sufficient. The project should avoid prematurely creating separate BFFs.

## Mapping To This Repo

Current repo constraints:

* Frontend is planned as `apps/frontend/`.
* Go microservices live under `services/<service>/` with independent `go.mod` files.
* Services communicate through HTTP/REST.
* The README already defines `gateway` as the backend unified entry point for routing, auth context passing, aggregation APIs, and cross-service coordination.
* Backend spec forbids cross-service `internal/` imports and warns against putting shared business logic in `services/gateway/`.
* Error handling spec requires stable JSON error responses across gateway and services.
* Logging spec requires request IDs/correlation fields and avoids leaking sensitive data.

## Recommendation

Use a thin-but-intentional gateway:

* In scope for gateway:
  * Public HTTP API surface consumed by frontend.
  * Request authentication enforcement via auth service or token verification policy.
  * Propagation of user context, roles/permissions, and `request_id` to downstream services.
  * HTTP routing and versioned API namespace.
  * Workflow aggregation endpoints that are explicitly client-facing and span services.
  * Stable public error envelope and downstream error translation.
  * CORS, request size limits, timeouts, retry/circuit-breaker policy, and access/request logs.
  * SSE pass-through/proxying for QA/report generation streams.
  * Health/readiness endpoints.

* Out of scope for gateway:
  * User identity storage or password/session ownership; belongs to auth.
  * File metadata, object storage, parsing, vector indexing; belongs to file/knowledge.
  * RAG, intent recognition, retrieval, LLM calls; belongs to qa/knowledge.
  * Report outline/content/export business workflows; belongs to document.
  * PostgreSQL migrations for other services.
  * Shared Go libraries or cross-service business logic.

## Early Documents That Unlock Parallel Work

Recommended first documentation package:

1. `docs/gateway.md`
   * Gateway purpose, responsibilities, non-responsibilities, routing map, auth/context propagation, error policy, streaming policy, timeout/retry rules.

2. `docs/api/gateway.openapi.yaml`
   * Frontend-facing API contract, including request/response DTOs, error envelope, auth requirements, pagination shape, upload constraints, and SSE endpoints.

3. `docs/api/internal-services.md`
   * Downstream service contract index: which service owns which endpoint, service base URL env var, request headers passed from gateway, and ownership notes.

4. `docs/service-boundaries.md`
   * Boundary matrix for gateway/auth/file/knowledge/qa/document to prevent business logic drifting into gateway.

5. `docs/frontend-backend-contract.md`
   * Frontend integration conventions: base path, auth flow, token/session behavior, error handling, loading/retry behavior, pagination/filter conventions.

6. `deploy/.env.example`
   * Names of gateway downstream service URLs and required non-secret runtime variables.

7. Optional later: `docs/adr/0001-gateway-boundary.md`
   * ADR-lite record once the team chooses the gateway style.

## Feasible Approaches

### Approach A: Contract-first thin gateway (recommended)

Define gateway boundaries and OpenAPI before implementing full service logic. Gateway initially exposes public routes, auth/context/error/observability conventions, health checks, and mockable contracts.

Pros:

* Best for parallel frontend/backend development.
* Reduces risk that gateway becomes a business monolith.
* Gives each service team a clear API target.

Cons:

* Requires discipline to keep OpenAPI and implementation synchronized.
* Some endpoint details may change as service teams discover edge cases.

### Approach B: Feature-slice gateway docs

Document gateway APIs by product feature first: login, knowledge management, file upload, QA, report generation. Each feature doc includes frontend API, downstream service owner, and data shapes.

Pros:

* Easier for feature teams to understand.
* Maps directly to UI pages and user workflows.

Cons:

* Boundary rules can become duplicated across feature docs.
* Cross-cutting behavior may drift unless there is also a shared gateway policy document.

### Approach C: Implementation-first gateway skeleton

Build `services/gateway/` skeleton first, then document actual endpoints as they are added.

Pros:

* Quickly creates runnable code and CI shape.
* Useful if the team wants to validate Go router/config choices immediately.

Cons:

* Less helpful for parallel development at project start.
* Raises risk of accidental API decisions before service ownership is clear.
