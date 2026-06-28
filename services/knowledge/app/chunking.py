import hashlib
import re
from dataclasses import dataclass

from app.config import CHUNK_MAX_CHARS, CHUNK_OVERLAP_CHARS


HEADING_PATTERN = re.compile(r"^(#{1,6}\s+.+|[0-9]+(?:\.[0-9]+)*\s+.+)$")


@dataclass(frozen=True)
class TextChunk:
    chunk_index: int
    section_path: str
    content: str
    content_hash: str
    token_count: int


def content_hash(content: str) -> str:
    return hashlib.sha256(content.encode("utf-8")).hexdigest()


def token_count(content: str) -> int:
    return len(re.findall(r"\S+", content))


def split_long_text(text: str, max_chars: int, overlap_chars: int) -> list[str]:
    if len(text) <= max_chars:
        return [text]

    parts: list[str] = []
    start = 0
    while start < len(text):
        end = min(start + max_chars, len(text))
        if end < len(text):
            newline = text.rfind("\n", start, end)
            if newline > start + max_chars // 2:
                end = newline
        part = text[start:end].strip()
        if part:
            parts.append(part)
        if end >= len(text):
            break
        next_start = max(end - overlap_chars, start + 1)
        newline = text.find("\n", next_start, end)
        if newline != -1:
            next_start = newline + 1
        start = next_start
    return parts


def semantic_chunk_text(text: str) -> list[TextChunk]:
    normalized = text.replace("\r\n", "\n").replace("\r", "\n").strip()
    if not normalized:
        return []

    blocks: list[tuple[str, str]] = []
    section = "root"
    current: list[str] = []

    def flush_current() -> None:
        nonlocal current
        block = "\n".join(current).strip()
        if block:
            blocks.append((section, block))
        current = []

    for line in normalized.split("\n"):
        stripped = line.strip()
        if stripped and HEADING_PATTERN.match(stripped) and len(stripped) <= 160:
            flush_current()
            section = stripped.lstrip("#").strip()
            current.append(stripped)
            continue

        if not stripped:
            flush_current()
            continue

        current.append(line.rstrip())

    flush_current()

    chunks: list[TextChunk] = []
    buffer: list[str] = []
    buffer_section = "root"

    def emit_buffer() -> None:
        nonlocal buffer, buffer_section
        content = "\n\n".join(buffer).strip()
        if not content:
            buffer = []
            return
        for part in split_long_text(content, CHUNK_MAX_CHARS, CHUNK_OVERLAP_CHARS):
            chunks.append(
                TextChunk(
                    chunk_index=len(chunks),
                    section_path=buffer_section,
                    content=part,
                    content_hash=content_hash(part),
                    token_count=token_count(part),
                )
            )
        buffer = []

    for block_section, block in blocks:
        projected = "\n\n".join([*buffer, block]).strip()
        if buffer and len(projected) > CHUNK_MAX_CHARS:
            emit_buffer()
        if not buffer:
            buffer_section = block_section
        buffer.append(block)

    emit_buffer()
    return chunks
