package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	defaultSessionTTL = 24 * time.Hour

	reasonInvalidCredentials = "invalid_credentials"
	reasonAccountUnavailable = "account_unavailable"
	reasonDefaultRole        = "default_role"
	reasonUserLogout         = "user_logout"
)

type Clock func() time.Time

type IDGenerator func(prefix string) string

type TokenGenerator func() (string, error)

type Option func(*Service)

type Service struct {
	repo                Repository
	now                 Clock
	newID               IDGenerator
	newAccessToken      TokenGenerator
	tokenHashSecret     []byte
	tokenHashKeyVersion string
	sessionTTL          time.Duration
	defaultRoleCode     string
	logger              *slog.Logger
}

func New(repo Repository, opts ...Option) *Service {
	s := &Service{
		repo: repo,
		now: func() time.Time {
			return time.Now().UTC()
		},
		newID:               newID,
		newAccessToken:      newOpaqueAccessToken,
		tokenHashSecret:     []byte("auth-local-development-token-hash-secret"),
		tokenHashKeyVersion: TokenHashKeyVersionV1,
		sessionTTL:          defaultSessionTTL,
		defaultRoleCode:     DefaultRoleCode,
		logger:              slog.Default(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func WithClock(now Clock) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

func WithIDGenerator(newID IDGenerator) Option {
	return func(s *Service) {
		if newID != nil {
			s.newID = newID
		}
	}
}

func WithTokenGenerator(newToken TokenGenerator) Option {
	return func(s *Service) {
		if newToken != nil {
			s.newAccessToken = newToken
		}
	}
}

func WithTokenHashSecret(secret []byte) Option {
	return func(s *Service) {
		s.tokenHashSecret = append([]byte(nil), secret...)
	}
}

func WithTokenHashKeyVersion(version string) Option {
	return func(s *Service) {
		if trimmed := strings.TrimSpace(version); trimmed != "" {
			s.tokenHashKeyVersion = trimmed
		}
	}
}

func WithSessionTTL(ttl time.Duration) Option {
	return func(s *Service) {
		if ttl > 0 {
			s.sessionTTL = ttl
		}
	}
}

func WithDefaultRoleCode(roleCode string) Option {
	return func(s *Service) {
		if trimmed := strings.TrimSpace(roleCode); trimmed != "" {
			s.defaultRoleCode = trimmed
		}
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func (s *Service) CreateUser(ctx context.Context, reqCtx RequestContext, input CreateUserInput) (SessionResponse, error) {
	if err := s.validateReady(); err != nil {
		return SessionResponse{}, err
	}
	if err := validateInternalCaller(reqCtx); err != nil {
		return SessionResponse{}, err
	}

	username, password, err := normalizeCredentials(input.Username, input.Password)
	if err != nil {
		return SessionResponse{}, err
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return SessionResponse{}, DependencyError("credential hashing failed", err)
	}

	now := s.now()
	user, err := s.repo.CreateUserWithCredential(ctx, CreateUserParams{
		ID:                        s.newID("usr"),
		Username:                  username,
		DisplayName:               username,
		Status:                    UserStatusActive,
		CreatedAt:                 now,
		PasswordCredentialID:      s.newID("cred"),
		PasswordHash:              passwordHash,
		PasswordHashAlg:           PasswordHashAlg,
		PasswordHashParamsVersion: PasswordHashParamsVersion,
		PasswordHashParamsJSON:    passwordHashParamsJSON(),
		DefaultRoleCode:           s.defaultRoleCode,
		RoleAssignmentID:          s.newID("urole"),
		AssignedBy:                callerService(reqCtx),
	})
	if err != nil {
		return SessionResponse{}, mapRepositoryError(err, "user not found")
	}

	userSummary := summaryFromRecord(user)
	s.recordSecurityEventBestEffort(ctx, reqCtx, SecurityEventParams{
		EventType:        SecurityEventUserCreated,
		UserID:           stringPtr(userSummary.ID),
		UsernameSnapshot: stringPtr(userSummary.Username),
		Status:           SecurityEventStatusSuccess,
	})
	if s.defaultRoleCode != "" {
		s.recordSecurityEventBestEffort(ctx, reqCtx, SecurityEventParams{
			EventType:        SecurityEventRoleAssigned,
			UserID:           stringPtr(userSummary.ID),
			UsernameSnapshot: stringPtr(userSummary.Username),
			Status:           SecurityEventStatusSuccess,
			ReasonCode:       stringPtr(reasonDefaultRole),
			MetadataJSON:     fmt.Sprintf(`{"role":%q}`, s.defaultRoleCode),
		})
	}

	session, err := s.createSessionForUser(ctx, reqCtx, userSummary)
	if err != nil {
		return SessionResponse{}, err
	}
	return SessionResponse{User: userSummary, Session: session}, nil
}

func (s *Service) CreateSession(ctx context.Context, reqCtx RequestContext, input CreateSessionInput) (SessionResponse, error) {
	if err := s.validateReady(); err != nil {
		return SessionResponse{}, err
	}
	if err := validateInternalCaller(reqCtx); err != nil {
		return SessionResponse{}, err
	}

	username, password, err := normalizeCredentials(input.Username, input.Password)
	if err != nil {
		return SessionResponse{}, err
	}

	user, err := s.repo.FindUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			if eventErr := s.recordSessionFailure(ctx, reqCtx, nil, username, reasonInvalidCredentials); eventErr != nil {
				return SessionResponse{}, eventErr
			}
			return SessionResponse{}, invalidCredentialsError()
		}
		return SessionResponse{}, mapRepositoryError(err, "user not found")
	}

	if !userCanCreateSession(user.User, s.now()) {
		if eventErr := s.recordSessionFailure(ctx, reqCtx, &user.User, username, reasonAccountUnavailable); eventErr != nil {
			return SessionResponse{}, eventErr
		}
		return SessionResponse{}, invalidCredentialsError()
	}

	credential, err := s.repo.FindCredentialByUserID(ctx, user.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			if eventErr := s.recordSessionFailure(ctx, reqCtx, &user.User, username, reasonInvalidCredentials); eventErr != nil {
				return SessionResponse{}, eventErr
			}
			return SessionResponse{}, invalidCredentialsError()
		}
		return SessionResponse{}, mapRepositoryError(err, "credential not found")
	}
	if credential.PasswordHashAlg != PasswordHashAlg || credential.PasswordHashParamsVersion != PasswordHashParamsVersion {
		return SessionResponse{}, DependencyError("credential parameters are unsupported", nil)
	}
	ok, err := verifyPassword(password, credential.PasswordHash)
	if err != nil {
		return SessionResponse{}, DependencyError("credential verification failed", err)
	}
	if !ok {
		if eventErr := s.recordSessionFailure(ctx, reqCtx, &user.User, username, reasonInvalidCredentials); eventErr != nil {
			return SessionResponse{}, eventErr
		}
		return SessionResponse{}, invalidCredentialsError()
	}

	userSummary := summaryFromRecord(user)
	session, err := s.createSessionForUser(ctx, reqCtx, userSummary)
	if err != nil {
		return SessionResponse{}, err
	}
	return SessionResponse{User: userSummary, Session: session}, nil
}

func (s *Service) GetUser(ctx context.Context, reqCtx RequestContext, userID string) (UserRecord, error) {
	if err := s.validateReady(); err != nil {
		return UserRecord{}, err
	}
	if err := validateInternalCaller(reqCtx); err != nil {
		return UserRecord{}, err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return UserRecord{}, ValidationError("request validation failed", map[string]string{"userId": "is required"})
	}
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return UserRecord{}, mapRepositoryError(err, "user not found")
	}
	return user, nil
}

func (s *Service) GetUserPermissions(ctx context.Context, reqCtx RequestContext, userID string) (UserPermissions, error) {
	user, err := s.GetUser(ctx, reqCtx, userID)
	if err != nil {
		return UserPermissions{}, err
	}
	return UserPermissions{
		UserID:      user.ID,
		Roles:       append([]string(nil), user.Roles...),
		Permissions: append([]string(nil), user.Permissions...),
		UpdatedAt:   user.UpdatedAt,
	}, nil
}

func (s *Service) GetSession(ctx context.Context, reqCtx RequestContext, sessionID string) (SessionIdentity, error) {
	if err := s.validateReady(); err != nil {
		return SessionIdentity{}, err
	}
	if err := validateInternalCaller(reqCtx); err != nil {
		return SessionIdentity{}, err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return SessionIdentity{}, ValidationError("request validation failed", map[string]string{"sessionId": "is required"})
	}
	identity, err := s.repo.FindSessionByID(ctx, sessionID)
	if err != nil {
		return SessionIdentity{}, mapRepositoryError(err, "session not found")
	}
	return identity, nil
}

func (s *Service) GetSessionByAccessToken(ctx context.Context, reqCtx RequestContext, accessToken string) (SessionIdentity, error) {
	if err := s.validateReady(); err != nil {
		return SessionIdentity{}, err
	}
	if err := validateInternalCaller(reqCtx); err != nil {
		return SessionIdentity{}, err
	}
	tokenHash, err := hashAccessToken(accessToken, s.tokenHashSecret, s.tokenHashKeyVersion)
	if err != nil {
		return SessionIdentity{}, UnauthorizedError()
	}
	identity, err := s.repo.FindActiveSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SessionIdentity{}, UnauthorizedError()
		}
		return SessionIdentity{}, mapRepositoryError(err, "session not found")
	}
	return identity, nil
}

