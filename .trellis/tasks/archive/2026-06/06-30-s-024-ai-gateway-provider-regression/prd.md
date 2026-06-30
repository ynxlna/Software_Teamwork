# S-024 AI Gateway provider adapter regression

## Goal

Complete GitHub issue #287 by adding AI Gateway provider adapter regression samples that downstream QA and Knowledge work can reuse before connecting to real model providers. The work must keep ordinary CI deterministic while documenting the explicit path for real provider smoke runs.

## Requirements

* Add controlled fake-provider regression coverage for the model invocation paths used by downstream services:
  * chat completion smoke remains covered by existing provider smoke tests;
  * embeddings must cover successful OpenAI-compatible `object=list` response flow through HTTP handler, provider adapter, service validation, and invocation recording;
  * rerank must cover successful provider `results[]` / `relevance_score` shape through HTTP handler, provider adapter normalization, service validation, top-n limiting, and invocation recording.
* Keep regular CI independent from external provider services.
* Add an explicit real-provider smoke entry point that skips unless the required opt-in flag and environment variables are present.
* Document fake/controlled provider seed profiles, commands, expected response shapes, and common diagnostics for auth failure, missing profile, missing env, and provider response shape mismatch.
* Keep public AI Gateway API behavior unchanged.
* Archive the Trellis task after code, docs, verification, and commit are complete.

## Acceptance Criteria

* [x] A reusable command runs fake/controlled provider regression samples for chat, embeddings, and rerank.
* [x] Real provider smoke only runs when explicitly enabled and all required environment variables are available.
* [x] Docs describe seed profiles, run commands, expected response shapes, and common failure causes.
* [x] Downstream QA/Knowledge tasks can reference this issue's docs and tests for provider adapter expectations.
* [x] `services/ai-gateway` local checks pass.

## Definition of Done

* Tests added or updated next to the AI Gateway service code.
* Documentation under `docs/services/ai-gateway/docs/` is updated with current implementation facts.
* `go test ./...` and `go build ./cmd/server` pass from `services/ai-gateway`.
* `git diff --check` passes.
* Work is committed, pushed to the fork branch, PR targets upstream `develop`, and this task is archived.

## Verification

* [x] `docker run --rm -u 1000:1000 -e GOCACHE=/tmp/go-cache -e GOPROXY=https://goproxy.cn,direct -v /home/eir/Software_Teamwork:/repo -w /repo/services/ai-gateway golang:1.25 sh -c 'gofmt -w internal/http/provider_smoke_test.go && go test ./internal/http -run "Test(EmbeddingSmoke_ControlledProviderOpenAIShapeRecordsSummary|RerankSmoke_ControlledProviderResultsShapeRecordsSummary|RealProviderSmoke_ExplicitEnvOnly)" -count=1 -v'`
* [x] `docker run --rm -u 1000:1000 -e GOCACHE=/tmp/go-cache -e GOPROXY=https://goproxy.cn,direct -v /home/eir/Software_Teamwork:/repo -w /repo/services/ai-gateway golang:1.25 sh -c 'go test ./... && go build ./cmd/server'`
* [x] `docker run --rm -u 1000:1000 -e GOCACHE=/tmp/go-cache -e GOPROXY=https://goproxy.cn,direct -v /home/eir/Software_Teamwork:/repo -w /repo/services/ai-gateway golang:1.25 sh -c 'go test ./internal/http -run "Test(ChatSmoke|ChatStreamSmoke|EmbeddingSmoke|RerankSmoke)" -count=1'`
* [x] `git diff --check`
* [x] Rebased onto `upstream/develop` `635177798b079c77edcef7207b62fc25b780ba86` and reran `go test ./... && go build ./cmd/server`, the controlled provider regression command, and `git diff --check`.

## Technical Approach

Use the existing AI Gateway `httptest.Server` smoke style in `services/ai-gateway/internal/http/provider_smoke_test.go` and the existing memory repository helper. Add success-path HTTP smoke tests that assert provider request path/header/body shape, response normalization, no secret/content leakage into invocation records, and low-sensitive invocation summaries.

Add an environment-gated real provider smoke test in the same HTTP package. It should use the same server/profile registration path but immediately skip unless `AI_GATEWAY_REAL_PROVIDER_SMOKE=1` and the relevant provider environment variables are set. Missing optional operation-specific model variables should skip only that operation.

Update AI Gateway provider adapter docs, implementation status, and seed runbook so QA/Knowledge can reuse the controlled sample command and know when a real provider smoke is intentionally skipped.

## Decision (ADR-lite)

**Context**: Issue #287 asks for real/controlled provider regression samples without making normal CI depend on external model services.

**Decision**: Implement deterministic controlled provider tests with `httptest.Server` as the CI path, and add an explicit env-gated real provider smoke that defaults to skipped.

**Consequences**: CI remains stable and validates adapter request/response contracts. Real provider compatibility is available for maintainers with credentials, but live provider success still depends on external account/model availability and should be recorded separately when run.

## Out of Scope

* No new provider capability matrix.
* No public AI Gateway API changes.
* No QA or Knowledge business smoke implementation.
* No real provider credentials, cassettes, or recorded provider payloads committed to the repository.

## Research References

* [`research/provider-regression.md`](research/provider-regression.md) - code and docs baseline for the provider adapter regression scope.

## Technical Notes

* Issue: <https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/287>
* Branch: `Special/test/ai-gateway-provider-regression`
* Base branch: `develop`
* Scope: `ai-gateway`
* Current upstream baseline after final rebase: `635177798b079c77edcef7207b62fc25b780ba86`
* Relevant docs:
  * `docs/services/ai-gateway/docs/provider-adapters.md`
  * `docs/services/ai-gateway/docs/implementation.md`
  * `docs/services/ai-gateway/docs/seed-runbook.md`
* Relevant specs:
  * `.trellis/spec/backend/index.md`
  * `.trellis/spec/backend/api-contracts.md`
  * `.trellis/spec/backend/error-handling.md`
  * `.trellis/spec/backend/quality-guidelines.md`
  * `.trellis/spec/backend/logging-guidelines.md`
  * `.trellis/spec/guides/index.md`
