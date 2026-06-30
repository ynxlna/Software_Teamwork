package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
)

type fakeQAService struct {
	create       func(context.Context, string, string) (service.Conversation, error)
	list         func(context.Context, string, service.ConversationListOptions) (service.Page[service.Conversation], error)
	get          func(context.Context, string, string) (service.Conversation, error)
	update       func(context.Context, string, string, string, string) (service.Conversation, error)
	delete       func(context.Context, string, string) error
	listMessages func(context.Context, string, string, service.MessageListOptions) (service.Page[service.Message], error)
	ask          func(context.Context, string, string, service.AskInput, service.ProgressObserver) (service.AskResult, error)
}

type fakeSettingsService struct{}
type fakeResourceService struct{}

func (fakeSettingsService) GetSettings(context.Context) (service.QASettings, error) {
	return service.QASettings{}, nil
}
func (fakeSettingsService) UpdateSettings(context.Context, string, string, service.UpdateQASettingsInput) (service.QASettings, error) {
	return service.QASettings{}, nil
}
func (fakeSettingsService) ListMCPServers(context.Context) ([]service.MCPServer, error) {
	return []service.MCPServer{}, nil
}
func (fakeSettingsService) CreateMCPServer(context.Context, string, string, service.MCPServerInput) (service.MCPServer, error) {
	return service.MCPServer{}, nil
}
func (fakeSettingsService) UpdateMCPServer(context.Context, string, string, string, service.MCPServerPatch) (service.MCPServer, error) {
	return service.MCPServer{}, nil
}
func (fakeSettingsService) DeleteMCPServer(context.Context, string, string, string) error {
	return nil
}
func (fakeSettingsService) TestLLMConnection(context.Context, service.LLMConnectionTestInput) (service.LLMConnectionTestResult, error) {
	return service.LLMConnectionTestResult{Success: true}, nil
}
func (fakeSettingsService) TestMCPConnection(context.Context, service.MCPConnectionTestInput) (service.MCPConnectionTestResult, error) {
	return service.MCPConnectionTestResult{Success: true}, nil
}