func (s *Service) RevokeSession(ctx context.Context, reqCtx RequestContext, sessionID string, reason string) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if err := validateInternalCaller(reqCtx); err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ValidationError("request validation failed", map[string]string{"sessionId": "is required"})
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = reasonUserLogout
	}

	session, err := s.repo.RevokeSession(ctx, RevokeSessionParams{
		SessionID: sessionID,
		Reason:    reason,
		RequestID: optionalString(reqCtx.RequestID),
		RevokedAt: s.now(),
	})
	if err != nil {
		return mapRepositoryError(err, "session not found")
	}
	s.recordSecurityEventBestEffort(ctx, reqCtx, SecurityEventParams{
		EventType:    SecurityEventSessionRevoked,
		UserID:       stringPtr(session.UserID),
		SessionID:    stringPtr(session.ID),
		Status:       SecurityEventStatusSuccess,
		ReasonCode:   stringPtr(reason),
		MetadataJSON: "{}",
	})
	return nil
}

func (s *Service) createSessionForUser(ctx context.Context, reqCtx RequestContext, user UserSummary) (SessionSummary, error) {
	accessToken, err := s.newAccessToken()
	if err != nil {
		return SessionSummary{}, DependencyError("access token generation failed", err)
	}
	tokenHash, err := hashAccessToken(accessToken, s.tokenHashSecret, s.tokenHashKeyVersion)
	if err != nil {
		return SessionSummary{}, DependencyError("access token hashing failed", err)
	}
	issuedAt := s.now()
	identity, err := s.repo.CreateSession(ctx, CreateSessionParams{
		ID:                        s.newID("sess"),
		UserID:                    user.ID,
		AccessTokenHash:           tokenHash,
		AccessTokenHashAlg:        TokenHashAlg,
		AccessTokenHashKeyVersion: s.tokenHashKeyVersion,
		IssuedAt:                  issuedAt,
		ExpiresAt:                 issuedAt.Add(s.sessionTTL),
		ClientIP:                  optionalString(clientIP(reqCtx)),
		UserAgent:                 optionalString(reqCtx.UserAgent),
		RequestID:                 optionalString(reqCtx.RequestID),
	})
	if err != nil {
		return SessionSummary{}, mapRepositoryError(err, "session not found")
	}
	s.recordSecurityEventBestEffort(ctx, reqCtx, SecurityEventParams{
		EventType:        SecurityEventSessionCreated,
		UserID:           stringPtr(user.ID),
		SessionID:        stringPtr(identity.Session.ID),
		UsernameSnapshot: stringPtr(user.Username),
		Status:           SecurityEventStatusSuccess,
	})
	return SessionSummary{
		SessionID:   identity.Session.ID,
		AccessToken: accessToken,
		TokenType:   identity.Session.TokenType,
		ExpiresAt:   identity.Session.ExpiresAt,
	}, nil
}

