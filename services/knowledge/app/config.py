import os


def get_env(name: str, default: str) -> str:
    return os.getenv(name, default)


def get_int_env(name: str, default: int) -> int:
    raw_value = os.getenv(name)
    if raw_value is None or raw_value == "":
        return default
    return int(raw_value)


POSTGRES_DSN = get_env("POSTGRES_DSN", "postgresql://knowledge:knowledge@localhost:5432/knowledge")
QDRANT_URL = get_env("QDRANT_URL", "http://localhost:6333").rstrip("/")
QDRANT_COLLECTION = get_env("QDRANT_COLLECTION", "knowledge_chunks")
EMBEDDING_PROVIDER = get_env("EMBEDDING_PROVIDER", "local_hashing")
EMBEDDING_DIMENSION = get_int_env("EMBEDDING_DIMENSION", 384)
EMBEDDING_API_BASE = get_env("EMBEDDING_API_BASE", "https://api.siliconflow.cn/v1").rstrip("/")
EMBEDDING_API_KEY = get_env("EMBEDDING_API_KEY", "")
EMBEDDING_MODEL = get_env("EMBEDDING_MODEL", "BAAI/bge-m3")
CHUNK_MAX_CHARS = get_int_env("CHUNK_MAX_CHARS", 1600)
CHUNK_OVERLAP_CHARS = get_int_env("CHUNK_OVERLAP_CHARS", 200)
