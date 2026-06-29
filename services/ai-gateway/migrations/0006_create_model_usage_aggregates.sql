-- +goose Up
CREATE TABLE IF NOT EXISTS model_usage_aggregates (
    id TEXT PRIMARY KEY,
    bucket_start_at TIMESTAMPTZ NOT NULL,
    bucket_granularity TEXT NOT NULL CHECK (bucket_granularity IN ('hour')),
    caller_service TEXT NOT NULL,
    profile_id TEXT NOT NULL REFERENCES model_profiles(id) ON DELETE RESTRICT,
    operation TEXT NOT NULL CHECK (operation IN ('chat_completion', 'embedding', 'reranking')),
    request_count BIGINT NOT NULL DEFAULT 0 CHECK (request_count >= 0),
    success_count BIGINT NOT NULL DEFAULT 0 CHECK (success_count >= 0),
    failure_count BIGINT NOT NULL DEFAULT 0 CHECK (failure_count >= 0),
    prompt_tokens BIGINT NOT NULL DEFAULT 0 CHECK (prompt_tokens >= 0),
    completion_tokens BIGINT NOT NULL DEFAULT 0 CHECK (completion_tokens >= 0),
    total_tokens BIGINT NOT NULL DEFAULT 0 CHECK (total_tokens >= 0),
    total_duration_ms BIGINT NOT NULL DEFAULT 0 CHECK (total_duration_ms >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (bucket_start_at, bucket_granularity, caller_service, profile_id, operation)
);

CREATE INDEX IF NOT EXISTS idx_model_usage_aggregates_bucket
    ON model_usage_aggregates (bucket_start_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_usage_aggregates_caller_bucket
    ON model_usage_aggregates (caller_service, bucket_start_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_usage_aggregates_profile_bucket
    ON model_usage_aggregates (profile_id, bucket_start_at DESC);
