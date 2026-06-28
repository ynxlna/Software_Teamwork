package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateKnowledgeBase(ctx context.Context, base service.KnowledgeBase) (service.KnowledgeBase, error) {
	chunkStrategy, err := json.Marshal(base.ChunkStrategy)
	if err != nil {
		return service.KnowledgeBase{}, fmt.Errorf("marshal chunk strategy: %w", err)
	}
	retrievalStrategy, err := json.Marshal(base.RetrievalStrategy)
	if err != nil {
		return service.KnowledgeBase{}, fmt.Errorf("marshal retrieval strategy: %w", err)
	}

	const query = `
		INSERT INTO knowledge_bases (
			id, name, description, doc_type, chunk_strategy, retrieval_strategy,
			created_by, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8, $9)
		RETURNING id, name, description, doc_type, chunk_strategy, retrieval_strategy,
			created_by, created_at, updated_at, deleted_at, 0 AS document_count, 0 AS chunk_count
	`
	row := r.db.QueryRowContext(ctx, query,
		base.ID,
		base.Name,
		base.Description,
		base.DocType,
		string(chunkStrategy),
		string(retrievalStrategy),
		base.CreatedBy,
		base.CreatedAt,
		base.UpdatedAt,
	)
	created, err := scanKnowledgeBase(row)
	if err != nil {
		if isUniqueViolation(err) {
			return service.KnowledgeBase{}, service.ErrConflict
		}
		return service.KnowledgeBase{}, fmt.Errorf("insert knowledge base: %w", err)
	}
	return created, nil
}

func (r *PostgresRepository) ListKnowledgeBases(ctx context.Context, filter service.KnowledgeBaseFilter) (service.KnowledgeBaseList, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	where := []string{"kb.deleted_at IS NULL"}
	args := []any{}
	if strings.TrimSpace(filter.OwnerUserID) != "" {
		args = append(args, strings.TrimSpace(filter.OwnerUserID))
		where = append(where, fmt.Sprintf("kb.created_by = $%d", len(args)))
	}
	if strings.TrimSpace(filter.DocType) != "" {
		args = append(args, strings.TrimSpace(filter.DocType))
		where = append(where, fmt.Sprintf("kb.doc_type = $%d", len(args)))
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(filter.Keyword))+"%")
		where = append(where, fmt.Sprintf("(lower(kb.name) LIKE $%d OR lower(kb.description) LIKE $%d)", len(args), len(args)))
	}
	whereSQL := strings.Join(where, " AND ")

	countQuery := "SELECT count(*) FROM knowledge_bases kb WHERE " + whereSQL
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return service.KnowledgeBaseList{}, fmt.Errorf("count knowledge bases: %w", err)
	}

	args = append(args, pageSize, offset)
	listQuery := fmt.Sprintf(`
		SELECT
			kb.id,
			kb.name,
			kb.description,
			kb.doc_type,
			kb.chunk_strategy,
			kb.retrieval_strategy,
			kb.created_by,
			kb.created_at,
			kb.updated_at,
			kb.deleted_at,
			COALESCE(count(DISTINCT kd.id), 0) AS document_count,
			COALESCE(count(dc.id), 0) AS chunk_count
		FROM knowledge_bases kb
		LEFT JOIN knowledge_documents kd
			ON kd.knowledge_base_id = kb.id AND kd.deleted_at IS NULL
		LEFT JOIN document_chunks dc
			ON dc.knowledge_base_id = kb.id
		WHERE %s
		GROUP BY kb.id
		ORDER BY kb.created_at DESC, kb.id ASC
		LIMIT $%d OFFSET $%d
	`, whereSQL, len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return service.KnowledgeBaseList{}, fmt.Errorf("list knowledge bases: %w", err)
	}
	defer rows.Close()

	items := []service.KnowledgeBase{}
	for rows.Next() {
		base, err := scanKnowledgeBase(rows)
		if err != nil {
			return service.KnowledgeBaseList{}, err
		}
		items = append(items, base)
	}
	if err := rows.Err(); err != nil {
		return service.KnowledgeBaseList{}, fmt.Errorf("iterate knowledge bases: %w", err)
	}

	return service.KnowledgeBaseList{
		Items: items,
		Page: service.Page{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}, nil
}

