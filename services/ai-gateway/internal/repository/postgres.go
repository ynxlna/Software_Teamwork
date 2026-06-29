package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/ai-gateway/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
	db   dbtx
}

type dbtx interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func NewPostgres(ctx context.Context, databaseURL string) (*PostgresRepository, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("AI_GATEWAY_DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("AI_GATEWAY_DATABASE_URL is invalid")
	}
	config.MaxConns = 10
	config.MinConns = 1
	config.MaxConnLifetime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return NewPostgresRepository(pool), nil
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool, db: pool}
}

func (r *PostgresRepository) Close() {
	r.pool.Close()
}

func (r *PostgresRepository) CheckReady(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *PostgresRepository) ListModelProfiles(ctx context.Context, filter service.ListModelProfilesFilter) ([]service.ModelProfile, error) {
	where := []string{"deleted_at IS NULL"}
	args := []any{}
	if filter.Purpose != nil {
		args = append(args, string(*filter.Purpose))
		where = append(where, fmt.Sprintf("purpose = $%d", len(args)))
	}
	if filter.Enabled != nil {
		args = append(args, *filter.Enabled)
		where = append(where, fmt.Sprintf("enabled = $%d", len(args)))
	}
	query := fmt.Sprintf(`
		SELECT
			id, name, purpose, provider, base_url, model, enabled, is_default,
			timeout_ms, api_key_configured, supports_streaming, dimensions, top_n,
			default_parameters_json, COALESCE(credential_id, ''), COALESCE(created_by_user_id, ''),
			COALESCE(updated_by_user_id, ''), created_at, updated_at, deleted_at
		FROM model_profiles
		WHERE %s
		ORDER BY purpose, is_default DESC, created_at DESC`, strings.Join(where, " AND "))
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list model profiles: %w", err)
	}
	defer rows.Close()
	items := []service.ModelProfile{}
	for rows.Next() {
		profile, err := scanModelProfile(rows)
		if err != nil {
			return nil, fmt.Errorf("scan model profile: %w", err)
		}
		items = append(items, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model profiles: %w", err)
	}
	return items, nil
}

func (r *PostgresRepository) GetModelProfile(ctx context.Context, id string) (service.ModelProfile, error) {
	row := r.db.QueryRow(ctx, `
		SELECT
			id, name, purpose, provider, base_url, model, enabled, is_default,
			timeout_ms, api_key_configured, supports_streaming, dimensions, top_n,
			default_parameters_json, COALESCE(credential_id, ''), COALESCE(created_by_user_id, ''),
			COALESCE(updated_by_user_id, ''), created_at, updated_at, deleted_at
		FROM model_profiles
		WHERE id = $1 AND deleted_at IS NULL`, id)
	profile, err := scanModelProfile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ModelProfile{}, service.ErrNotFound
		}
		return service.ModelProfile{}, fmt.Errorf("get model profile: %w", err)
	}
	return profile, nil
}

func (r *PostgresRepository) GetDefaultModelProfile(ctx context.Context, purpose service.Purpose) (service.ModelProfile, error) {
	row := r.db.QueryRow(ctx, `
		SELECT
			id, name, purpose, provider, base_url, model, enabled, is_default,
			timeout_ms, api_key_configured, supports_streaming, dimensions, top_n,
			default_parameters_json, COALESCE(credential_id, ''), COALESCE(created_by_user_id, ''),
			COALESCE(updated_by_user_id, ''), created_at, updated_at, deleted_at
		FROM model_profiles
		WHERE purpose = $1 AND enabled = true AND is_default = true AND deleted_at IS NULL
		ORDER BY updated_at DESC
		LIMIT 1`, string(purpose))
	profile, err := scanModelProfile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ModelProfile{}, service.ErrNotFound
		}
		return service.ModelProfile{}, fmt.Errorf("get default model profile: %w", err)
	}
	return profile, nil
}

