# Parser Runtime Service

This directory defines the internal document parser runtime that Knowledge calls
during ingestion.

Parser is not a business owner service. Knowledge remains the owner of
knowledge documents, ingestion jobs, chunks, embeddings, Qdrant indexing,
retrieval, and parser runtime configuration. Parser only converts raw document
bytes into normalized parsed content that Knowledge can validate, chunk, embed,
and index.

## Runtime

This service is a Python runtime managed with `uv`.

Implemented behavior:

- `GET /healthz` returns process liveness.
- `GET /readyz` reports whether the PaddleOCR runtime dependency is available.
- `POST /internal/v1/parsed-documents` accepts document bytes as base64 and
  returns normalized parsed text in the project `{ data, requestId }` envelope.
- TXT/Markdown, DOCX, PPTX, and XLSX text are parsed directly in the service.
- PDF and image input are routed to PaddleOCR.
- PaddleOCR loading is lazy by default so ordinary tests do not download or
  initialize OCR models.

## Directory Shape

```text
services/parser/
  pyproject.toml
  uv.lock
  Dockerfile
  api/
    openapi.yaml
  src/
    parser_service/
      config/
      http/
      service/
      backends/
        document.py
        paddleocr/
  tests/
```

The docs baseline separates service contracts under `docs/services/parser/api/`:

- `public.openapi.yaml` declares that Parser has no Gateway public API.
- `internal.openapi.yaml` defines the service-to-service Parser contract.

`services/parser/api/openapi.yaml` is the implementation-local copy used by the
Parser runtime and should stay aligned with the docs internal contract.

The implementation language is Python because PaddleOCR's maintained runtime
and examples are Python-first. Go stays on the Knowledge side as an HTTP client
to this service, not as the PaddleOCR runtime host.

## Internal Contract

Knowledge calls parser through the internal HTTP API instead of importing parser
implementation code or PaddleOCR dependencies.

Primary route:

```text
POST /internal/v1/parsed-documents
```

The route accepts raw document bytes as base64 plus metadata such as file name,
content type, and size. It returns normalized parsed text and backend metadata.
Full object storage references, provider bodies, raw OCR debug output, internal
file paths, and secrets are not part of the contract.

## Local Development

Install the non-OCR development dependencies:

```bash
cd services/parser
uv sync --group dev
```

Run checks:

```bash
uv run ruff check .
uv run pytest
uv run python -m compileall src tests
```

Run the service with the default dependency set:

```bash
uv run parser-service
```

Run with PaddleOCR installed locally:

```bash
uv sync --group dev --extra paddleocr
uv run parser-service
```

Build the runtime image:

```bash
docker build -t software-teamwork-parser:local .
```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `PARSER_HOST` | `0.0.0.0` | HTTP bind host. |
| `PARSER_PORT` | `8080` | HTTP bind port. |
| `PARSER_SERVICE_TOKEN` | empty | Optional expected `X-Service-Token`. |
| `PARSER_BACKEND` | `paddleocr` | Backend selector; only `paddleocr` is supported. |
| `PARSER_MAX_DOCUMENT_BYTES` | `8388608` | Maximum decoded document bytes. |
| `PARSER_MAX_CONCURRENCY` | `1` | Maximum concurrent parse jobs in one process. |
| `PARSER_QUEUE_TIMEOUT_SECONDS` | `0` | Queue wait timeout; `0` waits until capacity is available. |
| `PARSER_PARSE_TIMEOUT_SECONDS` | `120` | Per-document backend timeout. |
| `PARSER_LOAD_BACKEND_ON_STARTUP` | `false` | Eagerly load PaddleOCR at startup when true. |
| `PADDLEOCR_LANG` | `ch` | PaddleOCR language code. |
| `PADDLEOCR_DEVICE` | `cpu` | PaddleOCR device, for example `cpu` or `gpu`. |
| `PADDLEOCR_ENGINE` | empty | Optional PaddleOCR engine override. |
| `PADDLEOCR_CONFIG_PATH` | empty | Optional PaddleX config path. |
| `PADDLEOCR_USE_DOC_ORIENTATION_CLASSIFY` | `false` | PaddleOCR document orientation option. |
| `PADDLEOCR_USE_DOC_UNWARPING` | `false` | PaddleOCR document unwarping option. |
| `PADDLEOCR_USE_TEXTLINE_ORIENTATION` | `false` | PaddleOCR textline orientation option. |

## Deployment Boundary

Parser is deployed separately from Knowledge so OCR model loading, GPU/CPU
scheduling, and document parsing concurrency can evolve without coupling those
dependencies to the Knowledge service process.
