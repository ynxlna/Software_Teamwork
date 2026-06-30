package modelclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

func TestCompleteSendsFunctionToolsAndParsesToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-Caller-Service"); got != "qa" {
			t.Errorf("X-Caller-Service = %q", got)
		}
		if got := r.Header.Get("X-Request-Id"); got != "req-model-test" {
			t.Errorf("X-Request-Id = %q", got)
		}
		var request completionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.ToolChoice != "auto" || len(request.Tools) != 1 {
			t.Errorf("unexpected tool request: %+v", request)
		}
		if request.ProfileID != "profile-chat" {
			t.Errorf("profile_id = %q", request.ProfileID)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
          "choices":[{
            "message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"add","arguments":"{\"a\":1}"}}]},
            "finish_reason":"tool_calls"
          }],
          "usage":{"prompt_tokens":7,"completion_tokens":5,"total_tokens":12,"completion_tokens_details":{"reasoning_tokens":2}}
        }`))
	}))
	defer server.Close()

	client, err := New(Config{Endpoint: server.URL, Token: "test-token", TokenHeader: "Authorization", Model: "test", ProfileID: "profile-chat", MaxTokens: 100, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx := service.WithRequestID(context.Background(), "req-model-test")
	completion, err := client.Complete(ctx, []agent.Message{{Role: agent.RoleUser, Content: "add"}}, []agent.ToolDefinition{{
		Type: "function", Function: agent.FunctionTool{Name: "add", Parameters: map[string]any{"type": "object"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if completion.FinishReason != "tool_calls" || completion.Message.ToolCalls[0].Function.Name != "add" {
		t.Fatalf("unexpected completion: %+v", completion)
	}
	if completion.Usage.PromptTokens != 7 || completion.Usage.CompletionTokens != 3 || completion.Usage.ReasoningTokens != 2 || completion.Usage.TotalTokens != 12 {
		t.Fatalf("unexpected usage: %+v", completion.Usage)
	}
}

func TestCompleteRejectsDependencyErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "provider secret detail", http.StatusBadGateway)
	}))
	defer server.Close()
	client, err := New(Config{Endpoint: server.URL, TokenHeader: "Authorization", Model: "test", MaxTokens: 100, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Complete(context.Background(), []agent.Message{{Role: agent.RoleUser, Content: "hi"}}, nil)
	if err == nil || err.Error() != "AI gateway returned HTTP 502" {
		t.Fatalf("unexpected sanitized error: %v", err)
	}
}
