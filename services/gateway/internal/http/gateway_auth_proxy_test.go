package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	gatewayhttp "github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/http"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/platform/authclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/service"
)

func TestCreateSessionCachesSessionWithoutRawToken(t *testing.T) {
	hasher := testHasher(t)
	store := newMemorySessionStore()
	rawToken := "opaque-token-value-that-must-not-be-cached"
	auth := &fakeAuthClient{
		createSessionResult: service.SessionResponse{
			User: service.UserSummary{
				ID:          "usr_1",
				Username:    "alice",
				Roles:       []string{"admin"},
				Permissions: []string{"knowledge:read"},
			},
			Session: service.SessionSummary{
				SessionID:   "sess_1",
				AccessToken: rawToken,
				TokenType:   "Bearer",
				ExpiresAt:   time.Now().Add(time.Hour).UTC(),
			},
		},
	}
	server := newGatewayTestServer(t, gatewayDeps{auth: auth, store: store, hasher: hasher})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", strings.NewReader(`{"username":"alice","password":"secret"}`))
	req.Header.Set("X-Request-Id", "req_session")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if len(store.entries) != 1 {
		t.Fatalf("cached entries = %d", len(store.entries))
	}
	for key, entry := range store.entries {
		if strings.Contains(key, rawToken) || strings.Contains(entry.AccessTokenHash, rawToken) {
			t.Fatalf("raw token leaked into cache key or entry: key=%q entry=%+v", key, entry)
		}
		if entry.UserID != "usr_1" || entry.SessionID != "sess_1" || entry.RequestID != "req_session" {
			t.Fatalf("cache entry = %+v", entry)
		}
	}
	var body service.SessionEnvelope
	decodeJSON(t, res.Body, &body)
	if body.Data.Session.AccessToken != rawToken {
		t.Fatalf("access token response = %q", body.Data.Session.AccessToken)
	}
}

func TestProtectedRouteMissingTokenReturnsUnauthorized(t *testing.T) {
	server := newGatewayTestServer(t, gatewayDeps{store: newMemorySessionStore(), hasher: testHasher(t)})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-bases", nil)
	req.Header.Set("X-Request-Id", "req_missing_token")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body errorBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "unauthorized" || body.Error.RequestID != "req_missing_token" {
		t.Fatalf("error = %+v", body.Error)
	}
}

func TestProtectedRouteSessionStoreFailureReturnsDependencyError(t *testing.T) {
	server := newGatewayTestServer(t, gatewayDeps{store: failingSessionStore{}, hasher: testHasher(t)})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-bases", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("X-Request-Id", "req_redis_down")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body errorBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "dependency_error" {
		t.Fatalf("error = %+v", body.Error)
	}
}