func (r *PostgresRepository) FindKnowledgeBaseByID(ctx context.Context, id string) (service.KnowledgeBase, error) {
	const query = `
		SELECT
			kb.id,
			kb.name,
			kb.description,
			kb.doc_type,
			kb.chunk_strategy,
			kb.retrieval_strategy,
			kb.created_by,
			kb.created_at,
			kb.updated_at,
			kb.deleted_at,
			COALESCE(count(DISTINCT kd.id), 0) AS document_count,
			COALESCE(count(dc.id), 0) AS chunk_count
		FROM knowledge_bases kb
		LEFT JOIN knowledge_documents kd
			ON kd.knowledge_base_id = kb.id AND kd.deleted_at IS NULL
		LEFT JOIN document_chunks dc
			ON dc.knowledge_base_id = kb.id
		WHERE kb.id = $1 AND kb.deleted_at IS NULL
		GROUP BY kb.id
	`
	base, err := scanKnowledgeBase(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.KnowledgeBase{}, service.ErrNotFound
		}
		return service.KnowledgeBase{}, fmt.Errorf("find knowledge base: %w", err)
	}
	return base, nil
}

func (r *PostgresRepository) UpdateKnowledgeBase(ctx context.Context, base service.KnowledgeBase) (service.KnowledgeBase, error) {
	chunkStrategy, err := json.Marshal(base.ChunkStrategy)
	if err != nil {
		return service.KnowledgeBase{}, fmt.Errorf("marshal chunk strategy: %w", err)
	}
	retrievalStrategy, err := json.Marshal(base.RetrievalStrategy)
	if err != nil {
		return service.KnowledgeBase{}, fmt.Errorf("marshal retrieval strategy: %w", err)
	}

	const query = `
		UPDATE knowledge_bases
		SET name = $2,
			description = $3,
			doc_type = $4,
			chunk_strategy = $5::jsonb,
			retrieval_strategy = $6::jsonb,
			updated_at = $7
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, name, description, doc_type, chunk_strategy, retrieval_strategy,
			created_by, created_at, updated_at, deleted_at, 0 AS document_count, 0 AS chunk_count
	`
	updated, err := scanKnowledgeBase(r.db.QueryRowContext(ctx, query,
		base.ID,
		base.Name,
		base.Description,
		base.DocType,
		string(chunkStrategy),
		string(retrievalStrategy),
		base.UpdatedAt,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.KnowledgeBase{}, service.ErrNotFound
		}
		return service.KnowledgeBase{}, fmt.Errorf("update knowledge base: %w", err)
	}
	updated.DocumentCount = base.DocumentCount
	updated.ChunkCount = base.ChunkCount
	return updated, nil
}

