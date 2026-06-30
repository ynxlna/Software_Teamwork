# Parser Service Package

Python parser runtime code lives under this package.

The package exposes the internal HTTP contract documented in
`services/parser/api/openapi.yaml` and keeps PaddleOCR runtime dependencies out
of Knowledge. The HTTP layer owns request/response envelopes, the service layer
owns validation and concurrency limits, and backend adapters own format-specific
text extraction.
