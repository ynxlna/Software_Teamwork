from __future__ import annotations

import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    service_name: str = "parser"
    host: str = "0.0.0.0"
    port: int = 8080
    service_token: str = ""
    backend: str = "paddleocr"
    max_document_bytes: int = 8 * 1024 * 1024
    max_concurrency: int = 1
    queue_timeout_seconds: float = 0.0
    parse_timeout_seconds: float = 120.0
    load_backend_on_startup: bool = False
    paddleocr_lang: str = "ch"
    paddleocr_device: str = "cpu"
    paddleocr_engine: str = ""
    paddleocr_config_path: str = ""
    paddleocr_use_doc_orientation_classify: bool = False
    paddleocr_use_doc_unwarping: bool = False
    paddleocr_use_textline_orientation: bool = False

    @classmethod
    def from_env(cls) -> Settings:
        return cls(
            host=_string("PARSER_HOST", cls.host),
            port=_int("PARSER_PORT", cls.port, minimum=1, maximum=65535),
            service_token=_string("PARSER_SERVICE_TOKEN", cls.service_token),
            backend=_string("PARSER_BACKEND", cls.backend),
            max_document_bytes=_int(
                "PARSER_MAX_DOCUMENT_BYTES",
                cls.max_document_bytes,
                minimum=1,
            ),
            max_concurrency=_int("PARSER_MAX_CONCURRENCY", cls.max_concurrency, minimum=1),
            queue_timeout_seconds=_float(
                "PARSER_QUEUE_TIMEOUT_SECONDS",
                cls.queue_timeout_seconds,
                minimum=0,
            ),
            parse_timeout_seconds=_float(
                "PARSER_PARSE_TIMEOUT_SECONDS",
                cls.parse_timeout_seconds,
                minimum=1,
            ),
            load_backend_on_startup=_bool(
                "PARSER_LOAD_BACKEND_ON_STARTUP",
                cls.load_backend_on_startup,
            ),
            paddleocr_lang=_string("PADDLEOCR_LANG", cls.paddleocr_lang),
            paddleocr_device=_string("PADDLEOCR_DEVICE", cls.paddleocr_device),
            paddleocr_engine=_string("PADDLEOCR_ENGINE", cls.paddleocr_engine),
            paddleocr_config_path=_string("PADDLEOCR_CONFIG_PATH", cls.paddleocr_config_path),
            paddleocr_use_doc_orientation_classify=_bool(
                "PADDLEOCR_USE_DOC_ORIENTATION_CLASSIFY",
                cls.paddleocr_use_doc_orientation_classify,
            ),
            paddleocr_use_doc_unwarping=_bool(
                "PADDLEOCR_USE_DOC_UNWARPING",
                cls.paddleocr_use_doc_unwarping,
            ),
            paddleocr_use_textline_orientation=_bool(
                "PADDLEOCR_USE_TEXTLINE_ORIENTATION",
                cls.paddleocr_use_textline_orientation,
            ),
        )


def _string(name: str, default: str) -> str:
    return os.environ.get(name, default).strip()


def _int(name: str, default: int, *, minimum: int, maximum: int | None = None) -> int:
    raw = os.environ.get(name, "").strip()
    if raw == "":
        return default
    try:
        value = int(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be an integer") from exc
    if value < minimum:
        raise ValueError(f"{name} must be >= {minimum}")
    if maximum is not None and value > maximum:
        raise ValueError(f"{name} must be <= {maximum}")
    return value


def _float(name: str, default: float, *, minimum: float) -> float:
    raw = os.environ.get(name, "").strip()
    if raw == "":
        return default
    try:
        value = float(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be a number") from exc
    if value < minimum:
        raise ValueError(f"{name} must be >= {minimum}")
    return value


def _bool(name: str, default: bool) -> bool:
    raw = os.environ.get(name, "").strip().lower()
    if raw == "":
        return default
    if raw in {"1", "true", "yes", "on"}:
        return True
    if raw in {"0", "false", "no", "off"}:
        return False
    raise ValueError(f"{name} must be a boolean")
