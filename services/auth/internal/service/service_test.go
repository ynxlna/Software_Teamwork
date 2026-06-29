package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPasswordHashUsesArgon2idV1PHC(t *testing.T) {
	hash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("hash = %q", hash)
	}
	ok, err := verifyPassword("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("verifyPassword() error = %v", err)
	}
	if !ok {
		t.Fatalf("verifyPassword() = false")
	}
	ok, err = verifyPassword("wrong", hash)
	if err != nil {
		t.Fatalf("verifyPassword(wrong) error = %v", err)
	}
	if ok {
		t.Fatalf("verifyPassword(wrong) = true")
	}
}

func TestAccessTokenHashIsVersionedHMAC(t *testing.T) {
	hash, err := hashAccessToken("atk_v1_example", []byte("secret"), "v1")
	if err != nil {
		t.Fatalf("hashAccessToken() error = %v", err)
	}
	if !strings.HasPrefix(hash, "hmac-sha256:v1:") {
		t.Fatalf("hash = %q", hash)
	}
	if strings.Contains(hash, "atk_v1_example") {
		t.Fatalf("hash leaks raw token: %q", hash)
	}
	again, err := hashAccessToken("atk_v1_example", []byte("secret"), "v1")
	if err != nil {
		t.Fatalf("hashAccessToken() second error = %v", err)
	}
	if hash != again {
		t.Fatalf("hash is not deterministic: %q != %q", hash, again)
	}
}

func TestCreateSessionRejectsWrongPasswordAndRecordsFailure(t *testing.T) {
	repo := newFakeRepository(t)
	svc := newTestService(repo, "atk_v1_fixed")

	_, err := svc.CreateSession(context.Background(), testRequestContext(), CreateSessionInput{
		Username: "alice",
		Password: "wrong-password",
	})
	if appErr := requireAppError(t, err); appErr.Code != CodeUnauthorized {
		t.Fatalf("code = %s", appErr.Code)
	}
	if len(repo.sessions) != 0 {
		t.Fatalf("sessions = %+v", repo.sessions)
	}
	if !repo.hasEvent(SecurityEventSessionCreateFailed, SecurityEventStatusFailed, reasonInvalidCredentials) {
		t.Fatalf("events = %+v", repo.events)
	}
}

func TestCreateUserRejectsDuplicateUsername(t *testing.T) {
	repo := newFakeRepository(t)
	svc := newTestService(repo, "atk_v1_fixed")

	_, err := svc.CreateUser(context.Background(), testRequestContext(), CreateUserInput{
		Username: "alice",
		Password: "new-password",
	})
	if appErr := requireAppError(t, err); appErr.Code != CodeConflict {
		t.Fatalf("code = %s", appErr.Code)
	}
}

