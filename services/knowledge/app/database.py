from collections.abc import Iterator
from contextlib import contextmanager

import psycopg
from psycopg.rows import dict_row

from app.config import POSTGRES_DSN


@contextmanager
def db_connection() -> Iterator[psycopg.Connection]:
    with psycopg.connect(POSTGRES_DSN, row_factory=dict_row) as connection:
        yield connection


def init_db() -> None:
    statements = [
        """
        CREATE TABLE IF NOT EXISTS knowledge_bases (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL,
            description TEXT NOT NULL DEFAULT '',
            doc_type TEXT NOT NULL DEFAULT 'GENERAL',
            chunk_strategy JSONB NOT NULL DEFAULT '{}'::jsonb,
            retrieval_strategy JSONB NOT NULL DEFAULT '{}'::jsonb,
            document_count INTEGER NOT NULL DEFAULT 0,
            chunk_count INTEGER NOT NULL DEFAULT 0,
            created_by TEXT NOT NULL DEFAULT 'local',
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )
        """,
        """
        CREATE TABLE IF NOT EXISTS documents (
            id TEXT PRIMARY KEY,
            kb_id TEXT NOT NULL REFERENCES knowledge_bases(id),
            filename TEXT NOT NULL,
            content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
            size_bytes BIGINT NOT NULL DEFAULT 0,
            status TEXT NOT NULL,
            error_code TEXT,
            error_message TEXT,
            chunk_count INTEGER NOT NULL DEFAULT 0,
            tags JSONB NOT NULL DEFAULT '{}'::jsonb,
            parser_backend TEXT NOT NULL DEFAULT 'auto',
            created_by TEXT NOT NULL DEFAULT 'local',
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )
        """,
        """
        CREATE TABLE IF NOT EXISTS ingest_jobs (
            id TEXT PRIMARY KEY,
            document_id TEXT NOT NULL REFERENCES documents(id),
            job_type TEXT NOT NULL DEFAULT 'INGEST',
            status TEXT NOT NULL,
            current_stage TEXT NOT NULL,
            progress INTEGER NOT NULL DEFAULT 0,
            message TEXT NOT NULL DEFAULT '',
            error_code TEXT,
            error_message TEXT,
            started_at TIMESTAMPTZ,
            finished_at TIMESTAMPTZ,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )
        """,
        """
        CREATE TABLE IF NOT EXISTS document_chunks (
            id TEXT PRIMARY KEY,
            kb_id TEXT NOT NULL REFERENCES knowledge_bases(id),
            document_id TEXT NOT NULL REFERENCES documents(id),
            chunk_index INTEGER NOT NULL,
            section_path TEXT,
            content TEXT NOT NULL,
            content_hash TEXT NOT NULL,
            token_count INTEGER NOT NULL DEFAULT 0,
            chunk_type TEXT NOT NULL DEFAULT 'text',
            qdrant_point_id TEXT NOT NULL,
            embedding_provider TEXT NOT NULL,
            embedding_dimension INTEGER NOT NULL,
            embedding_preview JSONB NOT NULL DEFAULT '[]'::jsonb,
            metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )
        """,
        "CREATE INDEX IF NOT EXISTS idx_documents_kb_id ON documents(kb_id)",
        "CREATE INDEX IF NOT EXISTS idx_document_chunks_document_id ON document_chunks(document_id)",
        "CREATE INDEX IF NOT EXISTS idx_document_chunks_kb_id ON document_chunks(kb_id)",
        "CREATE INDEX IF NOT EXISTS idx_ingest_jobs_document_id ON ingest_jobs(document_id)",
    ]

    with db_connection() as connection:
        with connection.cursor() as cursor:
            for statement in statements:
                cursor.execute(statement)
        connection.commit()
