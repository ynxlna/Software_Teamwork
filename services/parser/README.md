# Parser Runtime Service

This directory defines the internal document parser runtime called by Knowledge
ingestion.

Parser is not a business owner service. Knowledge remains the owner of
knowledge documents, ingestion jobs, chunks, embeddings, Qdrant indexing,
retrieval, and parser runtime configuration. Parser only converts raw document
bytes into normalized parsed content.

## Runtime Direction

The first target backend is PaddleOCR for OCR-heavy PDFs, scanned pages,
images, tables, seals, and complex layouts. The intended implementation runtime
is Python because PaddleOCR is Python-first. Go services should call Parser over
HTTP and should not host PaddleOCR runtime dependencies.

This scaffold intentionally does not add PaddleOCR, Python packaging, Docker,
or runtime dependencies. Those belong in a follow-up implementation slice.

## Planned Shape

```text
services/parser/
  api/
    openapi.yaml
  src/
    parser_service/
      backends/
        paddleocr/
      config/
      http/
      service/
```

## Internal Contract

Primary route:

```text
POST /internal/v1/parsed-documents
```

The request carries base64 raw document bytes plus metadata such as file name,
content type, and size. The response returns normalized text, optional page and
block data, and sanitized backend metadata. Full object storage references,
provider bodies, raw OCR debug output, internal file paths, and secrets are not
part of the contract.
