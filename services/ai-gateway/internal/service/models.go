package service

import (
	"context"
	"encoding/json"
	"io"
	"time"
)

type Purpose string

const (
	PurposeChat      Purpose = "chat"
	PurposeEmbedding Purpose = "embedding"
	PurposeRerank    Purpose = "rerank"
)

type Provider string

const (
	ProviderOpenAICompatible Provider = "openai_compatible"
	ProviderSiliconFlow      Provider = "siliconflow"
	ProviderLocalCompatible  Provider = "local_compatible"
)

type CredentialStatus string

const (
	CredentialActive   CredentialStatus = "active"
	CredentialRotated  CredentialStatus = "rotated"
	CredentialDisabled CredentialStatus = "disabled"
)

type RevisionChangeType string

const (
	RevisionCreated           RevisionChangeType = "created"
	RevisionUpdated           RevisionChangeType = "updated"
	RevisionCredentialRotated RevisionChangeType = "credential_rotated"
	RevisionDeleted           RevisionChangeType = "deleted"
)

type RequestContext struct {
	RequestID     string
	CallerService string
	UserID        string
}

type ModelProfile struct {
	ID                string
	Name              string
	Purpose           Purpose
	Provider          Provider
	BaseURL           string
	Model             string
	Enabled           bool
	IsDefault         bool
	TimeoutMS         int
	APIKeyConfigured  bool
	SupportsStreaming bool
	Dimensions        *int
	TopN              *int
	DefaultParameters json.RawMessage
	CredentialID      string
	CreatedByUserID   string
	UpdatedByUserID   string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

type ProviderCredential struct {
	ID                   string
	ProfileID            string
	StorageMode          string
	Ciphertext           []byte
	Nonce                []byte
	EncryptionKeyVersion string
	FingerprintSHA256    string
	KeyLast4             string
	Status               CredentialStatus
	CreatedByUserID      string
	CreatedAt            time.Time
	RotatedAt            *time.Time
	DisabledAt           *time.Time
	DeletedAt            *time.Time
}

type ModelProfileRevision struct {
	ID                 string
	ProfileID          string
	RevisionNo         int
	ChangeType         RevisionChangeType
	ChangedFieldsJSON  json.RawMessage
	BeforeSnapshotJSON json.RawMessage
	AfterSnapshotJSON  json.RawMessage
	ChangedByUserID    string
	CallerService      string
	RequestID          string
	CreatedAt          time.Time
}

type ListModelProfilesFilter struct {
	Purpose *Purpose
	Enabled *bool
}

type CreateModelProfileInput struct {
	ID                string
	Name              string
	Purpose           Purpose
	Provider          Provider
	BaseURL           string
	Model             string
	APIKey            string
	Enabled           *bool
	IsDefault         *bool
	TimeoutMS         *int
	SupportsStreaming *bool
	Dimensions        *int
	TopN              *int
	DefaultParameters json.RawMessage
}

type UpdateModelProfileInput struct {
	ID                string
	Name              *string
	Provider          *Provider
	BaseURL           *string
	Model             *string
	APIKey            *string
	Enabled           *bool
	IsDefault         *bool
	TimeoutMS         *int
	SupportsStreaming *bool
	Dimensions        *int
	TopN              *int
	DefaultParameters *json.RawMessage
}

type ReadinessCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Readiness struct {
	Status string           `json:"status"`
	Checks []ReadinessCheck `json:"checks"`
}

type ChatCompletionInput struct {
	RequestContext RequestContext
	Payload        map[string]json.RawMessage
}

type ChatCompletionResult struct {
	Body json.RawMessage
}

type ChatCompletionStream struct {
	Body     io.ReadCloser
	Finalize func(StreamFinalizeInput) error
}

type StreamFinalizeInput struct {
	Status             InvocationStatus
	ProviderStatusCode *int
	Error              *OpenAIError
	Usage              *TokenUsage
}

type ProviderChatRequest struct {
	Profile   ModelProfile
	APIKey    string
	Payload   map[string]json.RawMessage
	Stream    bool
	RequestID string
}

type ProviderChatResult struct {
	Body               json.RawMessage
	Usage              *TokenUsage
	ProviderStatusCode int
}

type ProviderChatStream struct {
	Body               io.ReadCloser
	ProviderStatusCode int
}

type ChatProvider interface {
	CompleteChat(context.Context, ProviderChatRequest) (ProviderChatResult, error)
	StreamChat(context.Context, ProviderChatRequest) (ProviderChatStream, error)
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

const (
	OperationEmbedding = "embedding"
	OperationReranking = "reranking"
)

type EmbeddingInput struct {
	Model          string
	ProfileID      string
	Input          []string
	Dimensions     *int
	EncodingFormat string
	User           string
}

type EmbeddingVector struct {
	Object    string          `json:"object"`
	Index     int             `json:"index"`
	Embedding json.RawMessage `json:"embedding"`
}

type EmbeddingResponse struct {
	Object string            `json:"object"`
	Data   []EmbeddingVector `json:"data"`
	Model  string            `json:"model"`
	Usage  *TokenUsage       `json:"usage,omitempty"`
}

type RerankingDocument struct {
	ID   string
	Text string
}

type RerankingInput struct {
	Model     string
	ProfileID string
	Query     string
	Documents []RerankingDocument
	TopN      *int
	Metadata  map[string]string
}

type RerankingResult struct {
	Index      int     `json:"index"`
	DocumentID string  `json:"document_id"`
	Score      float64 `json:"score"`
}

type RerankingResponse struct {
	Object string            `json:"object"`
	Data   []RerankingResult `json:"data"`
	Model  string            `json:"model"`
	Usage  *TokenUsage       `json:"usage,omitempty"`
}

type ProviderEmbeddingRequest struct {
	RequestID         string
	Provider          Provider
	BaseURL           string
	APIKey            string
	TimeoutMS         int
	Model             string
	Input             []string
	Dimensions        *int
	EncodingFormat    string
	User              string
	DefaultParameters json.RawMessage
}

type ProviderRerankingRequest struct {
	RequestID         string
	Provider          Provider
	BaseURL           string
	APIKey            string
	TimeoutMS         int
	Model             string
	Query             string
	Documents         []RerankingDocument
	TopN              *int
	Metadata          map[string]string
	DefaultParameters json.RawMessage
}

type ProviderCallMetadata struct {
	StatusCode int
}

type ModelInvoker interface {
	CreateEmbeddings(context.Context, ProviderEmbeddingRequest) (EmbeddingResponse, ProviderCallMetadata, error)
	CreateReranking(context.Context, ProviderRerankingRequest) (RerankingResponse, ProviderCallMetadata, error)
}

type InvocationStatus string

const (
	InvocationSucceeded InvocationStatus = "succeeded"
	InvocationFailed    InvocationStatus = "failed"
	InvocationCancelled InvocationStatus = "cancelled"
	InvocationTimeout   InvocationStatus = "timeout"
)

type ProviderInvocation struct {
	ID                  string
	RequestID           string
	CallerService       string
	ExternalUserID      string
	Operation           string
	ProfileID           string
	Provider            Provider
	Model               string
	Stream              bool
	Status              InvocationStatus
	ProviderStatusCode  *int
	PromptTokens        *int
	CompletionTokens    *int
	TotalTokens         *int
	InputCount          *int
	EmbeddingDimensions *int
	RerankTopN          *int
	DurationMS          int64
	AttemptCount        int
	NormalizedErrorCode string
	NormalizedErrorType string
	ErrorMessage        string
	CreatedAt           time.Time
	FinishedAt          time.Time
}

type ProviderInvocationAttempt struct {
	ID                 string
	InvocationID       string
	AttemptNo          int
	Provider           Provider
	BaseURLHost        string
	Model              string
	Status             InvocationStatus
	ProviderStatusCode *int
	DurationMS         int64
	ErrorCode          string
	ErrorMessage       string
	StartedAt          time.Time
	FinishedAt         time.Time
}

type OpenAIError struct {
	HTTPStatus         int
	Message            string
	Type               string
	Param              string
	Code               string
	ProviderStatusCode *int
	Err                error
}

func (e *OpenAIError) Error() string { return e.Message }

func (e *OpenAIError) Unwrap() error { return e.Err }

type Repository interface {
	CheckReady(context.Context) error
	ListModelProfiles(context.Context, ListModelProfilesFilter) ([]ModelProfile, error)
	GetModelProfile(context.Context, string) (ModelProfile, error)
	GetDefaultModelProfile(context.Context, Purpose) (ModelProfile, error)
	GetActiveCredential(context.Context, string) (ProviderCredential, error)
	CreateModelProfile(context.Context, ModelProfile, ProviderCredential, ModelProfileRevision) (ModelProfile, error)
	UpdateModelProfile(context.Context, ModelProfile, *ProviderCredential, ModelProfileRevision) (ModelProfile, error)
	SoftDeleteModelProfile(context.Context, string, time.Time, ModelProfileRevision) error
	RecordProviderInvocation(context.Context, ProviderInvocation, []ProviderInvocationAttempt) error
}
