-- name: CreateKnowledgeBase :one
INSERT INTO knowledge_bases (
  id,
  name,
  description,
  doc_type,
  chunk_strategy,
  retrieval_strategy,
  created_by,
  created_at,
  updated_at
) VALUES (
  sqlc.arg(id),
  sqlc.arg(name),
  sqlc.arg(description),
  sqlc.arg(doc_type),
  sqlc.arg(chunk_strategy),
  sqlc.arg(retrieval_strategy),
  sqlc.arg(created_by),
  sqlc.arg(created_at),
  sqlc.arg(updated_at)
)
RETURNING
  id,
  name,
  description,
  doc_type,
  chunk_strategy,
  retrieval_strategy,
  0::bigint AS document_count,
  0::bigint AS chunk_count,
  created_by,
  created_at,
  updated_at,
  deleted_at;

-- name: ListKnowledgeBases :many
SELECT
  kb.id,
  kb.name,
  kb.description,
  kb.doc_type,
  kb.chunk_strategy,
  kb.retrieval_strategy,
  COALESCE(doc_counts.document_count, 0)::bigint AS document_count,
  COALESCE(chunk_counts.chunk_count, 0)::bigint AS chunk_count,
  kb.created_by,
  kb.created_at,
  kb.updated_at,
  kb.deleted_at
FROM knowledge_bases kb
LEFT JOIN (
  SELECT knowledge_base_id, COUNT(*)::bigint AS document_count
  FROM knowledge_documents
  WHERE deleted_at IS NULL
  GROUP BY knowledge_base_id
) doc_counts ON doc_counts.knowledge_base_id = kb.id
LEFT JOIN (
  SELECT dc.knowledge_base_id, COUNT(*)::bigint AS chunk_count
  FROM document_chunks dc
  JOIN knowledge_documents d ON d.id = dc.document_id
  WHERE d.deleted_at IS NULL
  GROUP BY dc.knowledge_base_id
) chunk_counts ON chunk_counts.knowledge_base_id = kb.id
WHERE kb.deleted_at IS NULL
  AND (sqlc.arg(can_read_all)::boolean OR kb.created_by = sqlc.arg(user_id))
ORDER BY kb.created_at DESC, kb.id DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountKnowledgeBases :one
SELECT COUNT(*)::bigint
FROM knowledge_bases kb
WHERE kb.deleted_at IS NULL
  AND (sqlc.arg(can_read_all)::boolean OR kb.created_by = sqlc.arg(user_id));

-- name: GetKnowledgeBase :one
SELECT
  kb.id,
  kb.name,
  kb.description,
  kb.doc_type,
  kb.chunk_strategy,
  kb.retrieval_strategy,
  COALESCE(doc_counts.document_count, 0)::bigint AS document_count,
  COALESCE(chunk_counts.chunk_count, 0)::bigint AS chunk_count,
  kb.created_by,
  kb.created_at,
  kb.updated_at,
  kb.deleted_at
FROM knowledge_bases kb
LEFT JOIN (
  SELECT knowledge_base_id, COUNT(*)::bigint AS document_count
  FROM knowledge_documents
  WHERE deleted_at IS NULL
  GROUP BY knowledge_base_id
) doc_counts ON doc_counts.knowledge_base_id = kb.id
LEFT JOIN (
  SELECT dc.knowledge_base_id, COUNT(*)::bigint AS chunk_count
  FROM document_chunks dc
  JOIN knowledge_documents d ON d.id = dc.document_id
  WHERE d.deleted_at IS NULL
  GROUP BY dc.knowledge_base_id
) chunk_counts ON chunk_counts.knowledge_base_id = kb.id
WHERE kb.id = sqlc.arg(id)
  AND kb.deleted_at IS NULL
  AND (sqlc.arg(can_read_all)::boolean OR kb.created_by = sqlc.arg(user_id));

-- name: UpdateKnowledgeBase :execrows
UPDATE knowledge_bases
SET
  name = sqlc.arg(name),
  description = sqlc.arg(description),
  doc_type = sqlc.arg(doc_type),
  chunk_strategy = sqlc.arg(chunk_strategy),
  retrieval_strategy = sqlc.arg(retrieval_strategy),
  updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
  AND (sqlc.arg(can_read_all)::boolean OR created_by = sqlc.arg(user_id));

-- name: MarkKnowledgeBaseDeleted :execrows
UPDATE knowledge_bases
SET deleted_at = sqlc.arg(deleted_at), updated_at = sqlc.arg(deleted_at)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
  AND (sqlc.arg(can_read_all)::boolean OR created_by = sqlc.arg(user_id));