func (s *Service) recordSessionFailure(ctx context.Context, reqCtx RequestContext, user *User, username string, reason string) error {
	var userID *string
	if user != nil {
		userID = stringPtr(user.ID)
	}
	return s.recordSecurityEvent(ctx, reqCtx, SecurityEventParams{
		EventType:        SecurityEventSessionCreateFailed,
		UserID:           userID,
		UsernameSnapshot: optionalString(username),
		Status:           SecurityEventStatusFailed,
		ReasonCode:       stringPtr(reason),
	})
}

func (s *Service) recordSecurityEvent(ctx context.Context, reqCtx RequestContext, params SecurityEventParams) error {
	params.ID = s.newID("sevt")
	params.RequestID = optionalString(reqCtx.RequestID)
	params.ClientIP = optionalString(clientIP(reqCtx))
	params.UserAgent = optionalString(reqCtx.UserAgent)
	params.CallerService = optionalString(reqCtx.CallerService)
	params.CreatedAt = s.now()
	if strings.TrimSpace(params.MetadataJSON) == "" {
		params.MetadataJSON = "{}"
	}
	if err := s.repo.RecordSecurityEvent(ctx, params); err != nil {
		return DependencyError("security event write failed", err)
	}
	return nil
}

func (s *Service) recordSecurityEventBestEffort(ctx context.Context, reqCtx RequestContext, params SecurityEventParams) {
	eventType := params.EventType
	if err := s.recordSecurityEvent(ctx, reqCtx, params); err != nil {
		s.logSecurityEventFailure(ctx, reqCtx, eventType, err)
	}
}