func (r *PostgresRepository) GetActiveCredential(ctx context.Context, profileID string) (service.ProviderCredential, error) {
	row := r.db.QueryRow(ctx, `
		SELECT
			id, profile_id, storage_mode, ciphertext, nonce, encryption_key_version,
			fingerprint_sha256, COALESCE(key_last4, ''), status, COALESCE(created_by_user_id, ''),
			created_at, rotated_at, disabled_at, deleted_at
		FROM provider_credentials
		WHERE profile_id = $1 AND status = 'active' AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1`, profileID)
	credential, err := scanProviderCredential(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ProviderCredential{}, service.ErrNotFound
		}
		return service.ProviderCredential{}, fmt.Errorf("get active provider credential: %w", err)
	}
	return credential, nil
}

func (r *PostgresRepository) CreateModelProfile(ctx context.Context, profile service.ModelProfile, credential service.ProviderCredential, revision service.ModelProfileRevision) (service.ModelProfile, error) {
	var created service.ModelProfile
	err := r.withTx(ctx, func(tx *PostgresRepository) error {
		if profile.Enabled && profile.IsDefault {
			if err := tx.unsetDefaultProfiles(ctx, profile.Purpose, profile.ID); err != nil {
				return err
			}
		}
		value, err := tx.insertProfile(ctx, profile)
		if err != nil {
			return err
		}
		if err := tx.insertCredential(ctx, credential); err != nil {
			return err
		}
		revision.ProfileID = value.ID
		if err := tx.insertRevision(ctx, revision); err != nil {
			return err
		}
		created = value
		return nil
	})
	return created, err
}

func (r *PostgresRepository) UpdateModelProfile(ctx context.Context, profile service.ModelProfile, credential *service.ProviderCredential, revision service.ModelProfileRevision) (service.ModelProfile, error) {
	var updated service.ModelProfile
	err := r.withTx(ctx, func(tx *PostgresRepository) error {
		if profile.Enabled && profile.IsDefault {
			if err := tx.unsetDefaultProfiles(ctx, profile.Purpose, profile.ID); err != nil {
				return err
			}
		}
		if credential != nil {
			if err := tx.rotateActiveCredential(ctx, profile.ID, profile.UpdatedAt); err != nil {
				return err
			}
			if err := tx.insertCredential(ctx, *credential); err != nil {
				return err
			}
		}
		value, err := tx.updateProfile(ctx, profile)
		if err != nil {
			return err
		}
		if err := tx.insertRevision(ctx, revision); err != nil {
			return err
		}
		updated = value
		return nil
	})
	return updated, err
}