-- name: MarkDocumentsDeletedByKnowledgeBase :exec
UPDATE knowledge_documents
SET deleted_at = sqlc.arg(deleted_at), updated_at = sqlc.arg(deleted_at)
WHERE knowledge_base_id = sqlc.arg(knowledge_base_id)
  AND deleted_at IS NULL;

-- name: CreateDocument :one
INSERT INTO knowledge_documents (
  id,
  knowledge_base_id,
  file_ref,
  name,
  content_type,
  size_bytes,
  status,
  error_code,
  error_message,
  tags,
  parser_backend,
  current_job_id,
  created_by,
  created_at,
  updated_at
) VALUES (
  sqlc.arg(id),
  sqlc.arg(knowledge_base_id),
  NULLIF(sqlc.arg(file_ref), ''),
  sqlc.arg(name),
  NULLIF(sqlc.arg(content_type), ''),
  sqlc.arg(size_bytes),
  sqlc.arg(status),
  NULL,
  NULL,
  sqlc.arg(tags),
  NULL,
  NULLIF(sqlc.arg(current_job_id), ''),
  sqlc.arg(created_by),
  sqlc.arg(created_at),
  sqlc.arg(updated_at)
)
RETURNING
  id,
  knowledge_base_id,
  file_ref,
  name,
  content_type,
  size_bytes,
  status,
  error_code,
  error_message,
  0::bigint AS chunk_count,
  tags,
  parser_backend,
  current_job_id,
  created_by,
  created_at,
  updated_at,
  deleted_at;

-- name: CreateProcessingJob :one
INSERT INTO processing_jobs (
  id,
  knowledge_base_id,
  document_id,
  job_type,
  status,
  current_stage,
  progress_percent,
  message,
  error_code,
  error_message,
  attempts,
  max_attempts,
  parser_config_id,
  parser_config_snapshot,
  started_at,
  finished_at,
  created_at,
  updated_at
) VALUES (
  sqlc.arg(id),
  sqlc.arg(knowledge_base_id),
  NULLIF(sqlc.arg(document_id), ''),
  sqlc.arg(job_type),
  sqlc.arg(status),
  NULLIF(sqlc.arg(current_stage), ''),
  0,
  NULLIF(sqlc.arg(message), ''),
  NULL,
  NULL,
  0,
  sqlc.arg(max_attempts),
  NULLIF(sqlc.arg(parser_config_id), ''),
  sqlc.arg(parser_config_snapshot),
  NULL,
  NULL,
  sqlc.arg(created_at),
  sqlc.arg(updated_at)
)
RETURNING
  id,
  knowledge_base_id,
  document_id,
  job_type,
  status,
  current_stage,
  progress_percent,
  message,
  error_code,
  error_message,
  attempts,
  max_attempts,
  parser_config_id,
  parser_config_snapshot,
  started_at,
  finished_at,
  created_at,
  updated_at;

-- name: MarkDocumentFailed :execrows
UPDATE knowledge_documents
SET
  status = 'failed',
  error_code = NULLIF(sqlc.arg(error_code), ''),
  error_message = NULLIF(sqlc.arg(error_message), ''),
  updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: MarkProcessingJobFailed :execrows
UPDATE processing_jobs
SET
  status = 'failed',
  error_code = NULLIF(sqlc.arg(error_code), ''),
  error_message = NULLIF(sqlc.arg(error_message), ''),
  finished_at = sqlc.arg(finished_at),
  updated_at = sqlc.arg(finished_at)
WHERE id = sqlc.arg(id);

-- name: ListDocumentsByKnowledgeBase :many
SELECT
  d.id,
  d.knowledge_base_id,
  d.file_ref,
  d.name,
  d.content_type,
  d.size_bytes,
  d.status,
  d.error_code,
  d.error_message,
  COALESCE(chunk_counts.chunk_count, 0)::bigint AS chunk_count,
  d.tags,
  d.parser_backend,
  d.current_job_id,
  d.created_by,
  d.created_at,
  d.updated_at,
  d.deleted_at
FROM knowledge_documents d
JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
LEFT JOIN (
  SELECT document_id, COUNT(*)::bigint AS chunk_count
  FROM document_chunks
  GROUP BY document_id
) chunk_counts ON chunk_counts.document_id = d.id
WHERE d.knowledge_base_id = sqlc.arg(knowledge_base_id)
  AND d.deleted_at IS NULL
  AND kb.deleted_at IS NULL
  AND (sqlc.arg(can_read_all)::boolean OR d.created_by = sqlc.arg(user_id) OR kb.created_by = sqlc.arg(user_id))
  AND (sqlc.arg(status)::text = '' OR d.status = sqlc.arg(status))
