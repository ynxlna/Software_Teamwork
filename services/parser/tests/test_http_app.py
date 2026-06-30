import base64

from fastapi.testclient import TestClient

from parser_service.config import Settings
from parser_service.http import create_app
from parser_service.service import BackendHealth, ParsedDocument, ParsedPage, ParserService


class FakeBackend:
    name = "fake"

    def __init__(self, *, ready: bool = True, content: str = " parsed text ") -> None:
        self.ready = ready
        self.content = content
        self.requests = []

    def health(self) -> BackendHealth:
        return BackendHealth(
            ready=self.ready,
            status="ready" if self.ready else "not_ready",
            reason="" if self.ready else "backend unavailable",
        )

    def warm_up(self) -> None:
        return None

    def parse(self, request):
        self.requests.append(request)
        return ParsedDocument(
            content=self.content,
            title=" Remote Title ",
            backend=self.name,
            pages=[ParsedPage(page_number=1, content=" page one ")],
        )


def test_healthz_does_not_require_service_token():
    client = _client(service_token="secret")

    response = client.get("/healthz", headers={"X-Request-Id": "req_123"})

    assert response.status_code == 200
    assert response.json() == {
        "data": {"service": "parser", "status": "ok"},
        "requestId": "req_123",
    }


def test_readyz_reports_backend_unavailable():
    client = _client(backend=FakeBackend(ready=False))

    response = client.get("/readyz", headers={"X-Request-Id": "req_123"})

    assert response.status_code == 503
    assert response.json()["data"] == {
        "service": "parser",
        "status": "not_ready",
        "backend": "fake",
        "reason": "backend unavailable",
    }


def test_create_parsed_document_requires_token_when_configured():
    client = _client(service_token="secret")

    response = client.post(
        "/internal/v1/parsed-documents",
        json={"dataBase64": _b64(b"hello")},
        headers={"X-Request-Id": "req_123"},
    )

    assert response.status_code == 401
    assert response.json() == {
        "error": {
            "code": "unauthorized",
            "message": "service token is required",
            "requestId": "req_123",
        }
    }


def test_create_parsed_document_rejects_invalid_token():
    client = _client(service_token="secret")

    response = client.post(
        "/internal/v1/parsed-documents",
        json={"dataBase64": _b64(b"hello")},
        headers={"X-Service-Token": "wrong", "X-Request-Id": "req_123"},
    )

    assert response.status_code == 403
    assert response.json()["error"]["code"] == "forbidden"


def test_create_parsed_document_returns_standard_envelope():
    backend = FakeBackend(content=" line one \n\n line two ")
    client = _client(backend=backend, service_token="secret")

    response = client.post(
        "/internal/v1/parsed-documents",
        json={
            "documentName": "scan.pdf",
            "contentType": "application/pdf",
            "sizeBytes": 5,
            "dataBase64": _b64(b"hello"),
        },
        headers={"X-Service-Token": "secret", "X-Request-Id": "req_123"},
    )

    assert response.status_code == 200
    assert response.json() == {
        "data": {
            "content": "line one\nline two",
            "title": "Remote Title",
            "backend": "fake",
            "pages": [{"pageNumber": 1, "content": "page one"}],
        },
        "requestId": "req_123",
    }
    assert backend.requests[0].document_name == "scan.pdf"
    assert backend.requests[0].content_type == "application/pdf"
    assert backend.requests[0].data == b"hello"


def test_create_parsed_document_validation_uses_project_error_shape():
    client = _client()

    response = client.post(
        "/internal/v1/parsed-documents",
        json={},
        headers={"X-Request-Id": "req_123"},
    )

    assert response.status_code == 400
    assert response.json() == {
        "error": {
            "code": "validation_error",
            "message": "request validation failed",
            "requestId": "req_123",
            "fields": {"dataBase64": "invalid"},
        }
    }


def _client(
    *,
    backend: FakeBackend | None = None,
    service_token: str = "",
    max_document_bytes: int = 1024,
) -> TestClient:
    backend = backend or FakeBackend()
    service = ParserService(
        backend=backend,
        max_document_bytes=max_document_bytes,
        parse_timeout_seconds=5,
    )
    app = create_app(
        settings=Settings(service_token=service_token),
        parser_service=service,
    )
    return TestClient(app)


def _b64(value: bytes) -> str:
    return base64.b64encode(value).decode("ascii")