func (r *PostgresRepository) SoftDeleteModelProfile(ctx context.Context, id string, deletedAt time.Time, revision service.ModelProfileRevision) error {
	return r.withTx(ctx, func(tx *PostgresRepository) error {
		tag, err := tx.db.Exec(ctx, `
			UPDATE model_profiles
			SET deleted_at = $2, enabled = false, is_default = false, updated_at = $2
			WHERE id = $1 AND deleted_at IS NULL`, id, deletedAt)
		if err != nil {
			return fmt.Errorf("delete model profile: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return service.ErrNotFound
		}
		if _, err := tx.db.Exec(ctx, `
			UPDATE provider_credentials
			SET status = 'disabled', disabled_at = $2
			WHERE profile_id = $1 AND status = 'active'`, id, deletedAt); err != nil {
			return fmt.Errorf("disable provider credential: %w", err)
		}
		return tx.insertRevision(ctx, revision)
	})
}

func (r *PostgresRepository) RecordProviderInvocation(ctx context.Context, invocation service.ProviderInvocation, attempts []service.ProviderInvocationAttempt) error {
	return r.withTx(ctx, func(tx *PostgresRepository) error {
		if err := tx.insertProviderInvocation(ctx, invocation); err != nil {
			return err
		}
		for _, attempt := range attempts {
			if _, err := tx.db.Exec(ctx, `
				INSERT INTO provider_invocation_attempts (
					id, invocation_id, attempt_no, provider, base_url_host, model,
					status, provider_status_code, duration_ms, error_code,
					error_message, started_at, finished_at
				)
				VALUES (
					$1, $2, $3, $4, NULLIF($5, ''), $6,
					$7, $8, $9, NULLIF($10, ''),
					NULLIF($11, ''), $12, $13
				)`,
				attempt.ID, attempt.InvocationID, attempt.AttemptNo, string(attempt.Provider),
				attempt.BaseURLHost, attempt.Model, string(attempt.Status),
				attempt.ProviderStatusCode, attempt.DurationMS, attempt.ErrorCode,
				attempt.ErrorMessage, attempt.StartedAt, attempt.FinishedAt); err != nil {
				return fmt.Errorf("insert provider invocation attempt: %w", err)
			}
		}
		return tx.upsertUsageAggregate(ctx, invocation)
	})
}

func (r *PostgresRepository) insertProviderInvocation(ctx context.Context, invocation service.ProviderInvocation) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO provider_invocations (
			id, request_id, caller_service, external_user_id, operation, profile_id,
			provider, model, stream, status, provider_status_code, prompt_tokens,
			completion_tokens, total_tokens, input_count, embedding_dimensions,
			rerank_top_n, duration_ms, attempt_count, normalized_error_code,
			normalized_error_type, error_message, created_at, finished_at
		)
		VALUES (
			$1, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16,
			$17, $18, $19, NULLIF($20, ''),
			NULLIF($21, ''), NULLIF($22, ''), $23, $24
		)`,
		invocation.ID, invocation.RequestID, invocation.CallerService, invocation.ExternalUserID,
		invocation.Operation, invocation.ProfileID, string(invocation.Provider), invocation.Model,
		invocation.Stream, string(invocation.Status), nullableInt(invocation.ProviderStatusCode),
		nullableInt(invocation.PromptTokens), nullableInt(invocation.CompletionTokens),
		nullableInt(invocation.TotalTokens), nullableInt(invocation.InputCount),
		nullableInt(invocation.EmbeddingDimensions), nullableInt(invocation.RerankTopN),
		invocation.DurationMS, invocation.AttemptCount, invocation.NormalizedErrorCode,
		invocation.NormalizedErrorType, invocation.ErrorMessage, invocation.CreatedAt, invocation.FinishedAt)
	if err != nil {
		return fmt.Errorf("insert provider invocation: %w", err)
	}
	return nil
}

func (r *PostgresRepository) upsertUsageAggregate(ctx context.Context, invocation service.ProviderInvocation) error {
	bucket := invocation.CreatedAt.UTC().Truncate(time.Hour)
	successCount := 0
	failureCount := 0
	if invocation.Status == service.InvocationSucceeded {
		successCount = 1
	} else {
		failureCount = 1
	}
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
		INSERT INTO model_usage_aggregates (
			id, bucket_start_at, bucket_granularity, caller_service, profile_id, operation,
			request_count, success_count, failure_count, prompt_tokens, completion_tokens,
			total_tokens, total_duration_ms, created_at, updated_at
		)
		VALUES ($1, $2, 'hour', $3, $4, $5, 1, $6, $7, $8, $9, $10, $11, $12, $12)
		ON CONFLICT (bucket_start_at, bucket_granularity, caller_service, profile_id, operation)
		DO UPDATE SET
			request_count = model_usage_aggregates.request_count + EXCLUDED.request_count,
			success_count = model_usage_aggregates.success_count + EXCLUDED.success_count,
			failure_count = model_usage_aggregates.failure_count + EXCLUDED.failure_count,
			prompt_tokens = model_usage_aggregates.prompt_tokens + EXCLUDED.prompt_tokens,
			completion_tokens = model_usage_aggregates.completion_tokens + EXCLUDED.completion_tokens,
			total_tokens = model_usage_aggregates.total_tokens + EXCLUDED.total_tokens,
			total_duration_ms = model_usage_aggregates.total_duration_ms + EXCLUDED.total_duration_ms,
			updated_at = EXCLUDED.updated_at`,
		usageAggregateID(bucket, invocation.CallerService, invocation.ProfileID, invocation.Operation),
		bucket, invocation.CallerService, invocation.ProfileID, invocation.Operation,
		successCount, failureCount, intValue(invocation.PromptTokens),
		intValue(invocation.CompletionTokens), intValue(invocation.TotalTokens), invocation.DurationMS, now)
	if err != nil {
		return fmt.Errorf("upsert model usage aggregate: %w", err)
	}
	return nil
}

