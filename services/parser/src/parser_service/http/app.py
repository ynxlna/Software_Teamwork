from __future__ import annotations

import hmac
import logging
import uuid
from typing import Annotated

from fastapi import Depends, FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse

from parser_service.backends.document import DocumentParserBackend
from parser_service.backends.paddleocr import PaddleOCRBackend
from parser_service.config import Settings
from parser_service.http.schemas import CreateParsedDocumentRequest
from parser_service.service import AppError, ParsedDocument, ParserService

logger = logging.getLogger(__name__)


def create_app(
    *,
    settings: Settings | None = None,
    parser_service: ParserService | None = None,
) -> FastAPI:
    settings = settings or Settings.from_env()
    parser_service = parser_service or build_parser_service(settings)

    app = FastAPI(
        title="Parser Runtime Internal API",
        version="0.1.0",
        docs_url=None,
        redoc_url=None,
        openapi_url=None,
    )
    app.state.settings = settings
    app.state.parser_service = parser_service

    @app.middleware("http")
    async def request_id_middleware(request: Request, call_next):
        request.state.request_id = _request_id(request)
        return await call_next(request)

    @app.exception_handler(AppError)
    async def app_error_handler(request: Request, exc: AppError) -> JSONResponse:
        return _error_response(request, exc.status_code, exc.code, exc.message, exc.fields)

    @app.exception_handler(RequestValidationError)
    async def request_validation_handler(
        request: Request,
        exc: RequestValidationError,
    ) -> JSONResponse:
        fields: dict[str, str] = {}
        for item in exc.errors():
            loc = ".".join(str(part) for part in item.get("loc", []) if part != "body")
            fields[loc or "body"] = "invalid"
        return _error_response(
            request,
            400,
            "validation_error",
            "request validation failed",
            fields or None,
        )

    @app.exception_handler(Exception)
    async def unexpected_error_handler(request: Request, exc: Exception) -> JSONResponse:
        logger.exception(
            "parser request failed",
            extra={
                "service": settings.service_name,
                "request_id": getattr(request.state, "request_id", ""),
                "operation": request.url.path,
                "status": "failed",
            },
        )
        return _error_response(request, 500, "internal_error", "internal server error")

    @app.get("/healthz")
    async def healthz(request: Request) -> JSONResponse:
        return _success_response(
            request,
            {
                "service": settings.service_name,
                "status": "ok",
            },
        )

    @app.get("/readyz")
    async def readyz(request: Request) -> JSONResponse:
        health = parser_service.health()
        data = {
            "service": settings.service_name,
            "status": health.status,
            "backend": parser_service.backend_name,
        }
        if health.reason:
            data["reason"] = health.reason
        return _success_response(request, data, status_code=200 if health.ready else 503)

    @app.post("/internal/v1/parsed-documents")
    async def create_parsed_document(
        request: Request,
        payload: CreateParsedDocumentRequest,
        _: Annotated[None, Depends(require_service_token)],
    ) -> JSONResponse:
        parsed = await parser_service.parse_document(
            document_name=payload.document_name or "",
            content_type=payload.content_type or "",
            size_bytes=payload.size_bytes,
            data_base64=payload.data_base64,
        )
        return _success_response(request, _parsed_document_data(parsed))

    return app


def build_parser_service(settings: Settings) -> ParserService:
    backend_name = settings.backend.strip().lower()
    if backend_name != "paddleocr":
        raise ValueError("PARSER_BACKEND must be paddleocr")
    ocr_backend = PaddleOCRBackend(
        lang=settings.paddleocr_lang,
        device=settings.paddleocr_device,
        engine=settings.paddleocr_engine,
        paddlex_config=settings.paddleocr_config_path,
        use_doc_orientation_classify=settings.paddleocr_use_doc_orientation_classify,
        use_doc_unwarping=settings.paddleocr_use_doc_unwarping,
        use_textline_orientation=settings.paddleocr_use_textline_orientation,
    )
    backend = DocumentParserBackend(ocr_backend=ocr_backend)
    service = ParserService(
        backend=backend,
        max_document_bytes=settings.max_document_bytes,
        max_concurrency=settings.max_concurrency,
        queue_timeout_seconds=settings.queue_timeout_seconds,
        parse_timeout_seconds=settings.parse_timeout_seconds,
    )
    if settings.load_backend_on_startup:
        service.warm_up()
    return service


def require_service_token(request: Request) -> None:
    settings: Settings = request.app.state.settings
    expected = settings.service_token
    if not expected:
        return
    supplied = request.headers.get("X-Service-Token", "").strip()
    if not supplied:
        raise AppError(code="unauthorized", message="service token is required", status_code=401)
    if not hmac.compare_digest(supplied, expected):
        raise AppError(code="forbidden", message="service token is invalid", status_code=403)


def _success_response(request: Request, data: object, status_code: int = 200) -> JSONResponse:
    return JSONResponse(
        status_code=status_code,
        content={
            "data": data,
            "requestId": request.state.request_id,
        },
    )


def _error_response(
    request: Request,
    status_code: int,
    code: str,
    message: str,
    fields: dict[str, str] | None = None,
) -> JSONResponse:
    error: dict[str, object] = {
        "code": code,
        "message": message,
        "requestId": getattr(request.state, "request_id", _request_id(request)),
    }
    if fields:
        error["fields"] = fields
    return JSONResponse(status_code=status_code, content={"error": error})


def _request_id(request: Request) -> str:
    supplied = request.headers.get("X-Request-Id", "").strip()
    if supplied:
        return supplied
    return "req_" + uuid.uuid4().hex[:16]


def _parsed_document_data(parsed: ParsedDocument) -> dict[str, object]:
    data: dict[str, object] = {
        "content": parsed.content,
        "title": parsed.title or None,
        "backend": parsed.backend,
    }
    if parsed.pages:
        data["pages"] = [
            {
                "pageNumber": page.page_number,
                "content": page.content,
            }
            for page in parsed.pages
        ]
    return data
