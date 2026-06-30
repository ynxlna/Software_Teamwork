# A-20260629-03 File MinIO Storage

## Goal

Implement the File Service MinIO object-store adapter so `services/file` can run with a persistent S3-compatible backend while keeping all bucket names, object keys, local paths, internal URLs, and credentials inside the File Service boundary.

## Requirements

- Add a MinIO-backed `service.ObjectStore` implementation under `services/file/internal/platform/storage`.
- Pin the official MinIO Go SDK version in `services/file/go.mod`.
- Wire `FILE_STORAGE_BACKEND=minio` through `internal/config` and `cmd/server`.
- Keep existing `memory` and `local` adapters; `local` remains the durable local smoke backend.
- Do not expose storage internals through File API JSON responses, content responses, logs, or config errors.
- Add tests for checksum handling, size mismatch, MinIO error mapping, and timeout/cancellation behavior.
- Update File Service runtime docs and technology baseline where the MinIO adapter status changes.

## Acceptance Criteria

- [x] Public/internal responses do not contain bucket, object key, internal URL, local filesystem path, access key, or secret key.
- [x] `FILE_STORAGE_BACKEND=minio` validates required MinIO configuration at startup.
- [x] MinIO adapter tests cover successful put/get/delete, checksum metadata, size mismatch, not-found mapping, dependency error mapping, and timeout/cancellation.
- [x] Existing local smoke backend remains available with `FILE_STORAGE_BACKEND=local`.
- [x] `cd services/file && go test ./...` passes.
- [x] `cd services/file && go build ./cmd/server` passes.
- [x] `git diff --check` passes.

## Definition of Done

- Tests are written before implementation and verified red for missing behavior.
- Service-local Go tests and build pass.
- Docs reflect the new runtime support without claiming PostgreSQL metadata or async cleanup have landed.
- Trellis task is archived and session journal records the work after user-approved commit flow.

## Technical Approach

Use a service-owned MinIO adapter that depends on the official SDK only inside `internal/platform/storage`. The adapter will keep the existing `ObjectStore` port:

```go
Put(ctx, key, body, contentType, sizeBytes) error
Get(ctx, key) (service.StoredObject, error)
Delete(ctx, key) error
```

Runtime wiring remains in `cmd/server`; handlers and service use cases continue to depend only on `service.ObjectStore`. Startup config introduces MinIO-specific env vars:

- `FILE_MINIO_ENDPOINT`
- `FILE_MINIO_ACCESS_KEY`
- `FILE_MINIO_SECRET_KEY`
- `FILE_MINIO_BUCKET`
- `FILE_MINIO_USE_SSL`
- `FILE_MINIO_REGION`
- `FILE_MINIO_TIMEOUT`

## Decision (ADR-lite)

**Context**: Issue #154 requires MinIO SDK/version alignment, local backend preservation, and no storage detail leakage. The current runtime supports only `memory` and `local`.

**Decision**: Implement only the MinIO adapter and runtime config in this slice. Do not implement async object cleanup/asynq in this PR because existing deletion remains synchronous and the issue only requires asynq version pinning if async cleanup is introduced.

**Consequences**: File can target MinIO for object bytes, but metadata runtime still uses the existing memory repository until the separate PostgreSQL runtime task lands. Object cleanup retries remain a documented follow-up.

## Out of Scope

- PostgreSQL metadata runtime wiring.
- New migrations or sqlc generated code.
- asynq cleanup worker and Redis configuration.
- Docker Compose MinIO server/mc provisioning.
- Gateway or owner-service API changes.
- Removing existing knowledge-document compatibility routes.

## Technical Notes

- Issue: https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/154
- S-04 reference: https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/121
- File docs: `docs/services/file/README.md`, `docs/services/file/docs/implementation.md`, `docs/services/file/docs/data-models.md`
- Architecture docs: `docs/architecture/service-boundaries.md`, `docs/architecture/technology-decisions.md`
- Backend specs: `.trellis/spec/backend/index.md`, `.trellis/spec/backend/database-guidelines.md`, `.trellis/spec/backend/error-handling.md`, `.trellis/spec/backend/logging-guidelines.md`, `.trellis/spec/backend/quality-guidelines.md`
- MinIO SDK research: `.trellis/tasks/06-30-file-minio-storage/research/minio-sdk.md`
