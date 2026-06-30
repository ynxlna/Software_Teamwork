-- name: InsertResponseRun :one
INSERT INTO response_runs (
    conversation_id,
    user_message_id,
    assistant_message_id,
    qa_config_version_id,
    llm_config_version_id,
    request_id,
    intent_type,
    route,
    status,
    max_iterations
) VALUES (
    sqlc.arg(conversation_id)::uuid,
    sqlc.arg(user_message_id)::uuid,
    sqlc.arg(assistant_message_id)::uuid,
    COALESCE(NULLIF(sqlc.arg(qa_config_version_id), '')::uuid, (SELECT id FROM qa_config_versions WHERE is_active = true ORDER BY version_no DESC LIMIT 1)),
    COALESCE(NULLIF(sqlc.arg(llm_config_version_id), '')::uuid, (SELECT id FROM llm_config_versions WHERE is_active = true ORDER BY version_no DESC LIMIT 1)),
    NULLIF(sqlc.arg(request_id), ''),
    NULLIF(sqlc.arg(intent_type), ''),
    'agent',
    'running',
    GREATEST(
        COALESCE(
            NULLIF(sqlc.arg(max_iterations), 0),
            (SELECT max_iterations FROM qa_config_versions WHERE is_active = true ORDER BY version_no DESC LIMIT 1),
            5
        ),
        1
    )
)
RETURNING
    id::text,
    conversation_id::text,
    user_message_id::text,
    assistant_message_id::text,
    status,
    current_iteration,
    max_iterations,
    started_at;

-- name: GetResponseRunForUser :one
SELECT
    rr.id::text,
    rr.conversation_id::text,
    rr.user_message_id::text,
    rr.assistant_message_id::text,
    rr.status,
    COALESCE(rr.current_iteration, 0)::integer,
    COALESCE(rr.max_iterations, 5)::integer,
    rr.stop_reason,
    COALESCE(rr.prompt_tokens, 0) + COALESCE(rr.completion_tokens, 0) + COALESCE(rr.reasoning_tokens, 0),
    COALESCE(rr.latency_ms, 0)::bigint,
    rr.started_at,
    rr.completed_at
FROM response_runs rr
JOIN conversations c ON c.id = rr.conversation_id
WHERE rr.id::text = sqlc.arg(id)::text
    AND c.external_user_id = sqlc.arg(external_user_id)
    AND c.deleted_at IS NULL;

-- name: GetResponseRunIDByAssistantMessage :one
SELECT rr.id::text
FROM response_runs rr
JOIN conversations c ON c.id = rr.conversation_id
WHERE rr.assistant_message_id = sqlc.arg(assistant_message_id)::uuid
    AND c.external_user_id = sqlc.arg(external_user_id);

-- name: UpdateResponseRunByAssistantMessage :exec
UPDATE response_runs
SET status = sqlc.arg(status),
    stop_reason = CASE
        WHEN sqlc.arg(status) = 'completed' THEN NULL
        ELSE sqlc.arg(status)
    END,
    completed_at = CASE
        WHEN sqlc.arg(status) <> 'running' THEN now()
        ELSE NULL
    END,
    latency_ms = CASE
        WHEN sqlc.arg(status) <> 'running' THEN EXTRACT(EPOCH FROM (now() - started_at)) * 1000
        ELSE NULL
    END
WHERE assistant_message_id = sqlc.arg(assistant_message_id)::uuid;

-- name: FinalizeResponseRun :one
UPDATE response_runs rr
SET status = sqlc.arg(status),
    stop_reason = NULLIF(sqlc.arg(termination_reason), ''),
    current_iteration = GREATEST(rr.current_iteration, sqlc.arg(current_iteration)),
    prompt_tokens = COALESCE(NULLIF(sqlc.arg(prompt_tokens), 0), (
        SELECT SUM(prompt_tokens)::integer FROM agent_model_invocations WHERE response_run_id = rr.id
    ), rr.prompt_tokens),
    completion_tokens = COALESCE(NULLIF(sqlc.arg(completion_tokens), 0), (
        SELECT SUM(completion_tokens)::integer FROM agent_model_invocations WHERE response_run_id = rr.id
    ), rr.completion_tokens),
    reasoning_tokens = COALESCE(NULLIF(sqlc.arg(reasoning_tokens), 0), (
        SELECT SUM(reasoning_tokens)::integer FROM agent_model_invocations WHERE response_run_id = rr.id
    ), rr.reasoning_tokens),
    completed_at = sqlc.arg(completed_at),
    latency_ms = EXTRACT(EPOCH FROM (sqlc.arg(completed_at) - rr.started_at)) * 1000
FROM conversations c
WHERE rr.id::text = sqlc.arg(id)::text
    AND c.id = rr.conversation_id
    AND c.external_user_id = sqlc.arg(external_user_id)
    AND c.deleted_at IS NULL
    AND rr.status = 'running'
RETURNING
    rr.id::text,
    rr.conversation_id::text,
    rr.user_message_id::text,
    rr.assistant_message_id::text,
    rr.status,
    COALESCE(rr.current_iteration, 0)::integer,
    COALESCE(rr.max_iterations, 5)::integer,
    rr.stop_reason,
    COALESCE(rr.prompt_tokens, 0) + COALESCE(rr.completion_tokens, 0) + COALESCE(rr.reasoning_tokens, 0),
    COALESCE(rr.latency_ms, 0)::bigint,
    rr.started_at,
    rr.completed_at;

-- name: UpdateResponseRunIteration :exec
UPDATE response_runs
SET current_iteration = GREATEST(current_iteration, sqlc.arg(iteration_no))
WHERE id = sqlc.arg(id)::uuid;

-- name: CancelResponseRun :one
UPDATE response_runs rr
SET status = 'cancelled',
    stop_reason = 'cancelled',
    completed_at = now(),
    latency_ms = EXTRACT(EPOCH FROM (now() - started_at)) * 1000
FROM conversations c
WHERE rr.id::text = sqlc.arg(id)::text
    AND c.id = rr.conversation_id
    AND c.external_user_id = sqlc.arg(external_user_id)
    AND c.deleted_at IS NULL
    AND rr.status IN ('running')
RETURNING rr.assistant_message_id::text;

-- name: AuthorizeResponseRunForUser :one
SELECT true
FROM response_runs rr
JOIN conversations c ON c.id = rr.conversation_id
WHERE rr.id::text = sqlc.arg(id)::text
    AND c.external_user_id = sqlc.arg(external_user_id);
