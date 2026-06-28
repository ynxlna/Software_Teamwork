package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/repository"
)

type ConversationStore interface {
	GetConversation(ctx context.Context, conversationID string) (repository.Conversation, error)
	ListMessageRecords(ctx context.Context, conversationID string) ([]repository.MessageRecord, error)
	NextSequence(ctx context.Context, conversationID string) (int, error)
	AppendMessage(ctx context.Context, record repository.MessageRecord) error
}

type ContextLimits struct {
	MaxMessages int
	MaxChars    int
}

type ConversationService struct {
	store  ConversationStore
	limits ContextLimits
}

func NewConversationService(store ConversationStore, limits ContextLimits) *ConversationService {
	if limits.MaxMessages <= 0 {
		limits.MaxMessages = 20
	}
	if limits.MaxChars <= 0 {
		limits.MaxChars = 8000
	}
	return &ConversationService{store: store, limits: limits}
}

func (s *ConversationService) GetHistory(ctx context.Context, userID, conversationID string) (ConversationHistory, error) {
	if strings.TrimSpace(userID) == "" {
		return ConversationHistory{}, NewError(CodeUnauthorized, "authentication required", nil)
	}
	if strings.TrimSpace(conversationID) == "" {
		return ConversationHistory{}, NewError(CodeValidation, "conversation_id is required", nil)
	}

	if err := s.authorize(ctx, userID, conversationID); err != nil {
		return ConversationHistory{}, err
	}

	records, err := s.store.ListMessageRecords(ctx, conversationID)
	if err != nil {
		return ConversationHistory{}, mapRepositoryError(err)
	}
	sortMessageRecords(records)

	messages := make([]MessageDTO, 0, len(records))
	for _, record := range records {
		messages = append(messages, mapMessage(record))
	}

	return ConversationHistory{
		ConversationID: conversationID,
		Messages:       messages,
	}, nil
}

func (s *ConversationService) BuildContext(ctx context.Context, userID, conversationID, currentMessage string) (ModelContext, error) {
	if strings.TrimSpace(currentMessage) == "" {
		return ModelContext{}, NewError(CodeValidation, "message is required", nil)
	}

	history, err := s.GetHistory(ctx, userID, conversationID)
	if err != nil {
		return ModelContext{}, err
	}

	messages := make([]ContextMessage, 0, len(history.Messages)+1)
	for _, message := range history.Messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		messages = append(messages, ContextMessage{Role: message.Role, Content: content})
	}
	messages = append(messages, ContextMessage{Role: RoleUser, Content: currentMessage})

	bounded, truncated := s.boundContext(messages)
	return ModelContext{
		ConversationID: conversationID,
		Messages:       bounded,
		Truncated:      truncated,
	}, nil
}

func (s *ConversationService) AcceptCurrentMessage(ctx context.Context, userID string, request StreamRequest) (StreamAccepted, error) {
	if strings.TrimSpace(request.ConversationID) == "" {
		return StreamAccepted{}, NewError(CodeValidation, "conversation_id is required", nil)
	}
	if strings.TrimSpace(request.Message) == "" {
		return StreamAccepted{}, NewError(CodeValidation, "message is required", nil)
	}

	modelContext, err := s.BuildContext(ctx, userID, request.ConversationID, request.Message)
	if err != nil {
		return StreamAccepted{}, err
	}

	sequenceNo, err := s.store.NextSequence(ctx, request.ConversationID)
	if err != nil {
		return StreamAccepted{}, mapRepositoryError(err)
	}

	messageID := NextMessageID(request.ConversationID, sequenceNo)
	record := NewUserMessageRecord(request.ConversationID, messageID, request.Message, sequenceNo)
	if err := s.store.AppendMessage(ctx, record); err != nil {
		return StreamAccepted{}, mapRepositoryError(err)
	}

	return StreamAccepted{
		ConversationID:      modelContext.ConversationID,
		ContextMessageCount: len(modelContext.Messages),
		Truncated:           modelContext.Truncated,
	}, nil
}

