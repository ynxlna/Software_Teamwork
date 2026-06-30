# Parser Config

Runtime configuration for the parser service lives here.

`Settings.from_env()` validates HTTP binding, service-token auth, document size
limits, parser concurrency, parse timeout, lazy/eager backend loading, and
PaddleOCR runtime options. Environment variables are documented in
`services/parser/README.md`.
