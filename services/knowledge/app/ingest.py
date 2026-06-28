from typing import Any
from uuid import uuid4

from app.chunking import semantic_chunk_text
from app.embedding import embed_text, embedding_dimension, embedding_provider_name
from app.ids import new_id
from app.parsers import parse_document
from app.qdrant_store import upsert_points
from app.repository import execute_many, execute_one, json_dumps


def update_document_status(
    document_id: str,
    status: str,
    error_code: str | None = None,
    error_message: str | None = None,
    chunk_count: int | None = None,
) -> None:
    execute_one(
        """
        UPDATE documents
        SET status = %s,
            error_code = %s,
            error_message = %s,
            chunk_count = COALESCE(%s, chunk_count),
            updated_at = now()
        WHERE id = %s
        RETURNING id
        """,
        (status, error_code, error_message, chunk_count, document_id),
    )


def update_job(
    job_id: str,
    status: str,
    stage: str,
    progress: int,
    message: str,
    error_code: str | None = None,
    error_message: str | None = None,
    finished: bool = False,
) -> None:
    finished_sql = "now()" if finished else "finished_at"
    execute_one(
        f"""
        UPDATE ingest_jobs
        SET status = %s,
            current_stage = %s,
            progress = %s,
            message = %s,
            error_code = %s,
            error_message = %s,
            finished_at = {finished_sql},
            updated_at = now()
        WHERE id = %s
        RETURNING id
        """,
        (status, stage, progress, message, error_code, error_message, job_id),
    )


def process_uploaded_document(
    *,
    kb_id: str,
    document_id: str,
    job_id: str,
    filename: str,
    content: bytes,
    tags: dict[str, Any],
) -> dict[str, Any]:
    try:
        update_document_status(document_id, "parsing")
        update_job(job_id, "running", "parsing", 20, "parsing document")
        parsed = parse_document(filename, content)
        execute_one(
            """
            UPDATE documents
            SET parser_backend = %s,
                updated_at = now()
            WHERE id = %s
            RETURNING id
            """,
            (parsed.parser_backend, document_id),
        )

        update_document_status(document_id, "chunking")
        update_job(job_id, "running", "chunking", 45, "chunking document")
        chunks = semantic_chunk_text(parsed.text)
        if not chunks:
            raise ValueError("Parsed document produced no text chunks")

        update_document_status(document_id, "embedding")
        update_job(job_id, "running", "embedding", 70, f"embedding {len(chunks)} chunks")

        points: list[dict[str, Any]] = []
        rows: list[tuple[Any, ...]] = []
        provider = embedding_provider_name()
        dimension = embedding_dimension()

        for chunk in chunks:
            chunk_id = new_id("chunk")
            point_id = str(uuid4())
            vector = embed_text(chunk.content)
            preview = vector[:8]
            metadata = {
                "parser": parsed.parser_backend,
                "parsed_metadata": parsed.metadata,
            }
            payload = {
                "kb_id": kb_id,
                "document_id": document_id,
                "chunk_id": chunk_id,
                "filename": filename,
                "section_path": chunk.section_path,
                "tags": tags,
                "chunk_index": chunk.chunk_index,
                "chunk_type": "text",
            }
            points.append({"id": point_id, "vector": vector, "payload": payload})
            rows.append(
                (
                    chunk_id,
                    kb_id,
                    document_id,
                    chunk.chunk_index,
                    chunk.section_path,
                    chunk.content,
                    chunk.content_hash,
                    chunk.token_count,
                    "text",
                    point_id,
                    provider,
                    dimension,
                    json_dumps(preview),
                    json_dumps(metadata),
                )
            )

        update_job(job_id, "running", "indexing", 85, f"upserting {len(points)} Qdrant points")
        upsert_points(points)

        execute_many(
            """
            INSERT INTO document_chunks (
                id, kb_id, document_id, chunk_index, section_path, content,
                content_hash, token_count, chunk_type, qdrant_point_id,
                embedding_provider, embedding_dimension, embedding_preview, metadata
            )
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s::jsonb, %s::jsonb)
            """,
            rows,
        )

        update_document_status(document_id, "ready", chunk_count=len(chunks))
        update_job(job_id, "succeeded", "done", 100, "document ready", finished=True)
        execute_one(
            """
            UPDATE knowledge_bases
            SET document_count = (
                    SELECT count(*) FROM documents
                    WHERE kb_id = %s
                ),
                chunk_count = (
                    SELECT count(*) FROM document_chunks
                    WHERE kb_id = %s
                ),
                updated_at = now()
            WHERE id = %s
            RETURNING id
            """,
            (kb_id, kb_id, kb_id),
        )
        return {"chunk_count": len(chunks), "parser_backend": parsed.parser_backend}
    except Exception as exc:
        update_document_status(document_id, "failed", "ingest_failed", str(exc))
        update_job(
            job_id,
            "failed",
            "failed",
            100,
            "document ingest failed",
            "ingest_failed",
            str(exc),
            finished=True,
        )
        raise
