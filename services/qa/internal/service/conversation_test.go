package service

import (
	"context"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/repository"
)

func TestGetHistorySortsBySequenceAndIncludesPartialMessages(t *testing.T) {
	t.Parallel()

	store := repository.NewMemoryStore()
	now := time.Date(2026, 6, 28, 1, 2, 3, 0, time.UTC)
	store.SeedRecords(repository.Conversation{ID: "conv_1", OwnerID: "user_1"}, repository.MessageRecord{
		Message: repository.Message{
			ID:             "msg_3",
			ConversationID: "conv_1",
			Role:           RoleAssistant,
			Status:         StatusFailed,
			SequenceNo:     3,
			ErrorCode:      "model_timeout",
			CreatedAt:      now.Add(3 * time.Minute),
		},
		ContentBlocks: []repository.ContentBlock{
			{ID: "block_2", Type: "text", Text: "answer", SequenceNo: 2},
			{ID: "block_1", Type: "text", Text: "partial ", SequenceNo: 1},
		},
		Citations: []repository.Citation{
			{ID: "citation_1", DocumentID: "doc_1", ChunkID: "chunk_1", SourceTitle: "Guide", SourceText: "source", Score: 0.91, SequenceNo: 1},
		},
		ProcessSteps: []repository.ProcessStep{
			{ID: "step_2", Title: "Generate", Status: StatusFailed, Detail: "timeout", SequenceNo: 2, CreatedAt: now.Add(2 * time.Minute)},
			{ID: "step_1", Title: "Retrieve", Status: StatusCompleted, SequenceNo: 1, CreatedAt: now.Add(time.Minute)},
		},
	}, repository.MessageRecord{
		Message: repository.Message{
			ID:             "msg_1",
			ConversationID: "conv_1",
			Role:           RoleUser,
			Status:         StatusCompleted,
			SequenceNo:     1,
			Content:        "question",
			CreatedAt:      now,
		},
	}, repository.MessageRecord{
		Message: repository.Message{
			ID:             "msg_2",
			ConversationID: "conv_1",
			Role:           RoleAssistant,
			Status:         StatusStopped,
			SequenceNo:     2,
			Content:        "stopped answer",
			CreatedAt:      now.Add(2 * time.Minute),
		},
	})

	svc := NewConversationService(store, ContextLimits{MaxMessages: 20, MaxChars: 8000})
	history, err := svc.GetHistory(context.Background(), "user_1", "conv_1")
	if err != nil {
		t.Fatalf("GetHistory returned error: %v", err)
	}

	if got, want := len(history.Messages), 3; got != want {
		t.Fatalf("message count = %d, want %d", got, want)
	}
	if got := history.Messages[0].ID; got != "msg_1" {
		t.Fatalf("first message = %s, want msg_1", got)
	}
	if got := history.Messages[1].Status; got != StatusStopped {
		t.Fatalf("second status = %s, want stopped", got)
	}
	failed := history.Messages[2]
	if failed.Status != StatusFailed {
		t.Fatalf("third status = %s, want failed", failed.Status)
	}
	if failed.Content != "partial answer" {
		t.Fatalf("failed content = %q, want joined partial answer", failed.Content)
	}
	if failed.ErrorCode != "model_timeout" {
		t.Fatalf("error code = %q, want model_timeout", failed.ErrorCode)
	}
	if len(failed.Thinking) != 2 || failed.Thinking[0].ID != "step_1" {
		t.Fatalf("thinking steps were not sorted and preserved: %#v", failed.Thinking)
	}
	if len(failed.Citations) != 1 || failed.Citations[0].DocumentID != "doc_1" {
		t.Fatalf("citations were not preserved: %#v", failed.Citations)
	}
}

func TestGetHistoryRejectsCrossUserAccess(t *testing.T) {
	t.Parallel()

	store := repository.NewMemoryStore()
	store.SeedRecords(repository.Conversation{ID: "conv_1", OwnerID: "user_1"})
	svc := NewConversationService(store, ContextLimits{})

	_, err := svc.GetHistory(context.Background(), "user_2", "conv_1")
	if err == nil {
		t.Fatal("GetHistory should reject cross-user access")
	}
	appErr, ok := err.(*AppError)
	if !ok {
		t.Fatalf("error type = %T, want *AppError", err)
	}
	if appErr.Code != CodeForbidden {
		t.Fatalf("error code = %s, want forbidden", appErr.Code)
	}
}

func TestBuildContextUsesStoredHistoryAndCurrentMessageWithBounds(t *testing.T) {
	t.Parallel()

	store := repository.NewMemoryStore()
	store.SeedRecords(repository.Conversation{ID: "conv_1", OwnerID: "user_1"}, repository.MessageRecord{
		Message: repository.Message{ID: "msg_1", ConversationID: "conv_1", Role: RoleUser, Status: StatusCompleted, SequenceNo: 1, Content: "old"},
	}, repository.MessageRecord{
		Message: repository.Message{ID: "msg_2", ConversationID: "conv_1", Role: RoleAssistant, Status: StatusCompleted, SequenceNo: 2, Content: "answer"},
	})
	svc := NewConversationService(store, ContextLimits{MaxMessages: 2, MaxChars: 100})

	modelContext, err := svc.BuildContext(context.Background(), "user_1", "conv_1", "new question")
	if err != nil {
		t.Fatalf("BuildContext returned error: %v", err)
	}
	if !modelContext.Truncated {
		t.Fatal("context should be truncated by MaxMessages")
	}
	if got, want := len(modelContext.Messages), 2; got != want {
		t.Fatalf("context message count = %d, want %d", got, want)
	}
	if modelContext.Messages[0].Content != "answer" || modelContext.Messages[1].Content != "new question" {
		t.Fatalf("unexpected bounded context: %#v", modelContext.Messages)
	}
}
