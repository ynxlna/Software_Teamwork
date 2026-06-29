# Database Guidelines

> Database, cache, vector-search, and object-storage conventions for Go backend services.

---

## Overview

Each backend service owns its persistence concerns. A service may use
PostgreSQL, Redis, Qdrant, or MinIO only through service-local repository or
platform packages. Handlers must not talk directly to infrastructure clients.

Confirmed Go infrastructure stack:

- PostgreSQL: `pgx` + `sqlc`.
- Migrations: `goose`.
- Redis cache/session access: `go-redis`.
- Redis queues: `asynq`.
- Qdrant: a short-term hand-written HTTP client until usage justifies an official or generated client.
- MinIO: official MinIO Go SDK.

Do not introduce an ORM by default. If a service needs one, document the reason
in that service README, update `docs/architecture/technology-decisions.md`,
and then update this spec.

---

## PostgreSQL Ownership

- Each service owns the tables it writes.
- Do not let one service write another service's tables.
- Cross-service data needs should go through HTTP APIs, events, or explicit read-model decisions.
- Table schemas must be represented by migrations under `services/<service>/migrations/`.
- Services that use PostgreSQL must keep service-local `sqlc.yaml`, query files,
  generated `sqlc` code, and `goose` migrations. Generated query structs must
  not leak into HTTP handlers.

Use PostgreSQL for:

- user identities, roles, permissions, sessions, and tokens metadata,
- file metadata and processing states,
- knowledge metadata and ingestion status,
- document generation jobs and outputs metadata,
- audit-friendly business state.

---

## Query Patterns

- Use parameterized queries only. Never concatenate user input into SQL.
- Keep SQL in repository methods or dedicated query files.
- Keep repository methods small and named by intent, not by SQL operation.
- Return domain-oriented structs from repositories; do not leak raw DB rows into handlers.
- Pass `context.Context` through every database call.
- Use pagination for list endpoints.
- Use explicit column lists instead of `SELECT *`.

Example repository shape:

```go
type UserRepository struct {
    db *pgxpool.Pool
}

func (r *UserRepository) FindByID(ctx context.Context, id UserID) (User, error) {
    const query = `
        SELECT id, email, display_name, created_at
        FROM users
        WHERE id = $1
    `
    // scan and wrap errors here
}
```

---

## Transactions

- Start transactions at the service/use-case layer when one business operation changes multiple records.
- Keep transaction bodies short and deterministic.
- Pass transaction handles into repositories through explicit interfaces.
- Roll back on every error and wrap rollback failures only when they add useful context.
- Do not perform slow external calls while holding a PostgreSQL transaction.

---

## Migrations

- Store `goose` migrations in `services/<service>/migrations/`.
- Use forward-only migrations for the first implementation slice unless rollback
  is explicitly supported and verified by the service.
- Name migrations with an ordered prefix and action summary:

```text
0001_create_users.sql
0002_add_file_processing_state.sql
```

- CI should validate migrations when migration tooling is introduced.
- Schema changes must be backward-compatible when multiple services or deployments may overlap.

---

## Naming Conventions

PostgreSQL naming:

- Tables: plural snake_case, for example `users`, `knowledge_items`.
- Columns: snake_case, for example `created_at`, `owner_user_id`.
- Primary keys: `id`.
- Foreign keys: `<entity>_id`.
- Indexes: `idx_<table>_<columns>`.
- Unique indexes: `uniq_<table>_<columns>`.

Use UTC timestamps and name them consistently:

- `created_at`
- `updated_at`
- `deleted_at` for soft delete only when the service actually supports it.

---

## Redis

Use Redis for short-lived data only:

- sessions or token deny-lists,
- cache entries,
- short-lived coordination,
- `asynq` queues.

Rules:

- Every cache key must have a stable prefix: `<service>:<resource>:<id>`.
- Every cache entry must have an explicit TTL unless it is intentionally persistent.
- Redis must not be the only source of durable business truth.
- Cache invalidation must be owned by the service that owns the underlying data.
- Queued task payloads must be JSON and include traceable fields such as
  `requestId`, `jobId`, and `userId` when available. PostgreSQL remains the
  authority for durable job state, final status, failure summary, and retry
  count.

---

## Qdrant

Use Qdrant for vector search only. The `knowledge` service owns collection
creation, vector metadata shape, and retrieval conventions.

Rules:

- Store durable knowledge metadata in PostgreSQL; store vectors and search payloads in Qdrant.
- Keep Qdrant payload fields minimal and retrieval-oriented.
- Version embedding models and collection names or metadata when the embedding shape changes.
- Do not let `qa` mutate Qdrant collections directly; it should retrieve through `knowledge` or a documented retrieval API.
- Do not let `ai-gateway` write Qdrant collections; model generation and vector
  persistence remain separate service responsibilities.

---

## MinIO

Use MinIO for object payloads:

- uploaded source files,
- extracted text artifacts if they are too large for PostgreSQL,
- generated documents,
- temporary processing outputs when needed.

Rules:

- Store object metadata and ownership in PostgreSQL.
- Use bucket names that map to domain purpose, not implementation detail.
- Generate object keys server-side.
- Never expose raw internal object keys as authorization decisions.
- Prefer pre-signed URLs only after checking ownership and permission in the service.

---

## Common Mistakes

- Treating Redis cache entries as durable workflow state.
- Storing full documents in PostgreSQL when MinIO is the correct storage layer.
- Duplicating knowledge metadata between PostgreSQL and Qdrant without a source-of-truth rule.
- Running external HTTP calls inside PostgreSQL transactions.
- Letting `qa` bypass `knowledge` and directly own retrieval logic.
