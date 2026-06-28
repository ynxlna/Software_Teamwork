# QA Service

The QA service owns chat conversation history, message aggregation, context
construction, and future LLM orchestration.

This initial implementation supports task 3.3.2:

- maintain multi-turn context on the backend,
- return structured conversation history,
- include `completed`, `stopped`, and `failed` assistant messages,
- expose content blocks, process steps, citations, and error codes,
- reject cross-user conversation access.

## Local Development

```bash
go test ./...
go build ./cmd/server
go run ./cmd/server
```

## Temporary Auth Contract

Until the gateway and auth service are implemented, handlers read the
authenticated user ID from `X-User-ID`. Gateway integration should replace this
with a signed and validated identity context later.

## Endpoints

```text
GET  /api/conversations/{conversation_id}
POST /api/chat/stream
```

`POST /api/chat/stream` accepts only the current user message. Previous history
is loaded by the backend from persisted conversation messages.