func (r *PostgresRepository) withTx(ctx context.Context, fn func(*PostgresRepository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ai-gateway transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	txRepo := &PostgresRepository{pool: r.pool, db: tx}
	if err := fn(txRepo); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit ai-gateway transaction: %w", err)
	}
	return nil
}

func (r *PostgresRepository) unsetDefaultProfiles(ctx context.Context, purpose service.Purpose, excludeID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE model_profiles
		SET is_default = false, updated_at = now()
		WHERE purpose = $1 AND id <> $2 AND enabled = true AND is_default = true AND deleted_at IS NULL`,
		string(purpose), excludeID)
	if err != nil {
		return fmt.Errorf("unset default model profiles: %w", err)
	}
	return nil
}

func (r *PostgresRepository) insertProfile(ctx context.Context, profile service.ModelProfile) (service.ModelProfile, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO model_profiles (
			id, name, purpose, provider, base_url, model, enabled, is_default,
			timeout_ms, api_key_configured, supports_streaming, dimensions, top_n,
			default_parameters_json, credential_id, created_by_user_id, updated_by_user_id,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, NULLIF($15, ''), NULLIF($16, ''), NULLIF($17, ''), $18, $19)
		RETURNING
			id, name, purpose, provider, base_url, model, enabled, is_default,
			timeout_ms, api_key_configured, supports_streaming, dimensions, top_n,
			default_parameters_json, COALESCE(credential_id, ''), COALESCE(created_by_user_id, ''),
			COALESCE(updated_by_user_id, ''), created_at, updated_at, deleted_at`,
		profile.ID, profile.Name, string(profile.Purpose), string(profile.Provider), profile.BaseURL, profile.Model,
		profile.Enabled, profile.IsDefault, profile.TimeoutMS, profile.APIKeyConfigured, profile.SupportsStreaming,
		profile.Dimensions, profile.TopN, rawJSON(profile.DefaultParameters), profile.CredentialID,
		profile.CreatedByUserID, profile.UpdatedByUserID, profile.CreatedAt, profile.UpdatedAt)
	value, err := scanModelProfile(row)
	if err != nil {
		return service.ModelProfile{}, mapPostgresError("insert model profile", err)
	}
	return value, nil
}

func (r *PostgresRepository) updateProfile(ctx context.Context, profile service.ModelProfile) (service.ModelProfile, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE model_profiles
		SET
			name = $2,
			provider = $3,
			base_url = $4,
			model = $5,
			enabled = $6,
			is_default = $7,
			timeout_ms = $8,
			api_key_configured = $9,
			supports_streaming = $10,
			dimensions = $11,
			top_n = $12,
			default_parameters_json = $13::jsonb,
			credential_id = NULLIF($14, ''),
			updated_by_user_id = NULLIF($15, ''),
			updated_at = $16
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING
			id, name, purpose, provider, base_url, model, enabled, is_default,
			timeout_ms, api_key_configured, supports_streaming, dimensions, top_n,
			default_parameters_json, COALESCE(credential_id, ''), COALESCE(created_by_user_id, ''),
			COALESCE(updated_by_user_id, ''), created_at, updated_at, deleted_at`,
		profile.ID, profile.Name, string(profile.Provider), profile.BaseURL, profile.Model, profile.Enabled,
		profile.IsDefault, profile.TimeoutMS, profile.APIKeyConfigured, profile.SupportsStreaming,
		profile.Dimensions, profile.TopN, rawJSON(profile.DefaultParameters), profile.CredentialID,
		profile.UpdatedByUserID, profile.UpdatedAt)
	value, err := scanModelProfile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ModelProfile{}, service.ErrNotFound
		}
		return service.ModelProfile{}, mapPostgresError("update model profile", err)
	}
	return value, nil
}

func (r *PostgresRepository) insertCredential(ctx context.Context, credential service.ProviderCredential) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO provider_credentials (
			id, profile_id, storage_mode, ciphertext, nonce, encryption_key_version,
			fingerprint_sha256, key_last4, status, created_by_user_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, NULLIF($10, ''), $11)`,
		credential.ID, credential.ProfileID, credential.StorageMode, credential.Ciphertext, credential.Nonce,
		credential.EncryptionKeyVersion, credential.FingerprintSHA256, credential.KeyLast4,
		string(credential.Status), credential.CreatedByUserID, credential.CreatedAt)
	if err != nil {
		return mapPostgresError("insert provider credential", err)
	}
	return nil
}

