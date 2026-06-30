# Provider Adapter Regression Research

## Existing implementation

* AI Gateway owns `/internal/v1/chat/completions`, `/internal/v1/embeddings`, and `/internal/v1/rerankings`.
* Chat uses `provider.HTTPChatClient`; embeddings and rerank use `provider.HTTPClient`.
* Profiles own provider base URL, model, timeout, default parameters, and credentials. Callers only provide `model`, optional `profile_id`, and invocation payload.
* Service code records low-sensitive invocation summaries and rejects model/profile mismatch before reaching the provider.
* Provider adapter errors already discard raw provider response bodies and normalize status failures to project/OpenAI-style errors.

## Existing tests

* `provider_smoke_test.go` already covers chat provider 401/429/5xx/timeout, request ID forwarding, explicit profile routing, API key forwarding without response leakage, stream early close, and caller service recording.
* Embedding smoke tests cover provider 429 and 5xx normalization through the real HTTP adapter.
* Rerank smoke tests cover provider 429 normalization through the real HTTP adapter.
* Service-level tests cover embedding and rerank success, response validation, and invocation summaries through fake invokers.

## Gaps for issue #287

* No HTTP-level controlled provider success sample for embeddings through the real `provider.HTTPClient`.
* No HTTP-level controlled provider success sample for rerank `results[]` / `relevance_score` shape through the real `provider.HTTPClient`.
* Docs mention missing real provider smoke and rerank response-shape regression but do not provide a single reusable command/seed profile section for QA/Knowledge.
* There is no explicit env-gated live provider smoke entry point.

## Chosen approach

* Add deterministic `httptest.Server` smoke tests for embedding and rerank success paths.
* Add an env-gated live provider smoke test that skips by default and requires `AI_GATEWAY_REAL_PROVIDER_SMOKE=1` plus provider variables.
* Update provider adapter docs and seed runbook with commands, seed profiles, expected fake response shapes, and failure diagnostics.

## Spec update judgment

No `.trellis/spec/` update is needed for this task. The new executable contracts are service-local AI Gateway smoke commands and environment variables, so the appropriate source of truth is `docs/services/ai-gateway/docs/provider-adapters.md`, `implementation.md`, and `seed-runbook.md`. Backend global specs already require service-local `go test ./...`, no raw provider body leakage, and no credential/prompt/document text logging.
