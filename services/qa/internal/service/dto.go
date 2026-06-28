package service

import "time"

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"

	StatusCompleted = "completed"
	StatusStopped   = "stopped"
	StatusFailed    = "failed"
)

type ConversationHistory struct {
	ConversationID string       `json:"conversation_id"`
	Messages       []MessageDTO `json:"messages"`
}

type MessageDTO struct {
	ID            string            `json:"id"`
	Role          string            `json:"role"`
	Status        string            `json:"status"`
	SequenceNo    int               `json:"sequence_no"`
	Content       string            `json:"content"`
	ContentBlocks []ContentBlockDTO `json:"content_blocks"`
	Thinking      []ProcessStepDTO  `json:"thinking"`
	Citations     []CitationDTO     `json:"citations"`
	ErrorCode     string            `json:"error_code,omitempty"`
	Timestamp     time.Time         `json:"timestamp"`
}

type ContentBlockDTO struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Text       string `json:"text"`
	SequenceNo int    `json:"sequence_no"`
}

type ProcessStepDTO struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	Detail     string    `json:"detail,omitempty"`
	SequenceNo int       `json:"sequence_no"`
	Timestamp  time.Time `json:"timestamp"`
}

type CitationDTO struct {
	ID          string  `json:"id"`
	DocumentID  string  `json:"document_id"`
	ChunkID     string  `json:"chunk_id"`
	SourceTitle string  `json:"source_title"`
	SourceText  string  `json:"source_text"`
	Score       float64 `json:"score"`
	SequenceNo  int     `json:"sequence_no"`
}

type ContextMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ModelContext struct {
	ConversationID string           `json:"conversation_id"`
	Messages       []ContextMessage `json:"messages"`
	Truncated      bool             `json:"truncated"`
}

type StreamRequest struct {
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
}

type StreamAccepted struct {
	ConversationID      string `json:"conversation_id"`
	ContextMessageCount int    `json:"context_message_count"`
	Truncated           bool   `json:"truncated"`
}
