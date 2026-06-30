# Parser HTTP

Internal HTTP handlers for parser service routes live here.

Handlers expose `/healthz`, `/readyz`, and `/internal/v1/parsed-documents` using
the standard project JSON envelope and sanitized error responses. The stable
contract is documented in `services/parser/api/openapi.yaml`.
