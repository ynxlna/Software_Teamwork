# QA -> AI Gateway Smoke Design Research

## Existing Coverage

- `services/qa/internal/platform/modelclient/openai_test.go` already proves QA sends `X-Service-Token`, `X-Caller-Service: qa`, request/user IDs, and `profile_id`, and that it drops raw downstream error bodies.
- `services/ai-gateway/internal/http/provider_smoke_test.go` already exercises controlled `httptest.Server` providers, provider authentication, request-ID forwarding, response validation, and normalized provider failures.
- `docs/services/ai-gateway/docs/seed-runbook.md` documents seeded profiles and explicitly separates controlled fake-provider tests from opt-in real-provider smoke.
- Issue #287 is open with no assignee, branch, or PR, so there is no reusable external provider fixture to consume yet.

## Options Considered

### A. Env-gated QA integration test against a running AI Gateway (chosen)

- Uses the same `modelclient.Client` and QA config as runtime.
- Normal test runs skip unless explicitly enabled.
- A controlled or real provider can be selected through the AI Gateway profile.
- Negative token/profile probes are handled by AI Gateway before invoking the provider.

### B. Import AI Gateway internals into QA tests

- Rejected because both services are separate Go modules and Go `internal` boundaries intentionally prevent this coupling.

### C. Add a QA-owned fake provider/server orchestration stack

- Rejected for this issue because it duplicates #287 and expands QA ownership into provider-adapter test infrastructure.

## Environment Contract

| Variable | Purpose |
| --- | --- |
| `QA_AI_GATEWAY_SMOKE=1` | Explicit opt-in gate. |
| `AI_GATEWAY_URL` | AI Gateway `/internal/v1/chat/completions` endpoint. |
| `AI_GATEWAY_TOKEN` | Preferred QA service token; falls back to `INTERNAL_SERVICE_TOKEN`. |
| `AI_GATEWAY_TOKEN_HEADER` | Defaults to `X-Service-Token`. |
| `AI_GATEWAY_PROFILE_ID` | Required explicit chat profile for the smoke. |
| `MODEL_ID` | Must exactly match the selected profile model. |
| `AI_GATEWAY_TIMEOUT` | Positive duration for the model request. |

## Assertion Boundary

- Positive call: assistant role, finish reason, and content or tool calls.
- Invalid token: classified `dependency_error`; no raw body or token in output.
- Missing profile: classified `dependency_error`; no provider call should be needed.
- Logs: emit a generated request ID for correlation, profile/model identifiers, and configuration hints, but never token values or provider raw bodies.