func (s *ConversationService) authorize(ctx context.Context, userID, conversationID string) error {
	conversation, err := s.store.GetConversation(ctx, conversationID)
	if err != nil {
		return mapRepositoryError(err)
	}
	if conversation.OwnerID != userID {
		return NewError(CodeForbidden, "conversation access denied", nil)
	}
	return nil
}

func (s *ConversationService) boundContext(messages []ContextMessage) ([]ContextMessage, bool) {
	truncated := false
	if len(messages) > s.limits.MaxMessages {
		messages = messages[len(messages)-s.limits.MaxMessages:]
		truncated = true
	}

	total := 0
	start := len(messages) - 1
	for i := len(messages) - 1; i >= 0; i-- {
		nextTotal := total + len([]rune(messages[i].Content))
		if nextTotal > s.limits.MaxChars && i != len(messages)-1 {
			truncated = true
			break
		}
		total = nextTotal
		start = i
	}

	if start > 0 {
		return messages[start:], true
	}
	return messages, truncated
}

func mapRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrConversationNotFound):
		return NewError(CodeNotFound, "conversation not found", err)
	default:
		return NewError(CodeInternal, "conversation storage failed", err)
	}
}

func mapMessage(record repository.MessageRecord) MessageDTO {
	blocks := make([]ContentBlockDTO, len(record.ContentBlocks))
	for i, block := range record.ContentBlocks {
		blocks[i] = ContentBlockDTO{
			ID:         block.ID,
			Type:       block.Type,
			Text:       block.Text,
			SequenceNo: block.SequenceNo,
		}
	}
	sort.SliceStable(blocks, func(i, j int) bool {
		return blocks[i].SequenceNo < blocks[j].SequenceNo
	})

	citations := make([]CitationDTO, len(record.Citations))
	for i, citation := range record.Citations {
		citations[i] = CitationDTO{
			ID:          citation.ID,
			DocumentID:  citation.DocumentID,
			ChunkID:     citation.ChunkID,
			SourceTitle: citation.SourceTitle,
			SourceText:  citation.SourceText,
			Score:       citation.Score,
			SequenceNo:  citation.SequenceNo,
		}
	}
	sort.SliceStable(citations, func(i, j int) bool {
		return citations[i].SequenceNo < citations[j].SequenceNo
	})

	thinking := make([]ProcessStepDTO, len(record.ProcessSteps))
	for i, step := range record.ProcessSteps {
		thinking[i] = ProcessStepDTO{
			ID:         step.ID,
			Title:      step.Title,
			Status:     step.Status,
			Detail:     step.Detail,
			SequenceNo: step.SequenceNo,
			Timestamp:  step.CreatedAt,
		}
	}
	sort.SliceStable(thinking, func(i, j int) bool {
		return thinking[i].SequenceNo < thinking[j].SequenceNo
	})

	content := strings.TrimSpace(record.Message.Content)
	if content == "" {
		content = joinTextBlocks(blocks)
	}

	return MessageDTO{
		ID:            record.Message.ID,
		Role:          record.Message.Role,
		Status:        record.Message.Status,
		SequenceNo:    record.Message.SequenceNo,
		Content:       content,
		ContentBlocks: blocks,
		Thinking:      thinking,
		Citations:     citations,
		ErrorCode:     record.Message.ErrorCode,
		Timestamp:     record.Message.CreatedAt,
	}
}

func joinTextBlocks(blocks []ContentBlockDTO) string {
	var text []string
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			text = append(text, block.Text)
		}
	}
	return strings.Join(text, "")
}

func sortMessageRecords(records []repository.MessageRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		left := records[i].Message
		right := records[j].Message
		if left.SequenceNo == right.SequenceNo {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		return left.SequenceNo < right.SequenceNo
	})
}

func NewUserMessageRecord(conversationID, messageID, content string, sequenceNo int) repository.MessageRecord {
	return repository.MessageRecord{
		Message: repository.Message{
			ID:             messageID,
			ConversationID: conversationID,
			Role:           RoleUser,
			Status:         StatusCompleted,
			SequenceNo:     sequenceNo,
			Content:        content,
			CreatedAt:      time.Now().UTC(),
		},
	}
}

func NextMessageID(conversationID string, sequenceNo int) string {
	return fmt.Sprintf("%s-msg-%d", conversationID, sequenceNo)
}
