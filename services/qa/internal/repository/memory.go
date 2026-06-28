package repository

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrMessageNotFound      = errors.New("message not found")
)

type MemoryStore struct {
	mu            sync.RWMutex
	conversations map[string]Conversation
	records       map[string][]MessageRecord
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		conversations: make(map[string]Conversation),
		records:       make(map[string][]MessageRecord),
	}
}

func (s *MemoryStore) SaveConversation(conversation Conversation) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if conversation.CreatedAt.IsZero() {
		conversation.CreatedAt = now
	}
	if conversation.UpdatedAt.IsZero() {
		conversation.UpdatedAt = now
	}
	s.conversations[conversation.ID] = conversation
}

func (s *MemoryStore) GetConversation(ctx context.Context, conversationID string) (Conversation, error) {
	if err := ctx.Err(); err != nil {
		return Conversation{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	conversation, ok := s.conversations[conversationID]
	if !ok {
		return Conversation{}, ErrConversationNotFound
	}
	return conversation, nil
}

func (s *MemoryStore) ListMessageRecords(ctx context.Context, conversationID string) ([]MessageRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.conversations[conversationID]; !ok {
		return nil, ErrConversationNotFound
	}

	source := s.records[conversationID]
	records := make([]MessageRecord, len(source))
	copy(records, source)
	sortRecords(records)
	return records, nil
}

func (s *MemoryStore) NextSequence(ctx context.Context, conversationID string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.conversations[conversationID]; !ok {
		return 0, ErrConversationNotFound
	}

	next := 1
	for _, record := range s.records[conversationID] {
		if record.Message.SequenceNo >= next {
			next = record.Message.SequenceNo + 1
		}
	}
	return next, nil
}

func (s *MemoryStore) AppendMessage(ctx context.Context, record MessageRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	conversation, ok := s.conversations[record.Message.ConversationID]
	if !ok {
		return ErrConversationNotFound
	}

	s.records[record.Message.ConversationID] = append(s.records[record.Message.ConversationID], record)
	conversation.UpdatedAt = time.Now().UTC()
	s.conversations[conversation.ID] = conversation
	return nil
}

func (s *MemoryStore) SeedRecords(conversation Conversation, records ...MessageRecord) {
	s.SaveConversation(conversation)

	s.mu.Lock()
	defer s.mu.Unlock()

	copied := make([]MessageRecord, len(records))
	copy(copied, records)
	sortRecords(copied)
	s.records[conversation.ID] = copied
}

func sortRecords(records []MessageRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		left := records[i].Message
		right := records[j].Message
		if left.SequenceNo == right.SequenceNo {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		return left.SequenceNo < right.SequenceNo
	})
}