func (f fakeQAService) CreateConversation(ctx context.Context, userID, title string) (service.Conversation, error) {
	return f.create(ctx, userID, title)
}
func (f fakeQAService) ListConversations(ctx context.Context, userID string, options service.ConversationListOptions) (service.Page[service.Conversation], error) {
	if f.list != nil {
		return f.list(ctx, userID, options)
	}
	return service.Page[service.Conversation]{Items: []service.Conversation{}, Page: 1, PageSize: 20}, nil
}
func (f fakeQAService) GetConversation(ctx context.Context, userID, sessionID string) (service.Conversation, error) {
	if f.get != nil {
		return f.get(ctx, userID, sessionID)
	}
	return service.Conversation{}, nil
}
func (f fakeQAService) UpdateConversation(ctx context.Context, userID, sessionID, title, status string) (service.Conversation, error) {
	if f.update != nil {
		return f.update(ctx, userID, sessionID, title, status)
	}
	return service.Conversation{}, nil
}
func (fakeResourceService) GetResponseRun(context.Context, string, string) (service.ResponseRun, error) {
	return service.ResponseRun{}, nil
}
func (fakeResourceService) CancelResponseRun(context.Context, string, string) (service.ResponseRun, error) {
	return service.ResponseRun{}, nil
}
func (fakeResourceService) ListStreamEvents(context.Context, string, string, string, int) ([]service.StreamEvent, error) {
	return []service.StreamEvent{}, nil
}
func (fakeResourceService) ListMessageCitations(context.Context, string, string) ([]service.Citation, error) {
	return []service.Citation{}, nil
}
func (fakeResourceService) GetCitation(context.Context, string, string) (service.Citation, error) {
	return service.Citation{}, nil
}
func (fakeResourceService) LookupCitations(context.Context, string, []string) ([]service.Citation, error) {
	return []service.Citation{}, nil
}
func (fakeResourceService) ListToolCalls(context.Context, string, string) ([]service.AgentToolCall, error) {
	return []service.AgentToolCall{}, nil
}
func (fakeResourceService) GetActiveQAConfigVersion(context.Context) (service.QAConfigVersion, error) {
	return service.QAConfigVersion{}, nil
}
func (fakeResourceService) CreateQAConfigVersion(context.Context, string, service.CreateQAConfigVersionInput) (service.QAConfigVersion, error) {
	return service.QAConfigVersion{}, nil
}
func (fakeResourceService) GetActiveLLMConfigVersion(context.Context) (service.LLMConfigVersion, error) {
	return service.LLMConfigVersion{}, nil
}
func (fakeResourceService) CreateLLMConfigVersion(context.Context, string, service.CreateLLMConfigVersionInput) (service.LLMConfigVersion, error) {
	return service.LLMConfigVersion{}, nil
}
func (fakeResourceService) TestLLMConnection(context.Context, string, service.LLMProfileTestInput) (service.LLMProfileTestResult, error) {
	return service.LLMProfileTestResult{}, nil
}
func (fakeResourceService) CreateRetrievalTestRun(context.Context, string, service.RetrievalTestInput) (service.RetrievalTestRun, error) {
	return service.RetrievalTestRun{}, nil
}
func (fakeResourceService) GetRetrievalTestRun(context.Context, string, string) (service.RetrievalTestRun, error) {
	return service.RetrievalTestRun{}, nil
}
func (fakeResourceService) GetMetricsOverview(context.Context, int) (service.MetricsOverview, error) {
	return service.MetricsOverview{}, nil
}
func (fakeResourceService) GetMetricsTrend(context.Context, int) (service.MetricsTrend, error) {
	return service.MetricsTrend{}, nil
}
func (fakeResourceService) GetTopQueries(context.Context, int, int) ([]service.TopQuery, error) {
	return []service.TopQuery{}, nil
}
func (fakeResourceService) GetIntentDistribution(context.Context, int) ([]service.IntentDistribution, error) {
	return []service.IntentDistribution{}, nil
}
func (f fakeQAService) DeleteConversation(ctx context.Context, userID, sessionID string) error {
	if f.delete != nil {
		return f.delete(ctx, userID, sessionID)
	}
	return nil
}
func (f fakeQAService) ListMessages(ctx context.Context, userID, sessionID string, options service.MessageListOptions) (service.Page[service.Message], error) {
	if f.listMessages != nil {
		return f.listMessages(ctx, userID, sessionID, options)
	}
	return service.Page[service.Message]{Items: []service.Message{}, Page: 1, PageSize: 50}, nil
}
func (f fakeQAService) Ask(ctx context.Context, userID, conversationID string, input service.AskInput, observer service.ProgressObserver) (service.AskResult, error) {
	return f.ask(ctx, userID, conversationID, input, observer)
}

