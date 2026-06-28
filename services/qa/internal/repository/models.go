package repository

import (
	"context"
	"time"
)

type Store interface {
	GetConversation(ctx context.Context, conversationID string) (Conversation, error)
	ListMessageRecords(ctx context.Context, conversationID string) ([]MessageRecord, error)
	NextSequence(ctx context.Context, conversationID string) (int, error)
	AppendMessage(ctx context.Context, record MessageRecord) error
}

type Conversation struct {
	ID        string
	OwnerID   string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Message struct {
	ID             string
	ConversationID string
	Role           string
	Status         string
	SequenceNo     int
	Content        string
	ErrorCode      string
	CreatedAt      time.Time
}

type ContentBlock struct {
	ID         string
	MessageID  string
	Type       string
	Text       string
	SequenceNo int
}

type Citation struct {
	ID          string
	MessageID   string
	DocumentID  string
	ChunkID     string
	SourceTitle string
	SourceText  string
	Score       float64
	SequenceNo  int
}

type ResponseRun struct {
	ID        string
	MessageID string
	Status    string
	ErrorCode string
	StartedAt time.Time
	EndedAt   time.Time
}

type ProcessStep struct {
	ID            string
	ResponseRunID string
	Title         string
	Status        string
	Detail        string
	SequenceNo    int
	CreatedAt     time.Time
}

type MessageRecord struct {
	Message       Message
	ContentBlocks []ContentBlock
	Citations     []Citation
	ResponseRun   *ResponseRun
	ProcessSteps  []ProcessStep
}
