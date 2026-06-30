# QA To AI Gateway Model Call Smoke

## Goal

Complete issue #288 with an explicit, environment-gated smoke test that sends a minimal QA model request through the real QA model client to a running AI Gateway, proves the selected chat profile can return a normalized completion, and provides actionable token/profile failure diagnostics without making ordinary CI depend on a model provider.

## Requirements

- Add a single documented `go test` entry for the QA -> AI Gateway chat-completion smoke.
- Skip the smoke unless `QA_AI_GATEWAY_SMOKE=1` is explicitly set.
- Reuse QA runtime environment names and fallback behavior: `AI_GATEWAY_URL`, `AI_GATEWAY_TOKEN` or `INTERNAL_SERVICE_TOKEN`, `AI_GATEWAY_TOKEN_HEADER`, `AI_GATEWAY_PROFILE_ID`, `MODEL_ID`, and `AI_GATEWAY_TIMEOUT`.
- Require an explicit profile ID and non-empty service token once the smoke is enabled; fail with actionable, non-secret configuration guidance when either is missing.
- Send a minimal user message with a unique request ID and user ID through `internal/platform/modelclient`.
- Assert a successful response has an assistant message, a finish reason, and either content or tool calls.
- Probe an invalid service token and a missing profile ID, asserting both are normalized to QA dependency errors without exposing downstream bodies or credentials.
- Document controlled/fake-provider and real-provider prerequisites, the exact PowerShell and Bash commands, expected output, environment variables, and common failure diagnosis.
- Update QA implementation status so the env-gated smoke is recorded as implemented without claiming a full QA/Knowledge/Gateway end-to-end flow.

## Acceptance Criteria

- [x] `QA_AI_GATEWAY_SMOKE` unset makes the smoke test report `SKIP` and keeps normal `go test ./...` stable.
- [x] The documented opt-in command calls AI Gateway with QA caller headers and the configured profile/model.
- [x] A valid controlled or real provider response produces a normalized non-empty assistant completion.
- [x] Invalid token and missing profile probes return classified QA dependency errors.
- [x] Test failures and documentation never print service tokens, provider API keys, prompts beyond the fixed smoke prompt, or raw downstream error bodies.
- [x] QA tests and server/agent builds pass; repository diff has no whitespace errors.

## Definition Of Done

- Regression and opt-in smoke coverage are committed.
- QA README and implementation status describe how to run and troubleshoot the smoke.
- Trellis quality verification passes.
- The work commit is followed by Trellis task archival and session journal commits, then the branch is pushed to the personal fork.

## Technical Approach

Place the smoke in the existing `modelclient` package so it exercises the same client used by the QA server and agent. Load the normal QA config to preserve token fallback and transport defaults. Keep the positive provider call first; run negative token/profile probes only after a valid response proves the environment is correctly configured. Use the existing service error classifier for stable assertions and a unique request ID for AI Gateway/provider log correlation.

## Decision (ADR-lite)

**Context:** AI Gateway already owns provider/profile adapters and has in-process fake-provider coverage, while issue #287 has not yet produced a reusable external fixture. Duplicating the adapter or importing another Go module's `internal` packages would violate service boundaries.

**Decision:** Implement an env-gated integration test against a separately running AI Gateway. Operators may point its selected profile at a controlled OpenAI-compatible provider or an explicitly configured real provider.

**Consequences:** Normal CI remains deterministic. The smoke proves the actual QA HTTP client, service token, caller headers, profile selection, and response normalization, but it requires an operator-provided AI Gateway/profile/provider runtime when explicitly enabled.

## Out Of Scope

- New AI Gateway provider adapters or profile-management APIs.
- A bundled fake provider runtime owned by QA.
- Full QA session/PostgreSQL, Gateway/Auth, Knowledge retrieval, File, Parser, MCP, or frontend flows.
- Streaming and tool-execution smoke coverage.
- Making real-provider calls mandatory in CI.

## Technical Notes

- Issue: <https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/288>
- Completed dependency: #234 seed/profile runbook and AI Gateway fake-provider tests.
- Pending dependency: #287 has no branch or PR artifact yet; this task uses its allowed controlled-provider boundary without implementing its adapter regression suite.
- Authoritative docs: `docs/services/qa/docs/implementation.md`, `docs/services/ai-gateway/docs/implementation.md`, and `docs/services/ai-gateway/docs/seed-runbook.md`.
- Research record: `research/smoke-design.md`.
