import json
from typing import Any

from app.database import db_connection


def execute_one(query: str, params: tuple[Any, ...]) -> dict[str, Any] | None:
    with db_connection() as connection:
        with connection.cursor() as cursor:
            cursor.execute(query, params)
            row = cursor.fetchone()
        connection.commit()
        return row


def execute_many(query: str, params: list[tuple[Any, ...]]) -> None:
    if not params:
        return
    with db_connection() as connection:
        with connection.cursor() as cursor:
            cursor.executemany(query, params)
        connection.commit()


def fetch_all(query: str, params: tuple[Any, ...] = ()) -> list[dict[str, Any]]:
    with db_connection() as connection:
        with connection.cursor() as cursor:
            cursor.execute(query, params)
            rows = cursor.fetchall()
        return list(rows)


def json_dumps(value: object) -> str:
    return json.dumps(value, ensure_ascii=False)
