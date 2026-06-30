package aigateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

func TestProfileClientGetsModelProfileWithInternalHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/internal/v1/model-profiles/mp-chat" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Service-Token") != "service-token" {
			t.Fatalf("X-Service-Token = %q", r.Header.Get("X-Service-Token"))
		}
		if r.Header.Get("X-Caller-Service") != "document" {
			t.Fatalf("X-Caller-Service = %q", r.Header.Get("X-Caller-Service"))
		}
		if r.Header.Get("X-Request-Id") != "req-123" {
			t.Fatalf("X-Request-Id = %q", r.Header.Get("X-Request-Id"))
		}
		if r.Header.Get("X-User-Id") != "user-1" {
			t.Fatalf("X-User-Id = %q", r.Header.Get("X-User-Id"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":        "mp-chat",
				"purpose":   "chat",
				"provider":  "openai-compatible",
				"model":     "gpt-test",
				"enabled":   true,
				"timeoutMs": 45000,
			},
			"requestId": "req-123",
		})
	}))
	defer server.Close()

	client, err := NewProfileClient(server.URL, "service-token", server.Client())
	if err != nil {
		t.Fatalf("NewProfileClient() error = %v", err)
	}
	profile, err := client.GetModelProfile(context.Background(), service.RequestContext{
		RequestID: "req-123",
		UserID:    "user-1",
	}, "mp-chat")
	if err != nil {
		t.Fatalf("GetModelProfile() error = %v", err)
	}
	if profile.ID != "mp-chat" || profile.Purpose != "chat" || profile.Model != "gpt-test" || !profile.Enabled || profile.TimeoutSeconds != 45 {
		t.Fatalf("profile = %+v", profile)
	}
}

func TestProfileClientMapsMissingProfileToNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    "not_found",
				"message": "model profile not found",
			},
		})
	}))
	defer server.Close()

	client, err := NewProfileClient(server.URL, "service-token", server.Client())
	if err != nil {
		t.Fatalf("NewProfileClient() error = %v", err)
	}
	_, err = client.GetModelProfile(context.Background(), service.RequestContext{}, "missing")
	if err == nil {
		t.Fatalf("GetModelProfile() error = nil, want not_found")
	}
	appErr, ok := service.Classify(err)
	if !ok || appErr.Code != service.CodeNotFound {
		t.Fatalf("GetModelProfile() error = %#v, want not_found", err)
	}
}