func TestConversationEndpointRequiresUserContext(t *testing.T) {
	server := newTestServer(t, fakeQAService{create: func(context.Context, string, string) (service.Conversation, error) {
		t.Fatal("service should not be called")
		return service.Conversation{}, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/qa-sessions", strings.NewReader(`{"title":"test"}`))
	request.Header.Set("X-Service-Token", "test-service-token")
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"code":"unauthorized"`) {
		t.Fatalf("unexpected error body: %s", recorder.Body.String())
	}
}

func TestAPIRequiresServiceToken(t *testing.T) {
	server := newTestServer(t, fakeQAService{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/v1/qa-sessions", nil)
	request.Header.Set("X-User-Id", "forged-user")
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"code":"unauthorized"`) {
		t.Fatalf("unexpected error body: %s", recorder.Body.String())
	}
}

func TestCreateConversationMatchesContract(t *testing.T) {
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	server := newTestServer(t, fakeQAService{create: func(_ context.Context, userID, title string) (service.Conversation, error) {
		if userID != "user-1" || title != "锅炉咨询" {
			t.Fatalf("unexpected input: user=%q title=%q", userID, title)
		}
		return service.Conversation{ID: "conversation-id", Title: title, Status: "active", CreatedAt: now, UpdatedAt: now}, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/qa-sessions", strings.NewReader(`{"title":"锅炉咨询"}`))
	request.Header.Set("X-User-Id", "user-1")
	request.Header.Set("X-Service-Token", "test-service-token")
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data      service.Conversation `json:"data"`
		RequestID string               `json:"requestId"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	conversation := body.Data
	if conversation.ID != "conversation-id" || conversation.Title != "锅炉咨询" {
		t.Fatalf("unexpected response: %+v", conversation)
	}
}

func TestCreateConversationAllowsEmptyBody(t *testing.T) {
	server := newTestServer(t, fakeQAService{create: func(_ context.Context, _, title string) (service.Conversation, error) {
		if title != "" {
			t.Fatalf("title=%q", title)
		}
		return service.Conversation{ID: "conversation-id", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/qa-sessions", nil)
	request.Header.Set("X-User-Id", "user-1")
	request.Header.Set("X-Service-Token", "test-service-token")
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestListConversationUsesDocumentedQueryParameters(t *testing.T) {
	server := newTestServer(t, fakeQAService{list: func(_ context.Context, userID string, options service.ConversationListOptions) (service.Page[service.Conversation], error) {
		if userID != "user-1" {
			t.Fatalf("userID=%q", userID)
		}
		want := service.ConversationListOptions{Page: 2, PageSize: 5, Status: "archived", Sort: "createdAt"}
		if options != want {
			t.Fatalf("options=%+v want %+v", options, want)
		}
		return service.Page[service.Conversation]{Items: []service.Conversation{{ID: "conversation-id", Status: "archived"}}, Page: options.Page, PageSize: options.PageSize, Total: 1}, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/v1/qa-sessions?page=2&pageSize=5&status=archived&sort=createdAt", nil)
	request.Header.Set("X-User-Id", "user-1")
	request.Header.Set("X-Service-Token", "test-service-token")
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"pageSize":5`) {
		t.Fatalf("unexpected page response: %s", recorder.Body.String())
	}
}

func TestStreamUsesContractEventNames(t *testing.T) {
	server := newTestServer(t, fakeQAService{ask: func(_ context.Context, _, _ string, input service.AskInput, observer service.ProgressObserver) (service.AskResult, error) {
		if input.Message != "检查要求" {
			t.Fatalf("unexpected input: %+v", input)
		}
		observer(service.ProgressEvent{Type: "message.created", Sequence: 1, Payload: map[string]any{"userMessageId": "user-message"}})
		observer(service.ProgressEvent{Type: "reasoning.step", Sequence: 2, Payload: map[string]any{"detail": "检索完成"}})
		observer(service.ProgressEvent{Type: "answer.delta", Sequence: 3, Payload: map[string]any{"text": "回答内容"}})
		observer(service.ProgressEvent{Type: "answer.completed", Sequence: 4, Payload: map[string]any{"messageId": "assistant-message"}})
		return service.AskResult{AssistantMessage: service.Message{ID: "assistant-message", Content: "回答内容", Status: "completed"}}, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/qa-sessions/conversation-id/messages", strings.NewReader(`{"message":"检查要求","mode":"knowledge_qa"}`))
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("X-User-Id", "user-1")
	request.Header.Set("X-Service-Token", "test-service-token")
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("unexpected stream response: status=%d content-type=%q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	body := recorder.Body.String()
	for _, event := range []string{"message.created", "reasoning.step", "answer.delta", "answer.completed"} {
		if !strings.Contains(body, "event: "+event) {
			t.Fatalf("missing event %q in %s", event, body)
		}
	}
}

func TestStreamDoesNotDuplicateServiceErrorEvent(t *testing.T) {
	server := newTestServer(t, fakeQAService{ask: func(_ context.Context, _, _ string, _ service.AskInput, observer service.ProgressObserver) (service.AskResult, error) {
		observer(service.ProgressEvent{Type: "error", Sequence: 2, Payload: map[string]any{"code": "dependency_error", "message": "answer generation failed"}})
		return service.AskResult{}, service.NewError(service.CodeDependency, "answer generation failed", nil)
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/qa-sessions/session-id/messages", strings.NewReader(`{"message":"question"}`))
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("X-User-Id", "user-1")
	request.Header.Set("X-Service-Token", "test-service-token")
	server.ServeHTTP(recorder, request)
	if count := strings.Count(recorder.Body.String(), "event: error"); count != 1 {
		t.Fatalf("error event count=%d body=%s", count, recorder.Body.String())
	}
}

func TestListMessagesUsesDocumentedQueryParameters(t *testing.T) {
	server := newTestServer(t, fakeQAService{listMessages: func(_ context.Context, userID, sessionID string, options service.MessageListOptions) (service.Page[service.Message], error) {
		if userID != "user-1" || sessionID != "session-1" {
			t.Fatalf("userID=%q sessionID=%q", userID, sessionID)
		}
		want := service.MessageListOptions{Page: 3, PageSize: 10, IncludeThinking: false, IncludeCitations: false}
		if options != want {
			t.Fatalf("options=%+v want %+v", options, want)
		}
		return service.Page[service.Message]{Items: []service.Message{{ID: "message-id", ConversationID: sessionID, Role: "user", Status: "completed", Content: "question"}}, Page: options.Page, PageSize: options.PageSize, Total: 1}, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/v1/qa-sessions/session-1/messages?page=3&pageSize=10&includeThinking=false&includeCitations=false", nil)
	request.Header.Set("X-User-Id", "user-1")
	request.Header.Set("X-Service-Token", "test-service-token")
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"pageSize":10`) {
		t.Fatalf("unexpected page response: %s", recorder.Body.String())
	}
}

func TestListMessagesDefaultsIncludeParametersToTrue(t *testing.T) {
	server := newTestServer(t, fakeQAService{listMessages: func(_ context.Context, _, _ string, options service.MessageListOptions) (service.Page[service.Message], error) {
		if !options.IncludeThinking || !options.IncludeCitations {
			t.Fatalf("include defaults = %+v", options)
		}
		return service.Page[service.Message]{Items: []service.Message{}, Page: options.Page, PageSize: options.PageSize}, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/v1/qa-sessions/session-1/messages", nil)
	request.Header.Set("X-User-Id", "user-1")
	request.Header.Set("X-Service-Token", "test-service-token")
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestListMessagesRejectsInvalidIncludeParameter(t *testing.T) {
	server := newTestServer(t, fakeQAService{listMessages: func(context.Context, string, string, service.MessageListOptions) (service.Page[service.Message], error) {
		t.Fatal("service should not be called")
		return service.Page[service.Message]{}, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/v1/qa-sessions/session-1/messages?includeThinking=maybe", nil)
	request.Header.Set("X-User-Id", "user-1")
	request.Header.Set("X-Service-Token", "test-service-token")
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"includeThinking":"must be a boolean"`) {
		t.Fatalf("unexpected error response: %s", recorder.Body.String())
	}
}

func TestListMessagesPropagatesCrossUserForbidden(t *testing.T) {
	server := newTestServer(t, fakeQAService{listMessages: func(context.Context, string, string, service.MessageListOptions) (service.Page[service.Message], error) {
		return service.Page[service.Message]{}, service.NewError(service.CodeForbidden, "conversation access denied", nil)
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/v1/qa-sessions/other-session/messages", nil)
	request.Header.Set("X-User-Id", "user-1")
	request.Header.Set("X-Service-Token", "test-service-token")
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"forbidden"`) {
		t.Fatalf("unexpected error response: %s", recorder.Body.String())
	}
}

func TestSessionOperationsReturnForbiddenForNonOwnerEvenWithAdminRole(t *testing.T) {
	forbidden := service.NewError(service.CodeForbidden, "conversation access denied", nil)
	qa := fakeQAService{
		get: func(context.Context, string, string) (service.Conversation, error) {
			return service.Conversation{}, forbidden
		},
		update: func(context.Context, string, string, string, string) (service.Conversation, error) {
			return service.Conversation{}, forbidden
		},
		delete: func(context.Context, string, string) error {
			return forbidden
		},
	}
	tests := []struct {
		name   string
		method string
		body   string
	}{
		{name: "detail", method: http.MethodGet},
		{name: "update", method: http.MethodPatch, body: `{"title":"private"}`},
		{name: "delete", method: http.MethodDelete},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(t, qa)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, "/internal/v1/qa-sessions/other-session", strings.NewReader(test.body))
			request.Header.Set("X-User-Id", "user-1")
			request.Header.Set("X-User-Roles", "admin")
			request.Header.Set("X-Service-Token", "test-service-token")
			server.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"code":"forbidden"`) {
				t.Fatalf("unexpected error response: %s", recorder.Body.String())
			}
		})
	}
}

func newTestServer(t *testing.T, qa fakeQAService) *Server {
	t.Helper()
	if qa.create == nil {
		qa.create = func(context.Context, string, string) (service.Conversation, error) {
			return service.Conversation{}, nil
		}
	}
	if qa.ask == nil {
		qa.ask = func(context.Context, string, string, service.AskInput, service.ProgressObserver) (service.AskResult, error) {
			return service.AskResult{}, nil
		}
	}
	server, err := NewServer(qa, fakeSettingsService{}, fakeResourceService{}, Config{MaxRequestBytes: 4096, ServiceToken: "test-service-token"})
	if err != nil {
		t.Fatal(err)
	}
	return server
}
