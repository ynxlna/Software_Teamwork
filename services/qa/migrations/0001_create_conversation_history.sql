CREATE TABLE conversations (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id),
    role TEXT NOT NULL,
    status TEXT NOT NULL,
    sequence_no INTEGER NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT uniq_messages_conversation_sequence UNIQUE (conversation_id, sequence_no)
);

CREATE INDEX idx_messages_conversation_sequence
    ON messages (conversation_id, sequence_no);

CREATE TABLE message_content_blocks (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES messages(id),
    block_type TEXT NOT NULL,
    text_content TEXT NOT NULL DEFAULT '',
    sequence_no INTEGER NOT NULL
);

CREATE INDEX idx_message_content_blocks_message_sequence
    ON message_content_blocks (message_id, sequence_no);

CREATE TABLE citations (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES messages(id),
    document_id TEXT NOT NULL,
    chunk_id TEXT NOT NULL,
    source_title TEXT NOT NULL DEFAULT '',
    source_text TEXT NOT NULL DEFAULT '',
    score DOUBLE PRECISION NOT NULL DEFAULT 0,
    sequence_no INTEGER NOT NULL
);

CREATE INDEX idx_citations_message_sequence
    ON citations (message_id, sequence_no);

CREATE TABLE response_runs (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES messages(id),
    status TEXT NOT NULL,
    error_code TEXT,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ
);

CREATE INDEX idx_response_runs_message
    ON response_runs (message_id);

CREATE TABLE response_process_steps (
    id TEXT PRIMARY KEY,
    response_run_id TEXT NOT NULL REFERENCES response_runs(id),
    title TEXT NOT NULL,
    status TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    sequence_no INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_response_process_steps_run_sequence
    ON response_process_steps (response_run_id, sequence_no);
