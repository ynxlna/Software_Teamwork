import hashlib
import json
import math
import re
import urllib.error
import urllib.request

from app.config import (
    EMBEDDING_API_BASE,
    EMBEDDING_API_KEY,
    EMBEDDING_DIMENSION,
    EMBEDDING_MODEL,
    EMBEDDING_PROVIDER,
)


TOKEN_PATTERN = re.compile(r"[A-Za-z0-9_./:+-]+|[\u4e00-\u9fff]")


def tokenize(text: str) -> list[str]:
    return [token.lower() for token in TOKEN_PATTERN.findall(text)]


def embed_text(text: str) -> list[float]:
    if EMBEDDING_PROVIDER == "local_hashing":
        return embed_text_local_hashing(text)
    if EMBEDDING_PROVIDER in {"openai_compatible", "siliconflow"}:
        return embed_text_openai_compatible(text)
    raise ValueError(f"Unsupported embedding provider: {EMBEDDING_PROVIDER}")


def embed_text_local_hashing(text: str) -> list[float]:
    vector = [0.0] * EMBEDDING_DIMENSION
    for token in tokenize(text):
        digest = hashlib.sha256(token.encode("utf-8")).digest()
        index = int.from_bytes(digest[:4], "big") % EMBEDDING_DIMENSION
        sign = 1.0 if digest[4] % 2 == 0 else -1.0
        vector[index] += sign

    norm = math.sqrt(sum(value * value for value in vector))
    if norm == 0:
        return vector

    return [round(value / norm, 6) for value in vector]


def embed_text_openai_compatible(text: str) -> list[float]:
    if not EMBEDDING_API_KEY:
        raise ValueError("EMBEDDING_API_KEY is required for openai-compatible embeddings")

    payload = json.dumps({"model": EMBEDDING_MODEL, "input": text}).encode("utf-8")
    request = urllib.request.Request(
        f"{EMBEDDING_API_BASE}/embeddings",
        data=payload,
        headers={
            "Authorization": f"Bearer {EMBEDDING_API_KEY}",
            "Content-Type": "application/json",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=60) as response:
            body = response.read()
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="ignore")
        raise RuntimeError(f"Embedding API failed: {exc.code} {detail}") from exc

    result = json.loads(body.decode("utf-8"))
    vector = result["data"][0]["embedding"]
    if len(vector) != EMBEDDING_DIMENSION:
        raise ValueError(
            f"Embedding dimension mismatch: got {len(vector)}, expected {EMBEDDING_DIMENSION}"
        )
    return [float(value) for value in vector]


def embedding_provider_name() -> str:
    return EMBEDDING_PROVIDER


def embedding_dimension() -> int:
    return EMBEDDING_DIMENSION