func (s *Service) logSecurityEventFailure(ctx context.Context, reqCtx RequestContext, eventType string, err error) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.WarnContext(ctx, "security event write failed",
		"service", "auth",
		"request_id", strings.TrimSpace(reqCtx.RequestID),
		"operation", "record_security_event",
		"event_type", strings.TrimSpace(eventType),
		"status", "failed",
		"error", err,
	)
}

func (s *Service) validateReady() error {
	if s == nil || s.repo == nil {
		return DependencyError("auth repository is not configured", nil)
	}
	if len(s.tokenHashSecret) == 0 {
		return DependencyError("token hash secret is not configured", nil)
	}
	return nil
}

func normalizeCredentials(username string, password string) (string, string, error) {
	fields := map[string]string{}
	username = strings.TrimSpace(username)
	if username == "" {
		fields["username"] = "is required"
	}
	if len(username) > 128 {
		fields["username"] = "must be at most 128 characters"
	}
	if password == "" {
		fields["password"] = "is required"
	}
	if len(password) > 1024 {
		fields["password"] = "must be at most 1024 characters"
	}
	if len(fields) > 0 {
		return "", "", ValidationError("request validation failed", fields)
	}
	return username, password, nil
}

func validateInternalCaller(reqCtx RequestContext) error {
	if strings.TrimSpace(reqCtx.CallerService) == "" {
		return UnauthorizedError()
	}
	return nil
}

func userCanCreateSession(user User, now time.Time) bool {
	if user.Status != UserStatusActive {
		return false
	}
	return user.LockedUntil == nil || !user.LockedUntil.After(now)
}

func summaryFromRecord(user UserRecord) UserSummary {
	return UserSummary{
		ID:          user.ID,
		Username:    user.Username,
		Roles:       append([]string(nil), user.Roles...),
		Permissions: append([]string(nil), user.Permissions...),
	}
}

func invalidCredentialsError() error {
	return NewError(CodeUnauthorized, "invalid username or password", nil)
}

func mapRepositoryError(err error, notFoundMessage string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return NotFoundError(notFoundMessage)
	}
	if errors.Is(err, ErrConflict) {
		return ConflictError("resource already exists", err)
	}
	if _, ok := Classify(err); ok {
		return err
	}
	return DependencyError("repository operation failed", err)
}

func callerService(reqCtx RequestContext) string {
	caller := strings.TrimSpace(reqCtx.CallerService)
	if caller == "" {
		return "gateway"
	}
	return caller
}

func clientIP(reqCtx RequestContext) string {
	if trimmed := strings.TrimSpace(reqCtx.ClientIP); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(reqCtx.ForwardedFor)
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func newID(prefix string) string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return prefix + "_" + hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
	}
	return prefix + "_" + hex.EncodeToString(bytes[:])
}