ORDER BY d.created_at DESC, d.id DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountDocumentsByKnowledgeBase :one
SELECT COUNT(*)::bigint
FROM knowledge_documents d
JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
WHERE d.knowledge_base_id = sqlc.arg(knowledge_base_id)
  AND d.deleted_at IS NULL
  AND kb.deleted_at IS NULL
  AND (sqlc.arg(can_read_all)::boolean OR d.created_by = sqlc.arg(user_id) OR kb.created_by = sqlc.arg(user_id))
  AND (sqlc.arg(status)::text = '' OR d.status = sqlc.arg(status));

-- name: GetDocument :one
SELECT
  d.id,
  d.knowledge_base_id,
  d.file_ref,
  d.name,
  d.content_type,
  d.size_bytes,
  d.status,
  d.error_code,
  d.error_message,
  COALESCE(chunk_counts.chunk_count, 0)::bigint AS chunk_count,
  d.tags,
  d.parser_backend,
  d.current_job_id,
  d.created_by,
  d.created_at,
  d.updated_at,
  d.deleted_at
FROM knowledge_documents d
JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
LEFT JOIN (
  SELECT document_id, COUNT(*)::bigint AS chunk_count
  FROM document_chunks
  GROUP BY document_id
) chunk_counts ON chunk_counts.document_id = d.id
WHERE d.id = sqlc.arg(id)
  AND d.deleted_at IS NULL
  AND kb.deleted_at IS NULL
  AND (sqlc.arg(can_read_all)::boolean OR d.created_by = sqlc.arg(user_id) OR kb.created_by = sqlc.arg(user_id));

-- name: UpdateDocumentTags :execrows
UPDATE knowledge_documents d
SET
  tags = sqlc.arg(tags),
  updated_at = sqlc.arg(updated_at)
FROM knowledge_bases kb
WHERE d.id = sqlc.arg(id)
  AND kb.id = d.knowledge_base_id
  AND d.deleted_at IS NULL
  AND kb.deleted_at IS NULL
  AND (sqlc.arg(can_read_all)::boolean OR d.created_by = sqlc.arg(user_id) OR kb.created_by = sqlc.arg(user_id));

-- name: MarkDocumentDeleted :execrows
UPDATE knowledge_documents d
SET
  deleted_at = sqlc.arg(deleted_at),
  updated_at = sqlc.arg(deleted_at),
  current_job_id = sqlc.arg(cleanup_job_id)
FROM knowledge_bases kb
WHERE d.id = sqlc.arg(id)
  AND kb.id = d.knowledge_base_id
  AND d.deleted_at IS NULL
  AND kb.deleted_at IS NULL
  AND (sqlc.arg(can_read_all)::boolean OR d.created_by = sqlc.arg(user_id) OR kb.created_by = sqlc.arg(user_id));

-- name: GetDeletedDocumentKnowledgeBaseID :one
SELECT d.knowledge_base_id
FROM knowledge_documents d
JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
WHERE d.id = sqlc.arg(id)
  AND d.deleted_at IS NOT NULL
  AND kb.deleted_at IS NULL
  AND (sqlc.arg(can_read_all)::boolean OR d.created_by = sqlc.arg(user_id) OR kb.created_by = sqlc.arg(user_id));

-- name: CountDocumentChunks :one
SELECT COUNT(*)::bigint
FROM document_chunks dc
JOIN knowledge_documents d ON d.id = dc.document_id
JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
WHERE dc.document_id = sqlc.arg(document_id)
  AND d.deleted_at IS NULL
  AND kb.deleted_at IS NULL
  AND (sqlc.arg(can_read_all)::boolean OR d.created_by = sqlc.arg(user_id) OR kb.created_by = sqlc.arg(user_id));

-- name: ListDocumentChunks :many
SELECT
  dc.id,
  dc.knowledge_base_id,
  dc.document_id,
  dc.chunk_index,
  dc.section_path,
  dc.content,
  dc.token_count,
  dc.chunk_type,
  dc.qdrant_point_id,
  dc.embedding_provider,
  dc.embedding_dimension,
  dc.metadata,
  dc.created_at
FROM document_chunks dc
JOIN knowledge_documents d ON d.id = dc.document_id
JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
WHERE dc.document_id = sqlc.arg(document_id)
  AND d.deleted_at IS NULL
  AND kb.deleted_at IS NULL
  AND (sqlc.arg(can_read_all)::boolean OR d.created_by = sqlc.arg(user_id) OR kb.created_by = sqlc.arg(user_id))
ORDER BY dc.chunk_index ASC, dc.id ASC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);
