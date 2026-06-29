package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/ai-gateway/internal/middleware"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/ai-gateway/internal/service"
)

func TestModelProfileRequiresServiceToken(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/model-profiles", nil)
	req.Header.Set("X-Caller-Service", "gateway")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestModelProfileRequiresCallerService(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/model-profiles", nil)
	req.Header.Set("X-Service-Token", "service-token")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestModelProfileRejectsUnknownCallerService(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/model-profiles", nil)
	req.Header.Set("X-Service-Token", "service-token")
	req.Header.Set("X-Caller-Service", "unknown-service")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"forbidden"`)) {
		t.Fatalf("body = %s, want forbidden error", rec.Body.String())
	}
}

func TestCreateModelProfileDoesNotReturnAPIKey(t *testing.T) {
	server := newTestServer(t)
	body := `{"name":"default-chat","purpose":"chat","provider":"siliconflow","baseUrl":"https://api.siliconflow.cn/v1","model":"Qwen","apiKey":"sk-secret-value","enabled":true,"isDefault":true}`
	req := authedRequest(http.MethodPost, "/internal/v1/model-profiles", strings.NewReader(body))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("sk-secret-value")) || bytes.Contains(rec.Body.Bytes(), []byte("apiKey\"")) {
		t.Fatalf("response leaked api key: %s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("apiKeyConfigured")) {
		t.Fatalf("response missing apiKeyConfigured: %s", rec.Body.String())
	}
}

func TestInvalidJSONReturnsSecretSafeError(t *testing.T) {
	server := newTestServer(t)
	req := authedRequest(http.MethodPost, "/internal/v1/model-profiles", strings.NewReader(`{"apiKey":"sk-secret"`))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("sk-secret")) {
		t.Fatalf("error leaked request body: %s", rec.Body.String())
	}
}

func TestReadyReturnsDegradedWhenProfilesMissing(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("degraded")) {
		t.Fatalf("ready body = %s", rec.Body.String())
	}
}

func TestModelInvocationRoutesReturnNotImplemented(t *testing.T) {
	server := newTestServer(t)
	paths := []string{
		"/internal/v1/chat/completions",
		"/internal/v1/embeddings",
		"/internal/v1/rerankings",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := authedRequest(http.MethodPost, path, strings.NewReader(`{}`))
			rec := httptest.NewRecorder()

			server.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte(`"type":"not_implemented_error"`)) {
				t.Fatalf("body = %s, want OpenAI-style not implemented error", rec.Body.String())
			}
			if bytes.Contains(rec.Body.Bytes(), []byte(`"data"`)) || bytes.Contains(rec.Body.Bytes(), []byte(`"requestId"`)) {
				t.Fatalf("body = %s, model invocation errors must not use project envelope", rec.Body.String())
			}
		})
	}
}

func TestModelInvocationRoutesRejectUnknownCallerService(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/chat/completions", strings.NewReader(`{}`))
	req.Header.Set("X-Service-Token", "service-token")
	req.Header.Set("X-Caller-Service", "unknown-service")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"type":"permission_error"`)) {
		t.Fatalf("body = %s, want OpenAI-style permission error", rec.Body.String())
	}
}

func TestModelInvocationRoutesRequireServiceToken(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/chat/completions", strings.NewReader(`{}`))
	req.Header.Set("X-Caller-Service", "qa")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"type":"authentication_error"`)) {
		t.Fatalf("body = %s, want OpenAI-style auth error", rec.Body.String())
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	tokenHash := sha256.Sum256([]byte("service-token"))
	auth, err := middleware.NewServiceTokenAuthenticator([]string{"sha256:" + hex.EncodeToString(tokenHash[:])})
	if err != nil {
		t.Fatalf("NewServiceTokenAuthenticator() error = %v", err)
	}
	encryptor, err := service.NewCredentialEncryptor([]byte("12345678901234567890123456789012"), "local-v1")
	if err != nil {
		t.Fatalf("NewCredentialEncryptor() error = %v", err)
	}
	return NewServer(Config{
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Profiles:      service.New(newMemoryRepository(), encryptor, 60000),
		Authenticator: auth,
	})
}

func authedRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("X-Service-Token", "service-token")
	req.Header.Set("X-Caller-Service", "gateway")
	req.Header.Set("Content-Type", "application/json")
	return req
}
