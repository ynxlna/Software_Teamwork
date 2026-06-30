from __future__ import annotations

from dataclasses import dataclass


@dataclass
class AppError(Exception):
    code: str
    message: str
    status_code: int
    fields: dict[str, str] | None = None

    def __str__(self) -> str:
        return self.message


def validation_error(message: str, fields: dict[str, str] | None = None) -> AppError:
    return AppError(code="validation_error", message=message, status_code=400, fields=fields)


def payload_too_large(message: str, fields: dict[str, str] | None = None) -> AppError:
    return AppError(code="validation_error", message=message, status_code=413, fields=fields)


def rate_limited(message: str) -> AppError:
    return AppError(code="rate_limited", message=message, status_code=429)


def dependency_error(message: str) -> AppError:
    return AppError(code="dependency_error", message=message, status_code=502)