func (r *PostgresRepository) rotateActiveCredential(ctx context.Context, profileID string, rotatedAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE provider_credentials
		SET status = 'rotated', rotated_at = $2
		WHERE profile_id = $1 AND status = 'active'`, profileID, rotatedAt)
	if err != nil {
		return fmt.Errorf("rotate provider credential: %w", err)
	}
	return nil
}

func (r *PostgresRepository) insertRevision(ctx context.Context, revision service.ModelProfileRevision) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO model_profile_revisions (
			id, profile_id, revision_no, change_type, changed_fields_json,
			before_snapshot_json, after_snapshot_json, changed_by_user_id,
			caller_service, request_id, created_at
		)
		VALUES (
			$1, $2,
			COALESCE((SELECT max(revision_no) + 1 FROM model_profile_revisions WHERE profile_id = $2), 1),
			$3, NULLIF($4, '')::jsonb, NULLIF($5, '')::jsonb, NULLIF($6, '')::jsonb,
			NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), $10
		)`,
		revision.ID, revision.ProfileID, string(revision.ChangeType), rawJSON(revision.ChangedFieldsJSON),
		rawJSON(revision.BeforeSnapshotJSON), rawJSON(revision.AfterSnapshotJSON), revision.ChangedByUserID,
		revision.CallerService, revision.RequestID, revision.CreatedAt)
	if err != nil {
		return mapPostgresError("insert model profile revision", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanModelProfile(row scanner) (service.ModelProfile, error) {
	var value service.ModelProfile
	var purpose, provider string
	var dimensions, topN pgtype.Int4
	var deletedAt pgtype.Timestamptz
	if err := row.Scan(
		&value.ID, &value.Name, &purpose, &provider, &value.BaseURL, &value.Model,
		&value.Enabled, &value.IsDefault, &value.TimeoutMS, &value.APIKeyConfigured,
		&value.SupportsStreaming, &dimensions, &topN, &value.DefaultParameters,
		&value.CredentialID, &value.CreatedByUserID, &value.UpdatedByUserID,
		&value.CreatedAt, &value.UpdatedAt, &deletedAt,
	); err != nil {
		return service.ModelProfile{}, err
	}
	value.Purpose = service.Purpose(purpose)
	value.Provider = service.Provider(provider)
	if dimensions.Valid {
		dim := int(dimensions.Int32)
		value.Dimensions = &dim
	}
	if topN.Valid {
		n := int(topN.Int32)
		value.TopN = &n
	}
	if deletedAt.Valid {
		value.DeletedAt = &deletedAt.Time
	}
	return value, nil
}

func scanProviderCredential(row scanner) (service.ProviderCredential, error) {
	var value service.ProviderCredential
	var status string
	var rotatedAt, disabledAt, deletedAt pgtype.Timestamptz
	if err := row.Scan(
		&value.ID, &value.ProfileID, &value.StorageMode, &value.Ciphertext, &value.Nonce,
		&value.EncryptionKeyVersion, &value.FingerprintSHA256, &value.KeyLast4, &status,
		&value.CreatedByUserID, &value.CreatedAt, &rotatedAt, &disabledAt, &deletedAt,
	); err != nil {
		return service.ProviderCredential{}, err
	}
	value.Status = service.CredentialStatus(status)
	if rotatedAt.Valid {
		value.RotatedAt = &rotatedAt.Time
	}
	if disabledAt.Valid {
		value.DisabledAt = &disabledAt.Time
	}
	if deletedAt.Valid {
		value.DeletedAt = &deletedAt.Time
	}
	return value, nil
}

func mapPostgresError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ErrNotFound
	}
	if isUniqueViolation(err) || isCheckViolation(err) {
		return service.ErrConflict
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func usageAggregateID(bucket time.Time, callerService, profileID, operation string) string {
	sum := sha256.Sum256([]byte(bucket.Format(time.RFC3339) + "\x00" + callerService + "\x00" + profileID + "\x00" + operation))
	return "usage_" + hex.EncodeToString(sum[:12])
}

func rawJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	return string(raw)
}
