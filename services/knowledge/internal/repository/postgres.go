package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository/sqlc"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type PostgresRepository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool, queries: sqlc.New(pool)}
}

const parserConfigColumns = `id, name, backend, enabled, is_default, concurrency, supported_content_types, endpoint_url, default_parameters, created_at, updated_at, deleted_at`

func (r *PostgresRepository) ListParserConfigs(ctx context.Context, enabled *bool) ([]service.ParserConfig, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+parserConfigColumns+` FROM parser_configs WHERE deleted_at IS NULL AND ($1::boolean IS NULL OR enabled = $1) ORDER BY created_at DESC`, enabled)
	if err != nil {
		return nil, wrapPostgresError("list parser configs", err)
	}
	defer rows.Close()
	items := []service.ParserConfig{}
	for rows.Next() {
		config, err := scanParserConfig(rows)
		if err != nil {
			return nil, wrapPostgresError("scan parser config", err)
		}
		items = append(items, config)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapPostgresError("list parser configs", err)
	}
	return items, nil
}

func (r *PostgresRepository) GetParserConfig(ctx context.Context, id string) (service.ParserConfig, error) {
	return r.getParserConfig(ctx, r.pool, id, false)
}

type parserConfigQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r *PostgresRepository) getParserConfig(ctx context.Context, q parserConfigQuerier, id string, forUpdate bool) (service.ParserConfig, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	config, err := scanParserConfig(q.QueryRow(ctx, `SELECT `+parserConfigColumns+` FROM parser_configs WHERE id=$1 AND deleted_at IS NULL`+suffix, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ParserConfig{}, service.ErrNotFound
	}
	if err != nil {
		return service.ParserConfig{}, wrapPostgresError("get parser config", err)
	}
	return config, nil
}

func (r *PostgresRepository) CreateParserConfig(ctx context.Context, config service.ParserConfig, audit service.ParserConfigAudit) (service.ParserConfig, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return service.ParserConfig{}, wrapPostgresError("begin parser config create", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if config.IsDefault {
		if _, err = tx.Exec(ctx, `UPDATE parser_configs SET is_default=false, updated_at=$1 WHERE is_default AND deleted_at IS NULL`, config.UpdatedAt); err != nil {
			return service.ParserConfig{}, wrapPostgresError("clear parser default", err)
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO parser_configs (id,name,backend,enabled,is_default,concurrency,supported_content_types,endpoint_url,default_parameters,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, config.ID, config.Name, config.Backend, config.Enabled, config.IsDefault, config.Concurrency, config.SupportedContentTypes, config.EndpointURL, config.DefaultParameters, config.CreatedAt, config.UpdatedAt)
	if err != nil {
		return service.ParserConfig{}, wrapPostgresError("create parser config", err)
	}
	if err = insertParserAudit(ctx, tx, audit); err != nil {
		return service.ParserConfig{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return service.ParserConfig{}, wrapPostgresError("commit parser config create", err)
	}
	return config, nil
}

func (r *PostgresRepository) UpdateParserConfig(ctx context.Context, config service.ParserConfig, audit service.ParserConfigAudit) (service.ParserConfig, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return service.ParserConfig{}, wrapPostgresError("begin parser config update", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = r.getParserConfig(ctx, tx, config.ID, true); err != nil {
		return service.ParserConfig{}, err
	}
	if config.IsDefault {
		if _, err = tx.Exec(ctx, `UPDATE parser_configs SET is_default=false, updated_at=$1 WHERE id<>$2 AND is_default AND deleted_at IS NULL`, config.UpdatedAt, config.ID); err != nil {
			return service.ParserConfig{}, wrapPostgresError("clear parser default", err)
		}
	}
	_, err = tx.Exec(ctx, `UPDATE parser_configs SET name=$2,backend=$3,enabled=$4,is_default=$5,concurrency=$6,supported_content_types=$7,endpoint_url=$8,default_parameters=$9,updated_at=$10 WHERE id=$1 AND deleted_at IS NULL`, config.ID, config.Name, config.Backend, config.Enabled, config.IsDefault, config.Concurrency, config.SupportedContentTypes, config.EndpointURL, config.DefaultParameters, config.UpdatedAt)
	if err != nil {
		return service.ParserConfig{}, wrapPostgresError("update parser config", err)
	}
	if err = insertParserAudit(ctx, tx, audit); err != nil {
		return service.ParserConfig{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return service.ParserConfig{}, wrapPostgresError("commit parser config update", err)
	}
	return config, nil
}

func (r *PostgresRepository) SoftDeleteParserConfig(ctx context.Context, id string, deletedAt time.Time, audit service.ParserConfigAudit) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return wrapPostgresError("begin parser config delete", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	config, err := r.getParserConfig(ctx, tx, id, true)
	if err != nil {
		return err
	}
	if config.IsDefault {
		return service.ErrConflict
	}
	tag, err := tx.Exec(ctx, `UPDATE parser_configs SET enabled=false,deleted_at=$2,updated_at=$2 WHERE id=$1 AND deleted_at IS NULL`, id, deletedAt)
	if err != nil {
		return wrapPostgresError("delete parser config", err)
	}
	if tag.RowsAffected() == 0 {
		return service.ErrNotFound
	}
	if err = insertParserAudit(ctx, tx, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) GetEffectiveParserConfig(ctx context.Context, contentType string) (service.ParserConfig, error) {
	const query = `SELECT ` + parserConfigColumns + `
		FROM parser_configs
		WHERE enabled
			AND deleted_at IS NULL
			AND (
				$1=''
				OR cardinality(supported_content_types)=0
				OR $1=ANY(supported_content_types)
				OR split_part($1,'/',1)||'/*'=ANY(supported_content_types)
			)
		ORDER BY
			CASE
				WHEN $1='' THEN 0
				WHEN $1=ANY(supported_content_types) THEN 0
				WHEN split_part($1,'/',1)||'/*'=ANY(supported_content_types) THEN 1
				WHEN cardinality(supported_content_types)=0 THEN 2
				ELSE 3
			END,
			is_default DESC,
			created_at ASC
		LIMIT 1`
	config, err := scanParserConfig(r.pool.QueryRow(ctx, query, contentType))
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ParserConfig{}, service.ErrNotFound
	}
	if err != nil {
		return service.ParserConfig{}, wrapPostgresError("get effective parser config", err)
	}
	return config, nil
}

func insertParserAudit(ctx context.Context, tx pgx.Tx, audit service.ParserConfigAudit) error {
	_, err := tx.Exec(ctx, `INSERT INTO parser_config_audits (id,parser_config_id,actor_user_id,action,summary,created_at) VALUES ($1,$2,$3,$4,$5,$6)`, audit.ID, audit.ParserConfigID, audit.ActorUserID, audit.Action, audit.Summary, audit.CreatedAt)
	if err != nil {
		return wrapPostgresError("insert parser config audit", err)
	}
	return nil
}

type parserConfigScanner interface{ Scan(...any) error }

func scanParserConfig(row parserConfigScanner) (service.ParserConfig, error) {
	var c service.ParserConfig
	var backend string
	var endpoint pgtype.Text
	var deleted pgtype.Timestamptz
	err := row.Scan(&c.ID, &c.Name, &backend, &c.Enabled, &c.IsDefault, &c.Concurrency, &c.SupportedContentTypes, &endpoint, &c.DefaultParameters, &c.CreatedAt, &c.UpdatedAt, &deleted)
	c.Backend = service.ParserBackend(backend)
	c.EndpointURL = textPtr(endpoint)
	c.DeletedAt = timePtr(deleted)
	return c, err
}

func (r *PostgresRepository) CreateKnowledgeBase(ctx context.Context, input service.CreateKnowledgeBaseRecord) (service.KnowledgeBase, error) {
	row, err := r.queries.CreateKnowledgeBase(ctx, sqlc.CreateKnowledgeBaseParams{
		ID:                input.ID,
		Name:              input.Name,
		Description:       input.Description,
		DocType:           input.DocType,
		ChunkStrategy:     []byte(input.ChunkStrategy),
		RetrievalStrategy: []byte(input.RetrievalStrategy),
		CreatedBy:         input.CreatedBy,
		CreatedAt:         pgTime(input.CreatedAt),
		UpdatedAt:         pgTime(input.UpdatedAt),
	})
	if err != nil {
		return service.KnowledgeBase{}, wrapPostgresError("create knowledge base", err)
	}
	return knowledgeBaseFromCreateRow(row), nil
}

func (r *PostgresRepository) ListKnowledgeBases(ctx context.Context, scope service.AccessScope, page service.PageInput) (service.KnowledgeBaseList, error) {
	limit, offset := limitOffset(page)
	total, err := r.queries.CountKnowledgeBases(ctx, sqlc.CountKnowledgeBasesParams{
		CanReadAll: scope.CanReadAll,
		UserID:     scope.UserID,
	})
	if err != nil {
		return service.KnowledgeBaseList{}, wrapPostgresError("count knowledge bases", err)
	}
	rows, err := r.queries.ListKnowledgeBases(ctx, sqlc.ListKnowledgeBasesParams{
		CanReadAll:  scope.CanReadAll,
		UserID:      scope.UserID,
		LimitCount:  limit,
		OffsetCount: offset,
	})
	if err != nil {
		return service.KnowledgeBaseList{}, wrapPostgresError("list knowledge bases", err)
	}
	items := make([]service.KnowledgeBase, 0, len(rows))
	for _, row := range rows {
		items = append(items, knowledgeBaseFromListRow(row))
	}
	return service.KnowledgeBaseList{
		Items: items,
		Page: service.Page{
			Page:     page.Page,
			PageSize: page.PageSize,
			Total:    total,
		},
	}, nil
}

func (r *PostgresRepository) GetKnowledgeBase(ctx context.Context, id string, scope service.AccessScope) (service.KnowledgeBase, error) {
	row, err := r.queries.GetKnowledgeBase(ctx, sqlc.GetKnowledgeBaseParams{
		ID:         id,
		CanReadAll: scope.CanReadAll,
		UserID:     scope.UserID,
	})
	if err != nil {
		return service.KnowledgeBase{}, wrapPostgresError("get knowledge base", err)
	}
	return knowledgeBaseFromGetRow(row), nil
}

func (r *PostgresRepository) UpdateKnowledgeBase(ctx context.Context, input service.UpdateKnowledgeBaseRecord, scope service.AccessScope) (service.KnowledgeBase, error) {
	current, err := r.GetKnowledgeBase(ctx, input.ID, scope)
	if err != nil {
		return service.KnowledgeBase{}, err
	}
	if input.Name != nil {
		current.Name = *input.Name
	}
	if input.Description != nil {
		current.Description = *input.Description
	}
	if input.DocType != nil {
		current.DocType = *input.DocType
	}
	if input.ChunkStrategy != nil {
		current.ChunkStrategy = append([]byte(nil), (*input.ChunkStrategy)...)
	}
	if input.RetrievalStrategy != nil {
		current.RetrievalStrategy = append([]byte(nil), (*input.RetrievalStrategy)...)
	}

	params := sqlc.UpdateKnowledgeBaseParams{
		ID:                input.ID,
		Name:              current.Name,
		Description:       current.Description,
		DocType:           current.DocType,
		ChunkStrategy:     []byte(current.ChunkStrategy),
		RetrievalStrategy: []byte(current.RetrievalStrategy),
		UpdatedAt:         pgTime(input.UpdatedAt),
		CanReadAll:        scope.CanReadAll,
		UserID:            scope.UserID,
	}

	rowsAffected, err := r.queries.UpdateKnowledgeBase(ctx, params)
	if err != nil {
		return service.KnowledgeBase{}, wrapPostgresError("update knowledge base", err)
	}
	if rowsAffected == 0 {
		return service.KnowledgeBase{}, service.ErrNotFound
	}
	return r.GetKnowledgeBase(ctx, input.ID, scope)
}

func (r *PostgresRepository) SoftDeleteKnowledgeBase(ctx context.Context, id string, deletedAt time.Time, scope service.AccessScope) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return wrapPostgresError("begin knowledge base delete transaction", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := r.queries.WithTx(tx)
	rowsAffected, err := qtx.MarkKnowledgeBaseDeleted(ctx, sqlc.MarkKnowledgeBaseDeletedParams{
		ID:         id,
		DeletedAt:  pgTime(deletedAt),
		CanReadAll: scope.CanReadAll,
		UserID:     scope.UserID,
	})
	if err != nil {
		return wrapPostgresError("mark knowledge base deleted", err)
	}
	if rowsAffected == 0 {
		return service.ErrNotFound
	}
	if err := qtx.MarkDocumentsDeletedByKnowledgeBase(ctx, sqlc.MarkDocumentsDeletedByKnowledgeBaseParams{
		KnowledgeBaseID: id,
		DeletedAt:       pgTime(deletedAt),
	}); err != nil {
		return wrapPostgresError("mark documents deleted by knowledge base", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return wrapPostgresError("commit knowledge base delete transaction", err)
	}
	return nil
}

func (r *PostgresRepository) ListDocumentsByKnowledgeBase(ctx context.Context, knowledgeBaseID string, status *service.DocumentStatus, scope service.AccessScope, page service.PageInput) (service.DocumentList, error) {
	statusValue := ""
	if status != nil {
		statusValue = string(*status)
	}
	limit, offset := limitOffset(page)
	total, err := r.queries.CountDocumentsByKnowledgeBase(ctx, sqlc.CountDocumentsByKnowledgeBaseParams{
		KnowledgeBaseID: knowledgeBaseID,
		CanReadAll:      scope.CanReadAll,
		UserID:          scope.UserID,
		Status:          statusValue,
	})
	if err != nil {
		return service.DocumentList{}, wrapPostgresError("count documents by knowledge base", err)
	}
	rows, err := r.queries.ListDocumentsByKnowledgeBase(ctx, sqlc.ListDocumentsByKnowledgeBaseParams{
		KnowledgeBaseID: knowledgeBaseID,
		CanReadAll:      scope.CanReadAll,
		UserID:          scope.UserID,
		Status:          statusValue,
		LimitCount:      limit,
		OffsetCount:     offset,
	})
	if err != nil {
		return service.DocumentList{}, wrapPostgresError("list documents by knowledge base", err)
	}
	if total == 0 {
		if _, err := r.GetKnowledgeBase(ctx, knowledgeBaseID, scope); err != nil {
			return service.DocumentList{}, err
		}
	}
	items := make([]service.KnowledgeDocument, 0, len(rows))
	for _, row := range rows {
		items = append(items, documentFromListRow(row))
	}
	return service.DocumentList{
		Items: items,
		Page: service.Page{
			Page:     page.Page,
			PageSize: page.PageSize,
			Total:    total,
		},
	}, nil
}

func (r *PostgresRepository) GetDocument(ctx context.Context, id string, scope service.AccessScope) (service.KnowledgeDocument, error) {
	row, err := r.queries.GetDocument(ctx, sqlc.GetDocumentParams{
		ID:         id,
		CanReadAll: scope.CanReadAll,
		UserID:     scope.UserID,
	})
	if err != nil {
		return service.KnowledgeDocument{}, wrapPostgresError("get document", err)
	}
	return documentFromGetRow(row), nil
}

func (r *PostgresRepository) CreateDocumentWithJob(ctx context.Context, input service.CreateDocumentWithJobRecord, scope service.AccessScope) (service.KnowledgeDocument, service.ProcessingJob, error) {
	if _, err := r.GetKnowledgeBase(ctx, input.KnowledgeBaseID, scope); err != nil {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, err
	}

	tags, err := json.Marshal(input.Tags)
	if err != nil {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, fmt.Errorf("marshal document tags: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, wrapPostgresError("begin document upload transaction", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := r.queries.WithTx(tx)
	docRow, err := qtx.CreateDocument(ctx, sqlc.CreateDocumentParams{
		ID:              input.DocumentID,
		KnowledgeBaseID: input.KnowledgeBaseID,
		FileRef:         input.FileRef,
		Name:            input.Name,
		ContentType:     input.ContentType,
		SizeBytes:       pgInt8(input.SizeBytes),
		Status:          string(input.Status),
		Tags:            tags,
		CurrentJobID:    input.CurrentJobID,
		CreatedBy:       input.CreatedBy,
		CreatedAt:       pgTime(input.CreatedAt),
		UpdatedAt:       pgTime(input.UpdatedAt),
	})
	if err != nil {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, wrapPostgresError("create document", err)
	}

	jobRow, err := qtx.CreateProcessingJob(ctx, sqlc.CreateProcessingJobParams{
		ID:                   input.JobID,
		KnowledgeBaseID:      input.KnowledgeBaseID,
		DocumentID:           input.DocumentID,
		JobType:              input.JobType,
		Status:               input.JobStatus,
		CurrentStage:         input.JobStage,
		Message:              input.JobMessage,
		MaxAttempts:          input.MaxAttempts,
		ParserConfigID:       input.ParserConfigID,
		ParserConfigSnapshot: []byte(input.ParserConfigSnapshot),
		CreatedAt:            pgTime(input.CreatedAt),
		UpdatedAt:            pgTime(input.UpdatedAt),
	})
	if err != nil {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, wrapPostgresError("create processing job", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return service.KnowledgeDocument{}, service.ProcessingJob{}, wrapPostgresError("commit document upload transaction", err)
	}
	return documentFromCreateRow(docRow), processingJobFromRow(jobRow), nil
}

func (r *PostgresRepository) MarkDocumentJobFailed(ctx context.Context, documentID string, jobID string, expectedAttempts *int32, code string, message string, failedAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return wrapPostgresError("begin mark document job failed transaction", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// A document can be soft-deleted while a worker is running. The processing job
	// must still reach a terminal state so later redeliveries do not get stuck.
	jobRows, err := tx.Exec(ctx, `
UPDATE processing_jobs
SET status = 'failed',
    error_code = $3,
    error_message = $4,
    finished_at = $5,
    updated_at = $5
WHERE id = $1
  AND document_id = $2
  AND ($6::int4 IS NULL OR (attempts = $6 AND status = 'running'))`,
		jobID,
		documentID,
		code,
		message,
		pgTime(failedAt),
		pgInt4Ptr(expectedAttempts),
	)
	if err != nil {
		return wrapPostgresError("mark processing job failed", err)
	}
	if jobRows.RowsAffected() == 0 {
		if expectedAttempts != nil {
			return service.ErrConflict
		}
		return service.ErrNotFound
	}
	if _, err := tx.Exec(ctx, `
UPDATE knowledge_documents
SET status = 'failed',
    error_code = $2,
    error_message = $3,
    updated_at = $4
WHERE id = $1
  AND deleted_at IS NULL`,
		documentID,
		code,
		message,
		pgTime(failedAt),
	); err != nil {
		return wrapPostgresError("mark document failed", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return wrapPostgresError("commit mark document job failed transaction", err)
	}
	return nil
}

func (r *PostgresRepository) GetProcessingJob(ctx context.Context, id string) (service.ProcessingJob, error) {
	row, err := r.scanProcessingJob(ctx, r.pool.QueryRow(ctx, `
SELECT id, knowledge_base_id, document_id, job_type, status, current_stage, progress_percent,
       message, error_code, error_message, attempts, max_attempts, started_at, finished_at, created_at, updated_at
FROM processing_jobs
WHERE id = $1`, id))
	if err != nil {
		return service.ProcessingJob{}, wrapPostgresError("get processing job", err)
	}
	return processingJobFromRow(row), nil
}

func (r *PostgresRepository) UpdateJobState(ctx context.Context, id string, update service.JobStateUpdate) (service.ProcessingJob, error) {
	row, err := r.scanProcessingJob(ctx, r.pool.QueryRow(ctx, `
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
  AND ($12::int4 IS NULL OR (attempts = $12 AND status = 'running'))
RETURNING id, knowledge_base_id, document_id, job_type, status, current_stage, progress_percent,
          message, error_code, error_message, attempts, max_attempts, started_at, finished_at, created_at, updated_at`,
		id,
		update.Status,
		pgTextPtr(update.CurrentStage),
		update.ProgressPercent,
		pgTextPtr(update.Message),
		pgTextPtr(update.ErrorCode),
		pgTextPtr(update.ErrorMessage),
		pgInt4Ptr(update.Attempts),
		pgTimePtr(update.StartedAt),
		pgTimePtr(update.FinishedAt),
		pgTime(update.UpdatedAt),
		pgInt4Ptr(update.ExpectedAttempts),
	))
	if errors.Is(err, pgx.ErrNoRows) && update.ExpectedAttempts != nil {
		return service.ProcessingJob{}, service.ErrConflict
	}
	if err != nil {
		return service.ProcessingJob{}, wrapPostgresError("update processing job", err)
	}
	return processingJobFromRow(row), nil
}

func (r *PostgresRepository) ClaimProcessingJob(ctx context.Context, id string, update service.JobStateUpdate) (service.ProcessingJob, error) {
	row, err := r.scanProcessingJob(ctx, r.pool.QueryRow(ctx, `
UPDATE processing_jobs
SET status = $2,
    current_stage = $3,
    progress_percent = $4,
    message = $5,
    error_code = NULL,
    error_message = NULL,
    attempts = attempts + 1,
    started_at = COALESCE($6, started_at),
    finished_at = NULL,
    updated_at = $7
WHERE id = $1
  AND (
    status IN ('queued', 'failed')
    OR (
      status = 'running'
      AND $8::timestamptz IS NOT NULL
      AND updated_at < $8
    )
  )
  AND (max_attempts <= 0 OR attempts < max_attempts)
RETURNING id, knowledge_base_id, document_id, job_type, status, current_stage, progress_percent,
          message, error_code, error_message, attempts, max_attempts, started_at, finished_at, created_at, updated_at`,
		id,
		update.Status,
		pgTextPtr(update.CurrentStage),
		update.ProgressPercent,
		pgTextPtr(update.Message),
		pgTimePtr(update.StartedAt),
		pgTime(update.UpdatedAt),
		pgTimePtr(update.StaleRunningBefore),
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ProcessingJob{}, service.ErrConflict
	}
	if err != nil {
		return service.ProcessingJob{}, wrapPostgresError("claim processing job", err)
	}
	return processingJobFromRow(row), nil
}

func (r *PostgresRepository) UpdateDocumentProcessingState(ctx context.Context, id string, update service.DocumentStateUpdate) (service.KnowledgeDocument, error) {
	rows, err := r.pool.Exec(ctx, `
UPDATE knowledge_documents
SET status = $2,
    error_code = $3,
    error_message = $4,
    updated_at = $5
WHERE id = $1
  AND deleted_at IS NULL`,
		id,
		string(update.Status),
		pgTextPtr(update.ErrorCode),
		pgTextPtr(update.ErrorMessage),
		pgTime(update.UpdatedAt),
	)
	if err != nil {
		return service.KnowledgeDocument{}, wrapPostgresError("update document processing state", err)
	}
	if rows.RowsAffected() == 0 {
		return service.KnowledgeDocument{}, service.ErrNotFound
	}
	return r.GetDocument(ctx, id, service.AccessScope{CanReadAll: true})
}

func (r *PostgresRepository) CompleteIngestion(ctx context.Context, input service.CompleteIngestionRecord) (service.ProcessingJob, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return service.ProcessingJob{}, wrapPostgresError("begin complete ingestion transaction", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var attempts int32
	var status string
	if err := tx.QueryRow(ctx, `
SELECT attempts, status
FROM processing_jobs
WHERE id = $1
  AND document_id = $2
FOR UPDATE`,
		input.JobID,
		input.DocumentID,
	).Scan(&attempts, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ProcessingJob{}, service.ErrNotFound
		}
		return service.ProcessingJob{}, wrapPostgresError("lock processing job for completion", err)
	}
	if input.ExpectedAttempts != nil && (attempts != *input.ExpectedAttempts || status != service.JobStatusRunning) {
		return service.ProcessingJob{}, service.ErrConflict
	}

	if _, err := tx.Exec(ctx, `DELETE FROM document_chunks WHERE document_id = $1`, input.DocumentID); err != nil {
		return service.ProcessingJob{}, wrapPostgresError("delete old document chunks", err)
	}
	for _, chunk := range input.Chunks {
		metadata, err := json.Marshal(chunk.Metadata)
		if err != nil {
			return service.ProcessingJob{}, fmt.Errorf("marshal chunk metadata: %w", err)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO document_chunks (
  id, knowledge_base_id, document_id, chunk_index, section_path, content, token_count,
  chunk_type, qdrant_point_id, embedding_provider, embedding_model, embedding_dimension,
  metadata, created_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7,
  $8, $9, $10, $11, $12,
  $13, $14
)`,
			chunk.ID,
			chunk.KnowledgeBaseID,
			chunk.DocumentID,
			chunk.ChunkIndex,
			pgTextPtr(chunk.SectionPath),
			chunk.Content,
			pgInt4Ptr(chunk.TokenCount),
			pgTextPtr(chunk.ChunkType),
			pgTextPtr(chunk.QdrantPointID),
			pgTextPtr(chunk.EmbeddingProvider),
			pgTextPtr(chunk.EmbeddingModel),
			pgInt4Ptr(chunk.EmbeddingDimension),
			metadata,
			pgTime(chunk.CreatedAt),
		); err != nil {
			return service.ProcessingJob{}, wrapPostgresError("insert document chunk", err)
		}
	}
	docRows, err := tx.Exec(ctx, `
UPDATE knowledge_documents
SET status = 'ready',
    error_code = NULL,
    error_message = NULL,
    parser_backend = COALESCE($2, parser_backend),
    updated_at = $3
WHERE id = $1
  AND deleted_at IS NULL`,
		input.DocumentID,
		pgTextPtr(input.ParserBackend),
		pgTime(input.UpdatedAt),
	)
	if err != nil {
		return service.ProcessingJob{}, wrapPostgresError("mark document ready", err)
	}
	if docRows.RowsAffected() == 0 {
		return service.ProcessingJob{}, service.ErrNotFound
	}
	row, err := r.scanProcessingJob(ctx, tx.QueryRow(ctx, `
UPDATE processing_jobs
SET status = 'succeeded',
    current_stage = 'completed',
    progress_percent = 100,
    message = 'document ingestion completed',
    error_code = NULL,
    error_message = NULL,
    finished_at = $2,
    updated_at = $3
WHERE id = $1
  AND document_id = $4
  AND ($5::int4 IS NULL OR (attempts = $5 AND status = 'running'))
RETURNING id, knowledge_base_id, document_id, job_type, status, current_stage, progress_percent,
          message, error_code, error_message, attempts, max_attempts, started_at, finished_at, created_at, updated_at`,
		input.JobID,
		pgTime(input.FinishedAt),
		pgTime(input.UpdatedAt),
		input.DocumentID,
		pgInt4Ptr(input.ExpectedAttempts),
	))
	if errors.Is(err, pgx.ErrNoRows) && input.ExpectedAttempts != nil {
		return service.ProcessingJob{}, service.ErrConflict
	}
	if err != nil {
		return service.ProcessingJob{}, wrapPostgresError("mark processing job succeeded", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return service.ProcessingJob{}, wrapPostgresError("commit complete ingestion transaction", err)
	}
	return processingJobFromRow(row), nil
}

func (r *PostgresRepository) ListChunks(ctx context.Context, documentID string, scope service.AccessScope, page service.PageInput) (service.ChunkList, error) {
	limit, offset := limitOffset(page)
	var count int64
	if err := r.pool.QueryRow(ctx, `
SELECT COUNT(*)::bigint
FROM document_chunks c
JOIN knowledge_documents d ON d.id = c.document_id
JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
WHERE c.document_id = $1
  AND d.deleted_at IS NULL
  AND kb.deleted_at IS NULL
  AND ($2::boolean OR d.created_by = $3 OR kb.created_by = $3)`,
		documentID,
		scope.CanReadAll,
		scope.UserID,
	).Scan(&count); err != nil {
		return service.ChunkList{}, wrapPostgresError("count document chunks", err)
	}
	if count == 0 {
		if _, err := r.GetDocument(ctx, documentID, scope); err != nil {
			return service.ChunkList{}, err
		}
	}
	rows, err := r.pool.Query(ctx, `
SELECT c.id, c.knowledge_base_id, c.document_id, c.chunk_index, c.section_path, c.content,
       c.token_count, c.chunk_type, c.qdrant_point_id, c.embedding_provider, c.embedding_model,
       c.embedding_dimension, c.metadata, c.created_at
FROM document_chunks c
JOIN knowledge_documents d ON d.id = c.document_id
JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
WHERE c.document_id = $1
  AND d.deleted_at IS NULL
  AND kb.deleted_at IS NULL
  AND ($2::boolean OR d.created_by = $3 OR kb.created_by = $3)
ORDER BY c.chunk_index ASC, c.id ASC
LIMIT $4 OFFSET $5`,
		documentID,
		scope.CanReadAll,
		scope.UserID,
		limit,
		offset,
	)
	if err != nil {
		return service.ChunkList{}, wrapPostgresError("list document chunks", err)
	}
	defer rows.Close()

	items := []service.DocumentChunk{}
	for rows.Next() {
		chunk, err := scanDocumentChunk(rows)
		if err != nil {
			return service.ChunkList{}, wrapPostgresError("scan document chunk", err)
		}
		items = append(items, chunk)
	}
	if err := rows.Err(); err != nil {
		return service.ChunkList{}, wrapPostgresError("iterate document chunks", err)
	}
	return service.ChunkList{
		Items: items,
		Page: service.Page{
			Page:     page.Page,
			PageSize: page.PageSize,
			Total:    count,
		},
	}, nil
}

func limitOffset(page service.PageInput) (int32, int32) {
	limit := page.PageSize
	offset := (page.Page - 1) * page.PageSize
	if limit > math.MaxInt32 {
		limit = math.MaxInt32
	}
	if offset > math.MaxInt32 {
		offset = math.MaxInt32
	}
	return int32(limit), int32(offset)
}

func knowledgeBaseFromCreateRow(row sqlc.CreateKnowledgeBaseRow) service.KnowledgeBase {
	return service.KnowledgeBase{
		ID:                row.ID,
		Name:              row.Name,
		Description:       row.Description,
		DocType:           row.DocType,
		ChunkStrategy:     cloneJSON(row.ChunkStrategy, `{}`),
		RetrievalStrategy: cloneJSON(row.RetrievalStrategy, `{}`),
		DocumentCount:     row.DocumentCount,
		ChunkCount:        row.ChunkCount,
		CreatedBy:         row.CreatedBy,
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
		DeletedAt:         timePtr(row.DeletedAt),
	}
}

func knowledgeBaseFromGetRow(row sqlc.GetKnowledgeBaseRow) service.KnowledgeBase {
	return service.KnowledgeBase{
		ID:                row.ID,
		Name:              row.Name,
		Description:       row.Description,
		DocType:           row.DocType,
		ChunkStrategy:     cloneJSON(row.ChunkStrategy, `{}`),
		RetrievalStrategy: cloneJSON(row.RetrievalStrategy, `{}`),
		DocumentCount:     row.DocumentCount,
		ChunkCount:        row.ChunkCount,
		CreatedBy:         row.CreatedBy,
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
		DeletedAt:         timePtr(row.DeletedAt),
	}
}

func knowledgeBaseFromListRow(row sqlc.ListKnowledgeBasesRow) service.KnowledgeBase {
	return service.KnowledgeBase{
		ID:                row.ID,
		Name:              row.Name,
		Description:       row.Description,
		DocType:           row.DocType,
		ChunkStrategy:     cloneJSON(row.ChunkStrategy, `{}`),
		RetrievalStrategy: cloneJSON(row.RetrievalStrategy, `{}`),
		DocumentCount:     row.DocumentCount,
		ChunkCount:        row.ChunkCount,
		CreatedBy:         row.CreatedBy,
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
		DeletedAt:         timePtr(row.DeletedAt),
	}
}

func documentFromGetRow(row sqlc.GetDocumentRow) service.KnowledgeDocument {
	var tags []string
	if len(row.Tags) > 0 {
		_ = json.Unmarshal(row.Tags, &tags)
	}
	return service.KnowledgeDocument{
		ID:              row.ID,
		KnowledgeBaseID: row.KnowledgeBaseID,
		FileRef:         textPtr(row.FileRef),
		Name:            row.Name,
		ContentType:     textPtr(row.ContentType),
		SizeBytes:       int64Ptr(row.SizeBytes),
		Status:          service.DocumentStatus(row.Status),
		ErrorCode:       textPtr(row.ErrorCode),
		ErrorMessage:    textPtr(row.ErrorMessage),
		ChunkCount:      row.ChunkCount,
		Tags:            tags,
		ParserBackend:   textPtr(row.ParserBackend),
		CurrentJobID:    textPtr(row.CurrentJobID),
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
		DeletedAt:       timePtr(row.DeletedAt),
	}
}

func documentFromListRow(row sqlc.ListDocumentsByKnowledgeBaseRow) service.KnowledgeDocument {
	var tags []string
	if len(row.Tags) > 0 {
		_ = json.Unmarshal(row.Tags, &tags)
	}
	return service.KnowledgeDocument{
		ID:              row.ID,
		KnowledgeBaseID: row.KnowledgeBaseID,
		FileRef:         textPtr(row.FileRef),
		Name:            row.Name,
		ContentType:     textPtr(row.ContentType),
		SizeBytes:       int64Ptr(row.SizeBytes),
		Status:          service.DocumentStatus(row.Status),
		ErrorCode:       textPtr(row.ErrorCode),
		ErrorMessage:    textPtr(row.ErrorMessage),
		ChunkCount:      row.ChunkCount,
		Tags:            tags,
		ParserBackend:   textPtr(row.ParserBackend),
		CurrentJobID:    textPtr(row.CurrentJobID),
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
		DeletedAt:       timePtr(row.DeletedAt),
	}
}

func documentFromCreateRow(row sqlc.CreateDocumentRow) service.KnowledgeDocument {
	var tags []string
	if len(row.Tags) > 0 {
		_ = json.Unmarshal(row.Tags, &tags)
	}
	return service.KnowledgeDocument{
		ID:              row.ID,
		KnowledgeBaseID: row.KnowledgeBaseID,
		FileRef:         textPtr(row.FileRef),
		Name:            row.Name,
		ContentType:     textPtr(row.ContentType),
		SizeBytes:       int64Ptr(row.SizeBytes),
		Status:          service.DocumentStatus(row.Status),
		ErrorCode:       textPtr(row.ErrorCode),
		ErrorMessage:    textPtr(row.ErrorMessage),
		ChunkCount:      row.ChunkCount,
		Tags:            tags,
		ParserBackend:   textPtr(row.ParserBackend),
		CurrentJobID:    textPtr(row.CurrentJobID),
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
		DeletedAt:       timePtr(row.DeletedAt),
	}
}

func processingJobFromRow(row sqlc.ProcessingJob) service.ProcessingJob {
	return service.ProcessingJob{
		ID:                   row.ID,
		KnowledgeBaseID:      row.KnowledgeBaseID,
		DocumentID:           textPtr(row.DocumentID),
		JobType:              row.JobType,
		Status:               row.Status,
		CurrentStage:         textPtr(row.CurrentStage),
		ProgressPercent:      row.ProgressPercent,
		Message:              textPtr(row.Message),
		ErrorCode:            textPtr(row.ErrorCode),
		ErrorMessage:         textPtr(row.ErrorMessage),
		Attempts:             row.Attempts,
		MaxAttempts:          row.MaxAttempts,
		ParserConfigID:       textPtr(row.ParserConfigID),
		ParserConfigSnapshot: cloneJSON(row.ParserConfigSnapshot, "{}"),
		StartedAt:            timePtr(row.StartedAt),
		FinishedAt:           timePtr(row.FinishedAt),
		CreatedAt:            row.CreatedAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
	}
}

func (r *PostgresRepository) scanProcessingJob(ctx context.Context, row pgx.Row) (sqlc.ProcessingJob, error) {
	var job sqlc.ProcessingJob
	err := row.Scan(
		&job.ID,
		&job.KnowledgeBaseID,
		&job.DocumentID,
		&job.JobType,
		&job.Status,
		&job.CurrentStage,
		&job.ProgressPercent,
		&job.Message,
		&job.ErrorCode,
		&job.ErrorMessage,
		&job.Attempts,
		&job.MaxAttempts,
		&job.StartedAt,
		&job.FinishedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return sqlc.ProcessingJob{}, err
	}
	return job, ctx.Err()
}

func scanDocumentChunk(rows pgx.Rows) (service.DocumentChunk, error) {
	var chunk service.DocumentChunk
	var sectionPath pgtype.Text
	var tokenCount pgtype.Int4
	var chunkType pgtype.Text
	var qdrantPointID pgtype.Text
	var embeddingProvider pgtype.Text
	var embeddingModel pgtype.Text
	var embeddingDimension pgtype.Int4
	var metadata []byte
	var createdAt pgtype.Timestamptz
	if err := rows.Scan(
		&chunk.ID,
		&chunk.KnowledgeBaseID,
		&chunk.DocumentID,
		&chunk.ChunkIndex,
		&sectionPath,
		&chunk.Content,
		&tokenCount,
		&chunkType,
		&qdrantPointID,
		&embeddingProvider,
		&embeddingModel,
		&embeddingDimension,
		&metadata,
		&createdAt,
	); err != nil {
		return service.DocumentChunk{}, err
	}
	chunk.SectionPath = textPtr(sectionPath)
	chunk.TokenCount = int32Ptr(tokenCount)
	chunk.ChunkType = textPtr(chunkType)
	chunk.QdrantPointID = textPtr(qdrantPointID)
	chunk.EmbeddingProvider = textPtr(embeddingProvider)
	chunk.EmbeddingModel = textPtr(embeddingModel)
	chunk.EmbeddingDimension = int32Ptr(embeddingDimension)
	chunk.Metadata = map[string]any{}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &chunk.Metadata)
	}
	chunk.CreatedAt = createdAt.Time
	return chunk, nil
}

func wrapPostgresError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return service.ErrConflict
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func cloneJSON(value []byte, fallback string) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(fallback)
	}
	return append(json.RawMessage(nil), value...)
}

func textPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}

func int64Ptr(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	number := value.Int64
	return &number
}

func int32Ptr(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}
	number := value.Int32
	return &number
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	timestamp := value.Time
	return &timestamp
}

func pgTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func pgInt8(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value >= 0}
}

func pgTextPtr(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}

func pgInt4Ptr(value *int32) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *value, Valid: true}
}

func pgTimePtr(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}
