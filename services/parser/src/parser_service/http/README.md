# Parser HTTP

Internal HTTP handlers for parser service routes belong here.

Handlers should expose `/healthz`, `/readyz`, and `/internal/v1/parsed-documents`
using the standard project JSON envelope and sanitized error responses.
