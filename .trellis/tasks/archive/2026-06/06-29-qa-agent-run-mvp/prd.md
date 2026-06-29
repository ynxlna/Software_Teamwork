# QA ResponseRun and Non-Streaming Agent Loop MVP

## Background

Issue #89 (`B-03`) requires the QA service to turn the message creation path into a traceable non-streaming Agent Run MVP. The implementation must build on the latest `develop`, where #87, #88, and #120 are complete.

Authoritative references:

- `docs/services/qa/README.md`
- `docs/services/qa/docs/data-models.md`
- `docs/services/ai-gateway/README.md`
- `docs/services/gateway/api/openapi.yaml`

## Scope

Enhance `POST /api/v1/qa-sessions/{sessionId}/messages` through the QA-owned internal route so a user message can create and complete a non-streaming Agent Run.

The implementation must:

- Persist the user message, assistant placeholder message, `response_runs`, and initial `message.created` event consistently.
- Load the current QA config and LLM config for the run and store their version IDs on `response_runs`.
- Call AI Gateway through the QA model client using OpenAI-compatible non-streaming chat completions.
- Save a final assistant answer when the model directly returns final text.
- Persist `agent_model_invocations` with provider/profile/model, status, finish reason, token usage, latency, and safe error summary.
- Update `response_runs` with status, current iteration, max iterations, termination reason, token usage, latency, and completion time.
- Return a `QAAnswerResponse`-compatible payload containing `userMessage`, `assistantMessage`, `responseRun`, `citations`, and `reasoningSteps`.

## Required Termination Handling

The service must map Agent Run outcomes to stable statuses and termination reasons:

- Success: `status=completed`, `terminationReason=completed`.
- Max iterations: `status=failed`, `terminationReason=max_iterations`.
- Model or AI Gateway failure: `status=failed`, `terminationReason=model_error`, public error `dependency_error`.
- Timeout: `status=failed`, `terminationReason=timeout`, public error `dependency_error`.
- Cancellation: `status=cancelled`, `terminationReason=cancelled`.

## Security and Privacy

The implementation must not persist or return:

- Full prompts.
- Private chain-of-thought.
- Provider raw errors or raw responses.
- Provider API keys, bearer tokens, or internal URLs.
- Full MCP tool parameters or tool results.

Only safe summaries, stable error codes, model/profile identifiers, token counts, latency, and public reasoning step summaries may be persisted or returned.

## Acceptance Criteria

- `POST /api/v1/qa-sessions/{sessionId}/messages` succeeds for a non-streaming model response and returns `responseRunId`, `userMessageId`, `assistantMessageId`, and completed status information.
- User message, assistant message, response run, initial event, and final run state remain consistent after success and failure.
- Active QA/LLM configuration version IDs are attached to the response run.
- `agent_model_invocations` records successful and failed model calls without storing full prompt or provider raw error data.
- AI Gateway failures are returned as `dependency_error` and persisted as safe failure summaries.
- Tests cover success, model failure, timeout, cancellation, and max-iterations behavior.
- Existing #88 session/message resource behavior remains intact.

## Out of Scope

- Full MCP tool execution behavior beyond the existing runner surface.
- Streaming/SSE production behavior beyond preserving existing stream event compatibility.
- Frontend changes.
- AI Gateway provider implementation changes.
