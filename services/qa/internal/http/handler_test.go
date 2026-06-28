package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
)

func TestGetConversationHistoryHandler(t *testing.T) {
	t.Parallel()

	router := testRouter()
	request := httptest.NewRequest(http.MethodGet, "/api/conversations/conv_1", nil)
	request.Header.Set("X-User-ID", "user_1")
	request.Header.Set("X-Request-ID", "req_test")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}

	var payload service.ConversationHistory
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ConversationID != "conv_1" {
		t.Fatalf("conversation_id = %s, want conv_1", payload.ConversationID)
	}
	if len(payload.Messages) != 1 || payload.Messages[0].Content != "hello" {
		t.Fatalf("unexpected messages: %#v", payload.Messages)
	}
}

func TestGetConversationHistoryHandlerRejectsCrossUser(t *testing.T) {
	t.Parallel()

	router := testRouter()
	request := httptest.NewRequest(http.MethodGet, "/api/conversations/conv_1", nil)
	request.Header.Set("X-User-ID", "user_2")
	request.Header.Set("X-Request-ID", "req_test")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"forbidden"`) {
		t.Fatalf("response should contain forbidden code: %s", response.Body.String())
	}
}

func TestChatStreamAcceptsOnlyCurrentMessage(t *testing.T) {
	t.Parallel()

	router, store := testRouterWithStore()
	body := strings.NewReader(`{"conversation_id":"conv_1","message":"follow up"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/chat/stream", body)
	request.Header.Set("X-User-ID", "user_1")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type = %q, want text/event-stream", got)
	}
	if !strings.Contains(response.Body.String(), "event: step") {
		t.Fatalf("response should contain step event: %s", response.Body.String())
	}

	records, err := store.ListMessageRecords(request.Context(), "conv_1")
	if err != nil {
		t.Fatalf("ListMessageRecords returned error: %v", err)
	}
	if got, want := len(records), 2; got != want {
		t.Fatalf("record count after stream = %d, want %d", got, want)
	}
	if records[1].Message.Content != "follow up" {
		t.Fatalf("persisted current message = %q, want follow up", records[1].Message.Content)
	}
}

func TestChatStreamRejectsFrontendSuppliedHistory(t *testing.T) {
	t.Parallel()

	router := testRouter()
	body := strings.NewReader(`{"conversation_id":"conv_1","message":"follow up","messages":[]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/chat/stream", body)
	request.Header.Set("X-User-ID", "user_1")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"validation_error"`) {
		t.Fatalf("response should contain validation error: %s", response.Body.String())
	}
}

func TestChatStreamRejectsMultipleJSONObjects(t *testing.T) {
	t.Parallel()

	router := testRouter()
	body := strings.NewReader(`{"conversation_id":"conv_1","message":"follow up"}{}`)
	request := httptest.NewRequest(http.MethodPost, "/api/chat/stream", body)
	request.Header.Set("X-User-ID", "user_1")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"validation_error"`) {
		t.Fatalf("response should contain validation error: %s", response.Body.String())
	}
}

func testRouter() http.Handler {
	router, _ := testRouterWithStore()
	return router
}

func testRouterWithStore() (http.Handler, *repository.MemoryStore) {
	store := repository.NewMemoryStore()
	store.SeedRecords(repository.Conversation{ID: "conv_1", OwnerID: "user_1"}, repository.MessageRecord{
		Message: repository.Message{
			ID:             "msg_1",
			ConversationID: "conv_1",
			Role:           service.RoleUser,
			Status:         service.StatusCompleted,
			SequenceNo:     1,
			Content:        "hello",
			CreatedAt:      time.Date(2026, 6, 28, 1, 2, 3, 0, time.UTC),
		},
	})
	conversations := service.NewConversationService(store, service.ContextLimits{MaxMessages: 20, MaxChars: 8000})
	return NewRouter(conversations), store
}
