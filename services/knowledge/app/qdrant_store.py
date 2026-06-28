import json
import urllib.error
import urllib.request
from typing import Any

from app.config import EMBEDDING_DIMENSION, QDRANT_COLLECTION, QDRANT_URL


def qdrant_request(method: str, path: str, payload: dict[str, Any] | None = None) -> dict[str, Any]:
    data = None
    headers = {"Content-Type": "application/json"}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(f"{QDRANT_URL}{path}", data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            body = response.read()
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="ignore")
        raise RuntimeError(f"Qdrant {method} {path} failed: {exc.code} {detail}") from exc
    if not body:
        return {}
    return json.loads(body.decode("utf-8"))


def ensure_collection() -> None:
    try:
        qdrant_request("GET", f"/collections/{QDRANT_COLLECTION}")
        return
    except RuntimeError:
        pass

    qdrant_request(
        "PUT",
        f"/collections/{QDRANT_COLLECTION}",
        {
            "vectors": {
                "size": EMBEDDING_DIMENSION,
                "distance": "Cosine",
            }
        },
    )


def upsert_points(points: list[dict[str, Any]]) -> None:
    if not points:
        return
    ensure_collection()
    qdrant_request(
        "PUT",
        f"/collections/{QDRANT_COLLECTION}/points?wait=true",
        {"points": points},
    )


def delete_points(point_ids: list[str]) -> None:
    if not point_ids:
        return
    ensure_collection()
    qdrant_request(
        "POST",
        f"/collections/{QDRANT_COLLECTION}/points/delete?wait=true",
        {"points": point_ids},
    )


def set_payload_for_points(point_ids: list[str], payload: dict[str, Any]) -> None:
    if not point_ids:
        return
    ensure_collection()
    qdrant_request(
        "POST",
        f"/collections/{QDRANT_COLLECTION}/points/payload?wait=true",
        {
            "payload": payload,
            "points": point_ids,
        },
    )


def search_points(vector: list[float], limit: int, query_filter: dict[str, Any] | None = None) -> list[dict[str, Any]]:
    ensure_collection()
    payload: dict[str, Any] = {
        "vector": vector,
        "limit": limit,
        "with_payload": True,
    }
    if query_filter:
        payload["filter"] = query_filter
    response = qdrant_request("POST", f"/collections/{QDRANT_COLLECTION}/points/search", payload)
    result = response.get("result", [])
    if not isinstance(result, list):
        return []
    return result