func TestProxyInjectsAuthenticatedContextHeaders(t *testing.T) {
	hasher := testHasher(t)
	store := newMemorySessionStore()
	accessToken := "valid-token"
	store.putToken(t, hasher, accessToken, service.SessionCacheEntry{
		SessionID:   "sess_1",
		UserID:      "usr_1",
		Username:    "alice",
		Roles:       []string{"admin", "operator"},
		Permissions: []string{"knowledge:read", "document:write"},
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour).UTC(),
	})

	var captured http.Header
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/knowledge-bases" {
			t.Fatalf("downstream path = %q", r.URL.Path)
		}
		captured = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"requestId":"req_proxy"}`))
	}))
	defer downstream.Close()

	server := newGatewayTestServer(t, gatewayDeps{
		store:         hasherStore{SessionStore: store},
		hasher:        hasher,
		ownerBaseURLs: map[string]string{"knowledge": downstream.URL},
		serviceToken:  "svc-token",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-bases?page=1", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Request-Id", "req_proxy")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if captured.Get("X-Request-Id") != "req_proxy" ||
		captured.Get("X-User-Id") != "usr_1" ||
		captured.Get("X-User-Roles") != "admin,operator" ||
		captured.Get("X-User-Permissions") != "knowledge:read,document:write" ||
		captured.Get("X-Caller-Service") != "gateway" ||
		captured.Get("X-Service-Token") != "svc-token" {
		t.Fatalf("downstream headers = %#v", captured)
	}
	if captured.Get("Authorization") != "" {
		t.Fatalf("authorization leaked to downstream: %q", captured.Get("Authorization"))
	}
}

func TestProxyOverwritesSpoofedForwardingHeaders(t *testing.T) {
	hasher := testHasher(t)
	store := newMemorySessionStore()
	accessToken := "valid-token"
	store.putToken(t, hasher, accessToken, service.SessionCacheEntry{
		SessionID:   "sess_1",
		UserID:      "usr_1",
		Username:    "alice",
		Roles:       []string{"admin"},
		Permissions: []string{"knowledge:read"},
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour).UTC(),
	})

	var captured http.Header
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"requestId":"req_proxy"}`))
	}))
	defer downstream.Close()

	server := newGatewayTestServer(t, gatewayDeps{
		store:         store,
		hasher:        hasher,
		ownerBaseURLs: map[string]string{"knowledge": downstream.URL},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-bases", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Request-Id", "req_proxy")
	req.Header.Set("Forwarded", "for=203.0.113.9;proto=https")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("X-Forwarded-Host", "evil.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if captured.Get("X-Forwarded-For") != "198.51.100.10" {
		t.Fatalf("x-forwarded-for = %q", captured.Get("X-Forwarded-For"))
	}
	if captured.Get("X-Forwarded-Proto") != "http" {
		t.Fatalf("x-forwarded-proto = %q", captured.Get("X-Forwarded-Proto"))
	}
	if captured.Get("Forwarded") != "" || captured.Get("X-Forwarded-Host") != "" {
		t.Fatalf("spoofed forwarding headers leaked: %#v", captured)
	}
}

func TestAuthClientErrorIsSanitized(t *testing.T) {
	auth := &fakeAuthClient{
		createSessionErr: &authclient.RemoteError{
			Status: http.StatusBadRequest,
			Detail: authclient.ErrorDetail{
				Code:    "internal_sql_error",
				Message: "select * from auth_credentials",
				Fields:  map[string]string{"password_hash": "secret"},
			},
		},
	}
	server := newGatewayTestServer(t, gatewayDeps{
		auth:   auth,
		store:  newMemorySessionStore(),
		hasher: testHasher(t),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", strings.NewReader(`{"username":"alice","password":"secret"}`))
	req.Header.Set("X-Request-Id", "req_auth_sanitized")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	raw := res.Body.String()
	for _, sensitive := range []string{"internal_sql_error", "auth_credentials", "password_hash", "select *"} {
		if strings.Contains(raw, sensitive) {
			t.Fatalf("auth internal detail leaked in response: %s", raw)
		}
	}
	var body errorBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "validation_error" ||
		body.Error.Message != "request validation failed" ||
		body.Error.RequestID != "req_auth_sanitized" {
		t.Fatalf("error = %+v", body.Error)
	}
}

func TestProxyMapsGatewayPathsToOwnerServiceNamespaces(t *testing.T) {
	hasher := testHasher(t)
	store := newMemorySessionStore()
	accessToken := "valid-token"
	store.putToken(t, hasher, accessToken, service.SessionCacheEntry{
		SessionID:   "sess_1",
		UserID:      "usr_1",
		Username:    "alice",
		Roles:       []string{"admin"},
		Permissions: []string{"knowledge:read", "qa:read", "document:read"},
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour).UTC(),
	})

	cases := []struct {
		name     string
		owner    string
		method   string
		path     string
		expected string
	}{
		{
			name:     "knowledge internal namespace",
			owner:    "knowledge",
			method:   http.MethodGet,
			path:     "/api/v1/knowledge-bases/kb_1",
			expected: "/internal/v1/knowledge-bases/kb_1",
		},
		{
			name:     "document service root namespace",
			owner:    "document",
			method:   http.MethodGet,
			path:     "/api/v1/report-types",
			expected: "/report-types",
		},
		{
			name:     "qa internal namespace",
			owner:    "qa",
			method:   http.MethodGet,
			path:     "/api/v1/qa-sessions/sess_1/messages",
			expected: "/internal/v1/qa-sessions/sess_1/messages",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedPath string
			downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{},"requestId":"req_proxy"}`))
			}))
			defer downstream.Close()

			server := newGatewayTestServer(t, gatewayDeps{
				store:         store,
				hasher:        hasher,
				ownerBaseURLs: map[string]string{tc.owner: downstream.URL},
			})
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+accessToken)
			req.Header.Set("X-Request-Id", "req_proxy")
			res := httptest.NewRecorder()

			server.ServeHTTP(res, req)

			if res.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
			}
			if capturedPath != tc.expected {
				t.Fatalf("downstream path = %q, want %q", capturedPath, tc.expected)
			}
		})
	}
}

func TestProxyUsesDownstreamPathTemplate(t *testing.T) {
	hasher := testHasher(t)
	store := newMemorySessionStore()
	accessToken := "valid-token"
	store.putToken(t, hasher, accessToken, service.SessionCacheEntry{
		SessionID:   "sess_1",
		UserID:      "usr_1",
		Username:    "alice",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:model-profile:write"},
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour).UTC(),
	})

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/model-profiles/mp_1" {
			t.Fatalf("downstream path = %q", r.URL.Path)
		}
		if r.URL.RawQuery != "includeDisabled=true" {
			t.Fatalf("downstream query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"mp_1"},"requestId":"req_model_profile"}`))
	}))
	defer downstream.Close()

	server := newGatewayTestServer(t, gatewayDeps{
		store:         store,
		hasher:        hasher,
		ownerBaseURLs: map[string]string{"ai-gateway": downstream.URL},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/model-profiles/mp_1?includeDisabled=true", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Request-Id", "req_model_profile")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
}

