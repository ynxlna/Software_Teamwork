package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
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

func wrapPostgresError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ErrNotFound
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