func (r *PostgresRepository) MarkKnowledgeBaseDeleted(ctx context.Context, id string, deletedAt time.Time) error {
	const query = `
		UPDATE knowledge_bases
		SET deleted_at = $2, updated_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`
	result, err := r.db.ExecContext(ctx, query, id, deletedAt)
	if err != nil {
		return fmt.Errorf("delete knowledge base: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete knowledge base rows affected: %w", err)
	}
	if affected == 0 {
		return service.ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) ListDocuments(ctx context.Context, filter service.DocumentFilter) (service.DocumentList, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	where := []string{"deleted_at IS NULL", "knowledge_base_id = $1"}
	args := []any{filter.KnowledgeBaseID}
	if strings.TrimSpace(filter.OwnerUserID) != "" {
		args = append(args, strings.TrimSpace(filter.OwnerUserID))
		where = append(where, fmt.Sprintf("created_by = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, string(filter.Status))
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT count(*) FROM knowledge_documents WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return service.DocumentList{}, fmt.Errorf("count documents: %w", err)
	}

	args = append(args, pageSize, offset)
	query := fmt.Sprintf(`
		SELECT
			id,
			knowledge_base_id,
			file_id,
			name,
			content_type,
			size_bytes,
			status,
			error_code,
			error_message,
			parsed_content,
			COALESCE((
				SELECT count(*)
				FROM document_chunks dc
				WHERE dc.document_id = knowledge_documents.id
			), 0) AS chunk_count,
			tags,
			parser_backend,
			created_by,
			created_at,
			updated_at,
			deleted_at,
			current_job_id
		FROM knowledge_documents
		WHERE %s
		ORDER BY created_at DESC, id ASC
		LIMIT $%d OFFSET $%d
	`, whereSQL, len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return service.DocumentList{}, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	items := []service.KnowledgeDocument{}
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			return service.DocumentList{}, err
		}
		items = append(items, doc)
	}
	if err := rows.Err(); err != nil {
		return service.DocumentList{}, fmt.Errorf("iterate documents: %w", err)
	}

	return service.DocumentList{
		Items: items,
		Page: service.Page{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}, nil
}

func (r *PostgresRepository) CreateIngestionJob(ctx context.Context, doc service.KnowledgeDocument, job service.ProcessingJob) (service.KnowledgeDocument, service.ProcessingJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, fmt.Errorf("begin ingestion job transaction: %w", err)
	}
	defer tx.Rollback()

	if job.IdempotencyKey != nil {
		existingJob, err := findJobByIdempotencyKey(ctx, tx, *job.IdempotencyKey)
		if err == nil {
			if existingJob.DocumentID == nil {
				return service.KnowledgeDocument{}, service.ProcessingJob{}, service.ErrConflict
			}
			existingDoc, err := findDocumentByID(ctx, tx, *existingJob.DocumentID)
			if err != nil {
				return service.KnowledgeDocument{}, service.ProcessingJob{}, err
			}
			if err := tx.Commit(); err != nil {
				return service.KnowledgeDocument{}, service.ProcessingJob{}, fmt.Errorf("commit idempotent ingestion job transaction: %w", err)
			}
			return existingDoc, existingJob, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return service.KnowledgeDocument{}, service.ProcessingJob{}, err
		}
	}

	tags, err := json.Marshal(doc.Tags)
	if err != nil {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, fmt.Errorf("marshal document tags: %w", err)
	}
	documentID := doc.ID
	const insertDocument = `
		INSERT INTO knowledge_documents (
			id, knowledge_base_id, file_id, name, content_type, size_bytes,
			status, error_code, error_message, tags, parser_backend, created_by,
			current_job_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12, $13, $14, $15)
	`
	if _, err := tx.ExecContext(ctx, insertDocument,
		doc.ID,
		doc.KnowledgeBaseID,
		doc.FileID,
		doc.Name,
		nullableString(doc.ContentType),
		doc.SizeBytes,
		string(doc.Status),
		nullableString(doc.ErrorCode),
		nullableString(doc.ErrorMessage),
		string(tags),
		nullableString(doc.ParserBackend),
		doc.CreatedBy,
		nullableString(&job.ID),
		doc.CreatedAt,
		nullableTime(doc.UpdatedAt),
	); err != nil {
		if isUniqueViolation(err) {
			return service.KnowledgeDocument{}, service.ProcessingJob{}, service.ErrConflict
		}
		return service.KnowledgeDocument{}, service.ProcessingJob{}, fmt.Errorf("insert knowledge document: %w", err)
	}

	const insertJob = `
		INSERT INTO processing_jobs (
			id, knowledge_base_id, document_id, job_type, status, current_stage,
			progress_percent, message, error_code, error_message, attempts,
			max_attempts, idempotency_key, started_at, finished_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`
	if _, err := tx.ExecContext(ctx, insertJob,
		job.ID,
		job.KnowledgeBaseID,
		documentID,
		string(job.JobType),
		string(job.Status),
		nullableJobStage(job.CurrentStage),
		job.ProgressPercent,
		nullableString(job.Message),
		nullableString(job.ErrorCode),
		nullableString(job.ErrorMessage),
		job.Attempts,
		job.MaxAttempts,
		nullableString(job.IdempotencyKey),
		nullableTime(job.StartedAt),
		nullableTime(job.FinishedAt),
		job.CreatedAt,
		job.UpdatedAt,
	); err != nil {
		if isUniqueViolation(err) {
			return service.KnowledgeDocument{}, service.ProcessingJob{}, service.ErrConflict
		}
		return service.KnowledgeDocument{}, service.ProcessingJob{}, fmt.Errorf("insert processing job: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, fmt.Errorf("commit ingestion job transaction: %w", err)
	}
	return doc, job, nil
}

func (r *PostgresRepository) FindJobByID(ctx context.Context, id string) (service.ProcessingJob, error) {
	job, err := findJobByID(ctx, r.db, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ProcessingJob{}, service.ErrNotFound
		}
		return service.ProcessingJob{}, fmt.Errorf("find job: %w", err)
	}
	return job, nil
}

func (r *PostgresRepository) CreateProcessingJob(ctx context.Context, job service.ProcessingJob) (service.ProcessingJob, error) {
	const query = `
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
			idempotency_key,
			started_at,
			finished_at,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
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
			idempotency_key,
			started_at,
			finished_at,
			created_at,
			updated_at
	`
	created, err := scanJob(r.db.QueryRowContext(ctx, query,
		job.ID,
		job.KnowledgeBaseID,
		nullableString(job.DocumentID),
		job.JobType,
		job.Status,
		nullableJobStage(job.CurrentStage),
		job.ProgressPercent,
		nullableString(job.Message),
		nullableString(job.ErrorCode),
		nullableString(job.ErrorMessage),
		job.Attempts,
		job.MaxAttempts,
		nullableString(job.IdempotencyKey),
		nullableTime(job.StartedAt),
		nullableTime(job.FinishedAt),
		job.CreatedAt,
		job.UpdatedAt,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return service.ProcessingJob{}, service.ErrConflict
		}
		return service.ProcessingJob{}, fmt.Errorf("insert processing job: %w", err)
	}
	return created, nil
}

func (r *PostgresRepository) UpdateJobState(ctx context.Context, id string, update service.JobStateUpdate) (service.ProcessingJob, error) {
	const query = `
		UPDATE processing_jobs
		SET status = $2,
			current_stage = $3,
			progress_percent = $4,
			message = $5,
			error_code = $6,
			error_message = $7,
			attempts = COALESCE($8, attempts),
			started_at = COALESCE($9, started_at),
			finished_at = COALESCE($10, finished_at),
			updated_at = $11
		WHERE id = $1
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
			idempotency_key,
			started_at,
			finished_at,
			created_at,
			updated_at
	`
	job, err := scanJob(r.db.QueryRowContext(ctx, query,
		id,
		string(update.Status),
		nullableJobStage(update.CurrentStage),
		update.ProgressPercent,
		nullableString(update.Message),
		nullableString(update.ErrorCode),
		nullableString(update.ErrorMessage),
		nullableInt(update.Attempts),
		nullableTime(update.StartedAt),
		nullableTime(update.FinishedAt),
		update.UpdatedAt,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ProcessingJob{}, service.ErrNotFound
		}
		return service.ProcessingJob{}, fmt.Errorf("update job state: %w", err)
	}
	return job, nil
}

func (r *PostgresRepository) FindDocumentByID(ctx context.Context, id string) (service.KnowledgeDocument, error) {
	doc, err := findDocumentByID(ctx, r.db, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.KnowledgeDocument{}, service.ErrNotFound
		}
		return service.KnowledgeDocument{}, fmt.Errorf("find document: %w", err)
	}
	return doc, nil
}

func (r *PostgresRepository) UpdateDocumentProcessingState(ctx context.Context, id string, update service.DocumentStateUpdate) (service.KnowledgeDocument, error) {
	const query = `
		UPDATE knowledge_documents
		SET status = $2,
			error_code = $3,
			error_message = $4,
			parsed_content = COALESCE($5, parsed_content),
			updated_at = $6
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING
			id,
			knowledge_base_id,
			file_id,
			name,
			content_type,
			size_bytes,
			status,
			error_code,
			error_message,
			parsed_content,
			COALESCE((
				SELECT count(*)
				FROM document_chunks dc
				WHERE dc.document_id = knowledge_documents.id
			), 0) AS chunk_count,
			tags,
			parser_backend,
			created_by,
			created_at,
			updated_at,
			deleted_at,
			current_job_id
	`
	doc, err := scanDocument(r.db.QueryRowContext(ctx, query,
		id,
		string(update.Status),
		nullableString(update.ErrorCode),
		nullableString(update.ErrorMessage),
		nullableString(update.ParsedContent),
		update.UpdatedAt,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.KnowledgeDocument{}, service.ErrNotFound
		}
		return service.KnowledgeDocument{}, fmt.Errorf("update document state: %w", err)
	}
	if update.ChunkCount != nil {
		doc.ChunkCount = *update.ChunkCount
	}
	return doc, nil
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func findDocumentByID(ctx context.Context, db queryRower, id string) (service.KnowledgeDocument, error) {
	const query = `
		SELECT
			id,
			knowledge_base_id,
			file_id,
			name,
			content_type,
			size_bytes,
			status,
			error_code,
			error_message,
			parsed_content,
			COALESCE((
				SELECT count(*)
				FROM document_chunks dc
				WHERE dc.document_id = knowledge_documents.id
			), 0) AS chunk_count,
			tags,
			parser_backend,
			created_by,
			created_at,
			updated_at,
			deleted_at,
			current_job_id
		FROM knowledge_documents
		WHERE id = $1 AND deleted_at IS NULL
	`
	return scanDocument(db.QueryRowContext(ctx, query, id))
}

func (r *PostgresRepository) ListChunks(ctx context.Context, filter service.ChunkFilter) (service.ChunkList, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT count(*) FROM document_chunks WHERE document_id = $1", filter.DocumentID).Scan(&total); err != nil {
		return service.ChunkList{}, fmt.Errorf("count document chunks: %w", err)
	}

	const query = `
		SELECT
			id,
			knowledge_base_id,
			document_id,
			chunk_index,
			section_path,
			content,
			token_count,
			chunk_type,
			qdrant_point_id,
			embedding_provider,
			embedding_model,
			embedding_dimension,
			metadata,
			created_at
		FROM document_chunks
		WHERE document_id = $1
		ORDER BY chunk_index ASC, id ASC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, filter.DocumentID, pageSize, offset)
	if err != nil {
		return service.ChunkList{}, fmt.Errorf("list document chunks: %w", err)
	}
	defer rows.Close()

	items := []service.DocumentChunk{}
	for rows.Next() {
		chunk, err := scanChunk(rows)
		if err != nil {
			return service.ChunkList{}, err
		}
		items = append(items, chunk)
	}
	if err := rows.Err(); err != nil {
		return service.ChunkList{}, fmt.Errorf("iterate document chunks: %w", err)
	}

	return service.ChunkList{
		Items: items,
		Page: service.Page{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}, nil
}

func (r *PostgresRepository) ReplaceDocumentChunks(ctx context.Context, documentID string, chunks []service.DocumentChunk) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace chunks transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM document_chunks WHERE document_id = $1", documentID); err != nil {
		return fmt.Errorf("delete document chunks: %w", err)
	}
	const insertChunk = `
		INSERT INTO document_chunks (
			id, knowledge_base_id, document_id, chunk_index, section_path, content,
			token_count, chunk_type, qdrant_point_id, embedding_provider,
			embedding_model, embedding_dimension, metadata, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14)
	`
	for _, chunk := range chunks {
		metadata, err := json.Marshal(chunk.Metadata)
		if err != nil {
			return fmt.Errorf("marshal chunk metadata: %w", err)
		}
		if _, err := tx.ExecContext(ctx, insertChunk,
			chunk.ID,
			chunk.KnowledgeBaseID,
			chunk.DocumentID,
			chunk.ChunkIndex,
			nullableString(chunk.SectionPath),
			chunk.Content,
			chunk.TokenCount,
			nullableString(chunk.ChunkType),
			nullableString(chunk.QdrantPointID),
			nullableString(chunk.EmbeddingProvider),
			nullableString(chunk.EmbeddingModel),
			nullableInt(chunk.EmbeddingDimension),
			string(metadata),
			chunk.CreatedAt,
		); err != nil {
			return fmt.Errorf("insert document chunk: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace chunks transaction: %w", err)
	}
	return nil
}

func (r *PostgresRepository) FindChunksByIDs(ctx context.Context, ids []string) ([]service.DocumentChunk, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}
	query := fmt.Sprintf(`
		SELECT
			id,
			knowledge_base_id,
			document_id,
			chunk_index,
			section_path,
			content,
			token_count,
			chunk_type,
			qdrant_point_id,
			embedding_provider,
			embedding_model,
			embedding_dimension,
			metadata,
			created_at
		FROM document_chunks
		WHERE id IN (%s)
	`, strings.Join(placeholders, ", "))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find document chunks: %w", err)
	}
	defer rows.Close()

	chunksByID := map[string]service.DocumentChunk{}
	for rows.Next() {
		chunk, err := scanChunk(rows)
		if err != nil {
			return nil, err
		}
		chunksByID[chunk.ID] = chunk
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate document chunks: %w", err)
	}

	chunks := make([]service.DocumentChunk, 0, len(chunksByID))
	for _, id := range ids {
		if chunk, ok := chunksByID[id]; ok {
			chunks = append(chunks, chunk)
		}
	}
	return chunks, nil
}

func (r *PostgresRepository) GetKnowledgeStats(ctx context.Context, filter service.StatsFilter) (service.KnowledgeStats, error) {
	owner := strings.TrimSpace(filter.OwnerUserID)
	args := []any{}
	baseOwnerSQL := ""
	docOwnerSQL := ""
	if owner != "" {
		args = append(args, owner)
		baseOwnerSQL = fmt.Sprintf(" AND kb.created_by = $%d", len(args))
		docOwnerSQL = fmt.Sprintf(" AND kd.created_by = $%d", len(args))
	}

	query := fmt.Sprintf(`
		WITH visible_bases AS (
			SELECT kb.id
			FROM knowledge_bases kb
			WHERE kb.deleted_at IS NULL%s
		),
		visible_documents AS (
			SELECT kd.id, kd.status
			FROM knowledge_documents kd
			JOIN visible_bases vb ON vb.id = kd.knowledge_base_id
			WHERE kd.deleted_at IS NULL%s
		)
		SELECT
			(SELECT count(*) FROM visible_bases) AS knowledge_base_count,
			(SELECT count(*) FROM visible_documents) AS document_count,
			(SELECT count(*) FROM document_chunks dc JOIN visible_documents vd ON vd.id = dc.document_id) AS chunk_count,
			(SELECT count(*) FROM visible_documents WHERE status = 'ready') AS ready_document_count,
			(SELECT count(*) FROM visible_documents WHERE status = 'failed') AS failed_document_count
	`, baseOwnerSQL, docOwnerSQL)

	stats := service.KnowledgeStats{}
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.KnowledgeBaseCount,
		&stats.DocumentCount,
		&stats.ChunkCount,
		&stats.ReadyDocumentCount,
		&stats.FailedDocumentCount,
	); err != nil {
		return service.KnowledgeStats{}, fmt.Errorf("aggregate knowledge stats: %w", err)
	}

	uploadArgs := append([]any{}, args...)
	uploadWhere := []string{"kd.deleted_at IS NULL", "kb.deleted_at IS NULL"}
	if owner != "" {
		uploadWhere = append(uploadWhere, "kb.created_by = $1", "kd.created_by = $1")
	}
	if !filter.Since.IsZero() {
		uploadArgs = append(uploadArgs, filter.Since)
		uploadWhere = append(uploadWhere, fmt.Sprintf("kd.created_at >= $%d", len(uploadArgs)))
	}
	if !filter.Until.IsZero() {
		uploadArgs = append(uploadArgs, filter.Until)
		uploadWhere = append(uploadWhere, fmt.Sprintf("kd.created_at <= $%d", len(uploadArgs)))
	}
	uploadQuery := fmt.Sprintf(`
		SELECT to_char(date_trunc('day', kd.created_at AT TIME ZONE 'UTC'), 'YYYY-MM-DD') AS upload_date,
			count(*) AS upload_count
		FROM knowledge_documents kd
		JOIN knowledge_bases kb ON kb.id = kd.knowledge_base_id
		WHERE %s
		GROUP BY upload_date
		ORDER BY upload_date ASC
	`, strings.Join(uploadWhere, " AND "))

	rows, err := r.db.QueryContext(ctx, uploadQuery, uploadArgs...)
	if err != nil {
		return service.KnowledgeStats{}, fmt.Errorf("aggregate recent uploads: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item service.DailyUploadStat
		if err := rows.Scan(&item.Date, &item.Count); err != nil {
			return service.KnowledgeStats{}, fmt.Errorf("scan recent upload stat: %w", err)
		}
		stats.RecentUploads = append(stats.RecentUploads, item)
	}
	if err := rows.Err(); err != nil {
		return service.KnowledgeStats{}, fmt.Errorf("iterate recent upload stats: %w", err)
	}

	return stats, nil
}

type knowledgeBaseScanner interface {
	Scan(dest ...any) error
}

func scanKnowledgeBase(scanner knowledgeBaseScanner) (service.KnowledgeBase, error) {
	var base service.KnowledgeBase
	var chunkStrategyBytes []byte
	var retrievalStrategyBytes []byte
	var createdBy sql.NullString
	var deletedAt sql.NullTime
	if err := scanner.Scan(
		&base.ID,
		&base.Name,
		&base.Description,
		&base.DocType,
		&chunkStrategyBytes,
		&retrievalStrategyBytes,
		&createdBy,
		&base.CreatedAt,
		&base.UpdatedAt,
		&deletedAt,
		&base.DocumentCount,
		&base.ChunkCount,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.KnowledgeBase{}, err
		}
		return service.KnowledgeBase{}, fmt.Errorf("scan knowledge base: %w", err)
	}
	if createdBy.Valid {
		base.CreatedBy = createdBy.String
	}
	if deletedAt.Valid {
		deleted := deletedAt.Time
		base.DeletedAt = &deleted
	}
	if err := json.Unmarshal(chunkStrategyBytes, &base.ChunkStrategy); err != nil {
		return service.KnowledgeBase{}, fmt.Errorf("decode chunk strategy: %w", err)
	}
	if err := json.Unmarshal(retrievalStrategyBytes, &base.RetrievalStrategy); err != nil {
		return service.KnowledgeBase{}, fmt.Errorf("decode retrieval strategy: %w", err)
	}
	return base, nil
}

func scanDocument(scanner knowledgeBaseScanner) (service.KnowledgeDocument, error) {
	var doc service.KnowledgeDocument
	var contentType sql.NullString
	var status string
	var errorCode sql.NullString
	var errorMessage sql.NullString
	var parsedContent sql.NullString
	var tagsBytes []byte
	var parserBackend sql.NullString
	var createdBy sql.NullString
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	var currentJobID sql.NullString
	if err := scanner.Scan(
		&doc.ID,
		&doc.KnowledgeBaseID,
		&doc.FileID,
		&doc.Name,
		&contentType,
		&doc.SizeBytes,
		&status,
		&errorCode,
		&errorMessage,
		&parsedContent,
		&doc.ChunkCount,
		&tagsBytes,
		&parserBackend,
		&createdBy,
		&doc.CreatedAt,
		&updatedAt,
		&deletedAt,
		&currentJobID,
	); err != nil {
		return service.KnowledgeDocument{}, err
	}
	doc.Status = service.DocumentStatus(status)
	if contentType.Valid {
		doc.ContentType = &contentType.String
	}
	if errorCode.Valid {
		doc.ErrorCode = &errorCode.String
	}
	if errorMessage.Valid {
		doc.ErrorMessage = &errorMessage.String
	}
	if parsedContent.Valid {
		doc.ParsedContent = &parsedContent.String
	}
	if len(tagsBytes) > 0 {
		if err := json.Unmarshal(tagsBytes, &doc.Tags); err != nil {
			return service.KnowledgeDocument{}, fmt.Errorf("decode document tags: %w", err)
		}
	}
	if parserBackend.Valid {
		doc.ParserBackend = &parserBackend.String
	}
	if createdBy.Valid {
		doc.CreatedBy = createdBy.String
	}
	if updatedAt.Valid {
		doc.UpdatedAt = &updatedAt.Time
	}
	if deletedAt.Valid {
		doc.DeletedAt = &deletedAt.Time
	}
	if currentJobID.Valid {
		doc.CurrentJobID = &currentJobID.String
	}
	return doc, nil
}

func scanChunk(scanner knowledgeBaseScanner) (service.DocumentChunk, error) {
	var chunk service.DocumentChunk
	var sectionPath sql.NullString
	var chunkType sql.NullString
	var qdrantPointID sql.NullString
	var embeddingProvider sql.NullString
	var embeddingModel sql.NullString
	var embeddingDimension sql.NullInt64
	var metadataBytes []byte
	if err := scanner.Scan(
		&chunk.ID,
		&chunk.KnowledgeBaseID,
		&chunk.DocumentID,
		&chunk.ChunkIndex,
		&sectionPath,
		&chunk.Content,
		&chunk.TokenCount,
		&chunkType,
		&qdrantPointID,
		&embeddingProvider,
		&embeddingModel,
		&embeddingDimension,
		&metadataBytes,
		&chunk.CreatedAt,
	); err != nil {
		return service.DocumentChunk{}, err
	}
	if sectionPath.Valid {
		chunk.SectionPath = &sectionPath.String
	}
	if chunkType.Valid {
		chunk.ChunkType = &chunkType.String
	}
	if qdrantPointID.Valid {
		chunk.QdrantPointID = &qdrantPointID.String
	}
	if embeddingProvider.Valid {
		chunk.EmbeddingProvider = &embeddingProvider.String
	}
	if embeddingModel.Valid {
		chunk.EmbeddingModel = &embeddingModel.String
	}
	if embeddingDimension.Valid {
		value := int(embeddingDimension.Int64)
		chunk.EmbeddingDimension = &value
	}
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &chunk.Metadata); err != nil {
			return service.DocumentChunk{}, fmt.Errorf("decode chunk metadata: %w", err)
		}
	}
	if chunk.Metadata == nil {
		chunk.Metadata = map[string]any{}
	}
	return chunk, nil
}

func findJobByID(ctx context.Context, db queryRower, id string) (service.ProcessingJob, error) {
	const query = `
		SELECT
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
			idempotency_key,
			started_at,
			finished_at,
			created_at,
			updated_at
		FROM processing_jobs
		WHERE id = $1
	`
	return scanJob(db.QueryRowContext(ctx, query, id))
}

func findJobByIdempotencyKey(ctx context.Context, db queryRower, key string) (service.ProcessingJob, error) {
	const query = `
		SELECT
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
			idempotency_key,
			started_at,
			finished_at,
			created_at,
			updated_at
		FROM processing_jobs
		WHERE idempotency_key = $1
	`
	return scanJob(db.QueryRowContext(ctx, query, key))
}

func scanJob(scanner knowledgeBaseScanner) (service.ProcessingJob, error) {
	var job service.ProcessingJob
	var documentID sql.NullString
	var jobType string
	var status string
	var stage sql.NullString
	var message sql.NullString
	var errorCode sql.NullString
	var errorMessage sql.NullString
	var idempotencyKey sql.NullString
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := scanner.Scan(
		&job.ID,
		&job.KnowledgeBaseID,
		&documentID,
		&jobType,
		&status,
		&stage,
		&job.ProgressPercent,
		&message,
		&errorCode,
		&errorMessage,
		&job.Attempts,
		&job.MaxAttempts,
		&idempotencyKey,
		&startedAt,
		&finishedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		return service.ProcessingJob{}, err
	}
	job.JobType = service.JobType(jobType)
	job.Status = service.JobStatus(status)
	if documentID.Valid {
		job.DocumentID = &documentID.String
	}
	if stage.Valid {
		value := service.JobStage(stage.String)
		job.CurrentStage = &value
	}
	if message.Valid {
		job.Message = &message.String
	}
	if errorCode.Valid {
		job.ErrorCode = &errorCode.String
	}
	if errorMessage.Valid {
		job.ErrorMessage = &errorMessage.String
	}
	if idempotencyKey.Valid {
		job.IdempotencyKey = &idempotencyKey.String
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		job.FinishedAt = &finishedAt.Time
	}
	return job, nil
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableJobStage(value *service.JobStage) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "duplicate key value") || strings.Contains(err.Error(), "SQLSTATE 23505")
}