func TestProxyStreamsBinaryContentWithoutJSONEnvelope(t *testing.T) {
	hasher := testHasher(t)
	store := newMemorySessionStore()
	accessToken := "valid-token"
	store.putToken(t, hasher, accessToken, service.SessionCacheEntry{
		SessionID:   "sess_1",
		UserID:      "usr_1",
		Username:    "alice",
		Roles:       []string{},
		Permissions: []string{"knowledge:read"},
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour).UTC(),
	})
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0, 1, 2, 3})
	}))
	defer downstream.Close()

	server := newGatewayTestServer(t, gatewayDeps{
		store:         store,
		hasher:        hasher,
		ownerBaseURLs: map[string]string{"knowledge": downstream.URL},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/doc_1/content", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", res.Code, res.Body.String())
	}
	if got := res.Body.Bytes(); !bytes.Equal(got, []byte{0, 1, 2, 3}) {
		t.Fatalf("body = %#v", got)
	}
}

func TestProxyNormalizesDownstreamErrorBody(t *testing.T) {
	hasher := testHasher(t)
	store := newMemorySessionStore()
	accessToken := "valid-token"
	store.putToken(t, hasher, accessToken, service.SessionCacheEntry{
		SessionID:   "sess_1",
		UserID:      "usr_1",
		Username:    "alice",
		Roles:       []string{},
		Permissions: []string{"knowledge:read"},
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour).UTC(),
	})
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"sql":"select * from private_table","internalUrl":"http://knowledge.internal"}`))
	}))
	defer downstream.Close()

	server := newGatewayTestServer(t, gatewayDeps{
		store:         store,
		hasher:        hasher,
		ownerBaseURLs: map[string]string{"knowledge": downstream.URL},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-bases/kb_1", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Request-Id", "req_downstream_404")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	raw := res.Body.String()
	if strings.Contains(raw, "private_table") || strings.Contains(raw, "knowledge.internal") {
		t.Fatalf("downstream raw body leaked: %s", raw)
	}
	var body errorBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "not_found" || body.Error.RequestID != "req_downstream_404" {
		t.Fatalf("error = %+v", body.Error)
	}
}

type gatewayDeps struct {
	auth          gatewayhttp.AuthClient
	store         service.SessionStore
	hasher        service.TokenHasher
	ownerBaseURLs map[string]string
	serviceToken  string
}

func newGatewayTestServer(t *testing.T, deps gatewayDeps) http.Handler {
	t.Helper()
	if deps.ownerBaseURLs == nil {
		deps.ownerBaseURLs = map[string]string{}
	}
	return gatewayhttp.NewServer(gatewayhttp.Config{
		Logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		ServiceVersion:       "test",
		Environment:          "test",
		RequestTimeout:       time.Second,
		MaxBodyBytes:         1024 * 1024,
		CORSAllowedOrigins:   []string{"*"},
		DownstreamTimeout:    time.Second,
		AuthClient:           deps.auth,
		SessionStore:         deps.store,
		TokenHasher:          deps.hasher,
		OwnerBaseURLs:        deps.ownerBaseURLs,
		InternalServiceToken: deps.serviceToken,
	})
}

func testHasher(t *testing.T) service.TokenHasher {
	t.Helper()
	hasher, err := service.NewTokenHasher("test-secret", "v1")
	if err != nil {
		t.Fatalf("NewTokenHasher() error = %v", err)
	}
	return hasher
}

type fakeAuthClient struct {
	createUserResult    service.SessionResponse
	createUserErr       error
	createSessionResult service.SessionResponse
	createSessionErr    error
	deleteSessionID     string
	deleteSessionErr    error
}

func (c *fakeAuthClient) CreateUser(context.Context, string, []byte) (service.SessionResponse, error) {
	if c.createUserErr != nil {
		return service.SessionResponse{}, c.createUserErr
	}
	return c.createUserResult, nil
}

func (c *fakeAuthClient) CreateSession(context.Context, string, []byte) (service.SessionResponse, error) {
	if c.createSessionErr != nil {
		return service.SessionResponse{}, c.createSessionErr
	}
	return c.createSessionResult, nil
}

func (c *fakeAuthClient) DeleteSession(_ context.Context, _ string, sessionID string) error {
	if c.deleteSessionErr != nil {
		return c.deleteSessionErr
	}
	c.deleteSessionID = sessionID
	return nil
}

type memorySessionStore struct {
	mu      sync.Mutex
	entries map[string]service.SessionCacheEntry
}

func newMemorySessionStore() *memorySessionStore {
	return &memorySessionStore{entries: map[string]service.SessionCacheEntry{}}
}

func (s *memorySessionStore) Put(_ context.Context, entry service.SessionCacheEntry, ttl time.Duration) error {
	if ttl <= 0 {
		return service.ErrSessionInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[entry.AccessTokenHash] = entry
	return nil
}

func (s *memorySessionStore) Get(_ context.Context, accessTokenHash string) (service.SessionCacheEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[accessTokenHash]
	if !ok {
		return service.SessionCacheEntry{}, service.ErrSessionNotFound
	}
	return entry, nil
}

func (s *memorySessionStore) Delete(_ context.Context, accessTokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, accessTokenHash)
	return nil
}

func (s *memorySessionStore) putToken(t *testing.T, hasher service.TokenHasher, accessToken string, entry service.SessionCacheEntry) {
	t.Helper()
	hash, err := hasher.Hash(accessToken)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	entry.AccessTokenHash = hash
	entry.CachedAt = time.Now().UTC()
	if entry.IssuedAt.IsZero() {
		entry.IssuedAt = entry.CachedAt
	}
	if err := s.Put(context.Background(), entry, time.Until(entry.ExpiresAt)); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
}

type failingSessionStore struct{}

func (failingSessionStore) Put(context.Context, service.SessionCacheEntry, time.Duration) error {
	return errors.New("unexpected put")
}

func (failingSessionStore) Get(context.Context, string) (service.SessionCacheEntry, error) {
	return service.SessionCacheEntry{}, service.ErrSessionStoreUnavailable
}

func (failingSessionStore) Delete(context.Context, string) error {
	return service.ErrSessionStoreUnavailable
}

type hasherStore struct {
	service.SessionStore
}

var _ service.SessionStore = (*memorySessionStore)(nil)