func TestCreateUserReturnsTokenButPersistsOnlyHash(t *testing.T) {
	repo := newFakeRepository(t)
	svc := newTestService(repo, "atk_v1_created")

	result, err := svc.CreateUser(context.Background(), testRequestContext(), CreateUserInput{
		Username: "bob",
		Password: "bob-password",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if result.Session.AccessToken != "atk_v1_created" {
		t.Fatalf("access token = %q", result.Session.AccessToken)
	}
	if result.Session.SessionID == "" || result.Session.TokenType != TokenTypeBearer {
		t.Fatalf("session = %+v", result.Session)
	}
	if got, want := strings.Join(result.User.Roles, ","), "standard"; got != want {
		t.Fatalf("roles = %q", got)
	}
	stored := repo.sessions[result.Session.SessionID]
	if stored.AccessTokenHash == "" || !strings.HasPrefix(stored.AccessTokenHash, "hmac-sha256:v1:") {
		t.Fatalf("stored hash = %q", stored.AccessTokenHash)
	}
	if strings.Contains(stored.AccessTokenHash, result.Session.AccessToken) {
		t.Fatalf("stored hash leaks token: %q", stored.AccessTokenHash)
	}
	if !repo.hasEvent(SecurityEventUserCreated, SecurityEventStatusSuccess, "") ||
		!repo.hasEvent(SecurityEventRoleAssigned, SecurityEventStatusSuccess, reasonDefaultRole) ||
		!repo.hasEvent(SecurityEventSessionCreated, SecurityEventStatusSuccess, "") {
		t.Fatalf("events = %+v", repo.events)
	}
}

func TestCreateUserReturnsSuccessWhenSecurityEventWriteFails(t *testing.T) {
	repo := newFakeRepository(t)
	repo.eventErr = errors.New("security event store unavailable")
	svc := newTestService(repo, "atk_v1_created")

	result, err := svc.CreateUser(context.Background(), testRequestContext(), CreateUserInput{
		Username: "bob",
		Password: "bob-password",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if result.User.ID == "" || result.Session.SessionID == "" || result.Session.AccessToken != "atk_v1_created" {
		t.Fatalf("result = %+v", result)
	}
	if _, ok := repo.usersByUsername["bob"]; !ok {
		t.Fatalf("user was not persisted")
	}
	if _, ok := repo.sessions[result.Session.SessionID]; !ok {
		t.Fatalf("session was not persisted")
	}
}

func TestCreateSessionReturnsSuccessWhenSecurityEventWriteFails(t *testing.T) {
	repo := newFakeRepository(t)
	repo.eventErr = errors.New("security event store unavailable")
	svc := newTestService(repo, "atk_v1_created")

	result, err := svc.CreateSession(context.Background(), testRequestContext(), CreateSessionInput{
		Username: "alice",
		Password: "correct-password",
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if result.Session.SessionID == "" || result.Session.AccessToken != "atk_v1_created" {
		t.Fatalf("session = %+v", result.Session)
	}
	if _, ok := repo.sessions[result.Session.SessionID]; !ok {
		t.Fatalf("session was not persisted")
	}
}

func TestRevokedTokenNoLongerReturnsActiveSession(t *testing.T) {
	repo := newFakeRepository(t)
	svc := newTestService(repo, "atk_v1_revoked")

	result, err := svc.CreateSession(context.Background(), testRequestContext(), CreateSessionInput{
		Username: "alice",
		Password: "correct-password",
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if _, err := svc.GetSessionByAccessToken(context.Background(), testRequestContext(), result.Session.AccessToken); err != nil {
		t.Fatalf("GetSessionByAccessToken() before revoke error = %v", err)
	}
	if err := svc.RevokeSession(context.Background(), testRequestContext(), result.Session.SessionID, "user_logout"); err != nil {
		t.Fatalf("RevokeSession() error = %v", err)
	}
	_, err = svc.GetSessionByAccessToken(context.Background(), testRequestContext(), result.Session.AccessToken)
	if appErr := requireAppError(t, err); appErr.Code != CodeUnauthorized {
		t.Fatalf("code = %s", appErr.Code)
	}
	if !repo.hasEvent(SecurityEventSessionRevoked, SecurityEventStatusSuccess, "user_logout") {
		t.Fatalf("events = %+v", repo.events)
	}
}

func TestRevokeSessionReturnsSuccessWhenSecurityEventWriteFails(t *testing.T) {
	repo := newFakeRepository(t)
	svc := newTestService(repo, "atk_v1_revoked")

	result, err := svc.CreateSession(context.Background(), testRequestContext(), CreateSessionInput{
		Username: "alice",
		Password: "correct-password",
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	repo.eventErr = errors.New("security event store unavailable")

	if err := svc.RevokeSession(context.Background(), testRequestContext(), result.Session.SessionID, "user_logout"); err != nil {
		t.Fatalf("RevokeSession() error = %v", err)
	}
	if got := repo.sessions[result.Session.SessionID].Status; got != SessionStatusRevoked {
		t.Fatalf("session status = %q", got)
	}
}

func newTestService(repo *fakeRepository, token string) *Service {
	now := time.Date(2026, 6, 29, 8, 0, 0, 0, time.UTC)
	counter := map[string]int{}
	return New(repo,
		WithClock(func() time.Time { return now }),
		WithTokenGenerator(func() (string, error) { return token, nil }),
		WithTokenHashSecret([]byte("test-token-hash-secret")),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithIDGenerator(func(prefix string) string {
			counter[prefix]++
			return prefix + "_" + strconv.Itoa(counter[prefix])
		}),
	)
}

func newFakeRepository(t *testing.T) *fakeRepository {
	t.Helper()
	now := time.Date(2026, 6, 29, 8, 0, 0, 0, time.UTC)
	hash, err := hashPassword("correct-password")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}
	user := UserRecord{
		User: User{
			ID:        "usr_alice",
			Username:  "alice",
			Status:    UserStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Roles:       []string{"standard"},
		Permissions: []string{"knowledge:read"},
	}
	return &fakeRepository{
		now:             now,
		usersByID:       map[string]UserRecord{user.ID: user},
		usersByUsername: map[string]UserRecord{user.Username: user},
		credentials: map[string]Credential{
			user.ID: {
				ID:                        "cred_alice",
				UserID:                    user.ID,
				CredentialType:            CredentialTypePassword,
				PasswordHash:              hash,
				PasswordHashAlg:           PasswordHashAlg,
				PasswordHashParamsVersion: PasswordHashParamsVersion,
			},
		},
		sessions:     map[string]Session{},
		activeByHash: map[string]string{},
	}
}

func testRequestContext() RequestContext {
	return RequestContext{
		RequestID:     "req_test",
		CallerService: "gateway",
		ClientIP:      "127.0.0.1",
		UserAgent:     "auth-test",
	}
}

func requireAppError(t *testing.T, err error) *AppError {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error is not AppError: %T %v", err, err)
	}
	return appErr
}

type fakeRepository struct {
	now             time.Time
	usersByID       map[string]UserRecord
	usersByUsername map[string]UserRecord
	credentials     map[string]Credential
	sessions        map[string]Session
	activeByHash    map[string]string
	events          []SecurityEventParams
	eventErr        error
}

func (r *fakeRepository) FindUserByID(_ context.Context, id string) (UserRecord, error) {
	user, ok := r.usersByID[id]
	if !ok {
		return UserRecord{}, ErrNotFound
	}
	return user, nil
}

func (r *fakeRepository) FindUserByUsername(_ context.Context, username string) (UserRecord, error) {
	user, ok := r.usersByUsername[username]
	if !ok {
		return UserRecord{}, ErrNotFound
	}
	return user, nil
}

func (r *fakeRepository) FindCredentialByUserID(_ context.Context, userID string) (Credential, error) {
	credential, ok := r.credentials[userID]
	if !ok {
		return Credential{}, ErrNotFound
	}
	return credential, nil
}

func (r *fakeRepository) FindSessionByID(_ context.Context, id string) (SessionIdentity, error) {
	session, ok := r.sessions[id]
	if !ok {
		return SessionIdentity{}, ErrNotFound
	}
	user, err := r.FindUserByID(context.Background(), session.UserID)
	if err != nil {
		return SessionIdentity{}, err
	}
	return SessionIdentity{Session: session, User: summaryFromRecord(user), AccessTokenHash: session.AccessTokenHash}, nil
}

func (r *fakeRepository) FindActiveSessionByTokenHash(ctx context.Context, tokenHash string) (SessionIdentity, error) {
	sessionID, ok := r.activeByHash[tokenHash]
	if !ok {
		return SessionIdentity{}, ErrNotFound
	}
	session, ok := r.sessions[sessionID]
	if !ok || session.Status != SessionStatusActive || !session.ExpiresAt.After(r.now) {
		return SessionIdentity{}, ErrNotFound
	}
	return r.FindSessionByID(ctx, session.ID)
}

func (r *fakeRepository) CreateUserWithCredential(_ context.Context, params CreateUserParams) (UserRecord, error) {
	if _, exists := r.usersByUsername[params.Username]; exists {
		return UserRecord{}, ErrConflict
	}
	user := UserRecord{
		User: User{
			ID:          params.ID,
			Username:    params.Username,
			DisplayName: params.DisplayName,
			Status:      params.Status,
			CreatedAt:   params.CreatedAt,
			UpdatedAt:   params.CreatedAt,
		},
		Roles:       []string{params.DefaultRoleCode},
		Permissions: []string{"knowledge:read"},
	}
	r.usersByID[user.ID] = user
	r.usersByUsername[user.Username] = user
	r.credentials[user.ID] = Credential{
		ID:                        params.PasswordCredentialID,
		UserID:                    user.ID,
		CredentialType:            CredentialTypePassword,
		PasswordHash:              params.PasswordHash,
		PasswordHashAlg:           params.PasswordHashAlg,
		PasswordHashParamsVersion: params.PasswordHashParamsVersion,
	}
	return user, nil
}

func (r *fakeRepository) CreateSession(_ context.Context, params CreateSessionParams) (SessionIdentity, error) {
	if _, ok := r.usersByID[params.UserID]; !ok {
		return SessionIdentity{}, ErrNotFound
	}
	session := Session{
		ID:                        params.ID,
		UserID:                    params.UserID,
		AccessTokenHash:           params.AccessTokenHash,
		AccessTokenHashAlg:        params.AccessTokenHashAlg,
		AccessTokenHashKeyVersion: params.AccessTokenHashKeyVersion,
		TokenType:                 TokenTypeBearer,
		Status:                    SessionStatusActive,
		IssuedAt:                  params.IssuedAt,
		ExpiresAt:                 params.ExpiresAt,
		ClientIP:                  params.ClientIP,
		UserAgent:                 params.UserAgent,
		CreatedRequestID:          params.RequestID,
		CreatedAt:                 params.IssuedAt,
		UpdatedAt:                 params.IssuedAt,
	}
	r.sessions[session.ID] = session
	r.activeByHash[session.AccessTokenHash] = session.ID
	return r.FindSessionByID(context.Background(), session.ID)
}

func (r *fakeRepository) RevokeSession(_ context.Context, params RevokeSessionParams) (Session, error) {
	session, ok := r.sessions[params.SessionID]
	if !ok || session.Status != SessionStatusActive {
		return Session{}, ErrNotFound
	}
	session.Status = SessionStatusRevoked
	session.RevokedAt = &params.RevokedAt
	session.RevokeReason = &params.Reason
	session.RevokedRequestID = params.RequestID
	session.UpdatedAt = params.RevokedAt
	r.sessions[session.ID] = session
	delete(r.activeByHash, session.AccessTokenHash)
	return session, nil
}

func (r *fakeRepository) RecordSecurityEvent(_ context.Context, params SecurityEventParams) error {
	r.events = append(r.events, params)
	if r.eventErr != nil {
		return r.eventErr
	}
	return nil
}

func (r *fakeRepository) hasEvent(eventType string, status string, reason string) bool {
	for _, event := range r.events {
		if event.EventType != eventType || event.Status != status {
			continue
		}
		if reason == "" {
			return true
		}
		if event.ReasonCode != nil && *event.ReasonCode == reason {
			return true
		}
	}
	return false
}
