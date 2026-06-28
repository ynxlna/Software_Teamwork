CREATE TABLE IF NOT EXISTS knowledge_bases (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    doc_type TEXT NOT NULL,
    chunk_strategy JSONB NOT NULL,
    retrieval_strategy JSONB NOT NULL,
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_knowledge_bases_created_at
    ON knowledge_bases (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_knowledge_bases_doc_type
    ON knowledge_bases (doc_type)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_knowledge_bases_deleted_at
    ON knowledge_bases (deleted_at);

CREATE TABLE IF NOT EXISTS knowledge_documents (
    id TEXT PRIMARY KEY,
    knowledge_base_id TEXT NOT NULL REFERENCES knowledge_bases(id),
    file_id TEXT NOT NULL,
    name TEXT NOT NULL,
    content_type TEXT,
    size_bytes BIGINT,
    status TEXT NOT NULL,
    error_code TEXT,
    error_message TEXT,
    parsed_content TEXT,
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    parser_backend TEXT,
    created_by TEXT,
    current_job_id TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_knowledge_documents_knowledge_base_id
    ON knowledge_documents (knowledge_base_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_knowledge_documents_status
    ON knowledge_documents (status)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_knowledge_documents_file_id
    ON knowledge_documents (file_id);

CREATE INDEX IF NOT EXISTS idx_knowledge_documents_created_at
    ON knowledge_documents (created_at DESC);

CREATE TABLE IF NOT EXISTS processing_jobs (
    id TEXT PRIMARY KEY,
    knowledge_base_id TEXT NOT NULL REFERENCES knowledge_bases(id),
    document_id TEXT REFERENCES knowledge_documents(id),
    job_type TEXT NOT NULL,
    status TEXT NOT NULL,
    current_stage TEXT,
    progress_percent INTEGER NOT NULL DEFAULT 0,
    message TEXT,
    error_code TEXT,
    error_message TEXT,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    idempotency_key TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_processing_jobs_status_created_at
    ON processing_jobs (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_processing_jobs_document_id
    ON processing_jobs (document_id);

CREATE INDEX IF NOT EXISTS idx_processing_jobs_knowledge_base_id
    ON processing_jobs (knowledge_base_id);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_processing_jobs_idempotency_key
    ON processing_jobs (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS document_chunks (
    id TEXT PRIMARY KEY,
    knowledge_base_id TEXT NOT NULL REFERENCES knowledge_bases(id),
    document_id TEXT NOT NULL REFERENCES knowledge_documents(id),
    chunk_index INTEGER NOT NULL,
    section_path TEXT,
    content TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    chunk_type TEXT,
    qdrant_point_id TEXT,
    embedding_provider TEXT,
    embedding_model TEXT,
    embedding_dimension INTEGER,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (document_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_document_chunks_document_id_chunk_index
    ON document_chunks (document_id, chunk_index);

CREATE INDEX IF NOT EXISTS idx_document_chunks_knowledge_base_id
    ON document_chunks (knowledge_base_id);

CREATE INDEX IF NOT EXISTS idx_document_chunks_qdrant_point_id
    ON document_chunks (qdrant_point_id);
