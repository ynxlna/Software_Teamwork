package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

type Code string

const (
	CodeValidation        Code = "validation_error"
	CodeUnauthorized      Code = "unauthorized"
	CodeForbidden         Code = "forbidden"
	CodeNotFound          Code = "not_found"
	CodeConflict          Code = "conflict"
	CodeDependency        Code = "dependency_error"
	CodeInternal          Code = "internal_error"
	CodeUnsupportedIntent Code = "unsupported_intent"
)

type AppError struct {
	Code    Code
	Message string
	Fields  map[string]string
	Err     error
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Err }

func NewError(code Code, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

func ValidationError(fields map[string]string) *AppError {
	return &AppError{Code: CodeValidation, Message: "request validation failed", Fields: fields}
}

func Classify(err error) (*AppError, bool) {
	var appErr *AppError
	return appErr, errors.As(err, &appErr)
}

type Conversation struct {
	ID                 string     `json:"id"`
	Title              string     `json:"title"`
	OwnerUserID        string     `json:"-"`
	Status             string     `json:"status"`
	MessageCount       int        `json:"messageCount"`
	LastMessagePreview string     `json:"lastMessagePreview,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	LastMessageAt      *time.Time `json:"-"`
}

type ConversationListOptions struct {
	Page     int
	PageSize int
	Status   string
	Sort     string
}

type Message struct {
	ID             string          `json:"id"`
	ConversationID string          `json:"sessionId"`
	SequenceNo     int             `json:"sequenceNo"`
	Role           string          `json:"role"`
	Content        string          `json:"content"`
	Intent         string          `json:"intent,omitempty"`
	Status         string          `json:"status"`
	Thinking       []ReasoningStep `json:"thinking,omitempty"`
	Citations      []Citation      `json:"citations,omitempty"`
	CitationCount  int             `json:"-"`
	CreatedAt      time.Time       `json:"createdAt"`
	CompletedAt    *time.Time      `json:"completedAt,omitempty"`
}

type MessageListOptions struct {
	Page             int
	PageSize         int
	IncludeThinking  bool
	IncludeCitations bool
}

type ReasoningStep struct {
	ID        string    `json:"id,omitempty"`
	MessageID string    `json:"messageId,omitempty"`
	Type      string    `json:"type"`
	Title     string    `json:"title,omitempty"`
	Summary   string    `json:"summary"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

func (s ReasoningStep) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type   string `json:"type"`
		Label  string `json:"label,omitempty"`
		Status string `json:"status"`
		Detail string `json:"detail,omitempty"`
	}{Type: publicStepType(s.Type), Label: s.Title, Status: publicStepStatus(s.Status), Detail: s.Summary})
}

type Page[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

type RetrievalOptions struct {
	TopK            int                 `json:"topK,omitempty"`
	ScoreThreshold  float64             `json:"scoreThreshold,omitempty"`
	RerankThreshold float64             `json:"rerankThreshold,omitempty"`
	EnableRerank    bool                `json:"enableRerank,omitempty"`
	TagFilters      map[string][]string `json:"tagFilters,omitempty"`
}

type AskInput struct {
	Message          string           `json:"message"`
	Mode             string           `json:"mode,omitempty"`
	KnowledgeBaseIDs []string         `json:"knowledgeBaseIds,omitempty"`
	Retrieval        RetrievalOptions `json:"retrieval,omitempty"`
}

type AskResult struct {
	UserMessage      Message         `json:"userMessage"`
	AssistantMessage Message         `json:"assistantMessage"`
	ResponseRun      ResponseRun     `json:"responseRun"`
	Citations        []any           `json:"citations"`
	ReasoningSteps   []ReasoningStep `json:"reasoningSteps"`
}

type ProgressEvent struct {
	Type             string
	Sequence         int
	Payload          map[string]any
	UserMessageID    string
	AssistantMessage string
	Intent           string
	Step             ReasoningStep
}

type ProgressObserver func(ProgressEvent)

type ModelInvocation struct {
	ResponseRunID    string
	IterationNo      int
	Provider         string
	ProfileID        string
	ModelName        string
	FinishReason     string
	Status           string
	PromptTokens     int
	CompletionTokens int
	ReasoningTokens  int
	TotalTokens      int
	LatencyMS        int64
	ErrorCode        string
	ErrorMessage     string
	StartedAt        time.Time
	FinishedAt       *time.Time
}

type ResponseRunStart struct {
	RequestID          string
	QAConfigVersionID  string
	LLMConfigVersionID string
	MaxIterations      int
}

type ResponseRunFinalization struct {
	RunID             string
	AssistantMessage  Message
	ReasoningSteps    []ReasoningStep
	StreamEvents      []StreamEvent
	Status            string
	TerminationReason string
	CurrentIteration  int
	PromptTokens      int
	CompletionTokens  int
	ReasoningTokens   int
	TotalTokens       int
	CompletedAt       time.Time
}

type Repository interface {
	CreateConversation(context.Context, Conversation) (Conversation, error)
	ListConversations(context.Context, string, ConversationListOptions) (Page[Conversation], error)
	GetConversation(context.Context, string, string) (Conversation, error)
	UpdateConversation(context.Context, string, Conversation) (Conversation, error)
	DeleteConversation(context.Context, string, string) error
	ListMessages(context.Context, string, string, MessageListOptions) (Page[Message], error)
	AppendMessages(context.Context, string, string, ResponseRunStart, ...Message) (ResponseRun, error)
	UpdateMessage(context.Context, string, Message) error
	FinalizeResponseRun(context.Context, string, ResponseRunFinalization) (ResponseRun, error)
	SaveReasoningSteps(context.Context, string, string, []ReasoningStep) error
	SaveStreamEvents(context.Context, string, string, []StreamEvent) error
	SaveModelInvocation(context.Context, string, ModelInvocation) (string, error)
	GetResponseRun(context.Context, string, string) (ResponseRun, error)
}

type AgentRunner interface {
	RunWithObserver(context.Context, []agent.Message, agent.Observer) (agent.Result, error)
}

type RuntimeSnapshot struct {
	Runner             AgentRunner
	SystemPrompt       string
	LLMModel           string
	LLMProfileID       string
	QAConfigVersionID  string
	LLMConfigVersionID string
	MaxIterations      int
	OverallTimeout     time.Duration
}

type RuntimeProvider interface {
	Acquire() (RuntimeSnapshot, func(), error)
}

type QAService struct {
	repository Repository
	runtime    RuntimeProvider
	now        func() time.Time
	activeMu   sync.Mutex
	activeRuns map[string]context.CancelFunc
}

func NewQAService(repository Repository, runtime RuntimeProvider) (*QAService, error) {
	if repository == nil || runtime == nil {
		return nil, errors.New("repository and runtime provider are required")
	}
	return &QAService{repository: repository, runtime: runtime, now: time.Now, activeRuns: map[string]context.CancelFunc{}}, nil
}

func (s *QAService) CreateConversation(ctx context.Context, userID, title string) (Conversation, error) {
	if strings.TrimSpace(userID) == "" {
		return Conversation{}, NewError(CodeUnauthorized, "authentication required", nil)
	}
	title = strings.TrimSpace(title)
	if utf8.RuneCountInString(title) > 200 {
		return Conversation{}, ValidationError(map[string]string{"title": "must not exceed 200 characters"})
	}
	now := s.now().UTC()
	return s.repository.CreateConversation(ctx, Conversation{
		ID: newID("conv"), Title: title, OwnerUserID: userID, Status: "active", CreatedAt: now, UpdatedAt: now,
	})
}

func (s *QAService) ListConversations(ctx context.Context, userID string, options ConversationListOptions) (Page[Conversation], error) {
	if strings.TrimSpace(userID) == "" {
		return Page[Conversation]{}, NewError(CodeUnauthorized, "authentication required", nil)
	}
	normalized, err := normalizeConversationListOptions(options)
	if err != nil {
		return Page[Conversation]{}, err
	}
	return s.repository.ListConversations(ctx, userID, normalized)
}

func (s *QAService) GetConversation(ctx context.Context, userID, id string) (Conversation, error) {
	return s.repository.GetConversation(ctx, userID, id)
}

func (s *QAService) UpdateConversation(ctx context.Context, userID, id, title, status string) (Conversation, error) {
	title, status = strings.TrimSpace(title), strings.TrimSpace(status)
	if title == "" && status == "" {
		return Conversation{}, ValidationError(map[string]string{"body": "title or status is required"})
	}
	if utf8.RuneCountInString(title) > 200 {
		return Conversation{}, ValidationError(map[string]string{"title": "must not exceed 200 characters"})
	}
	if status != "" && status != "active" && status != "archived" {
		return Conversation{}, ValidationError(map[string]string{"status": "must be active or archived"})
	}
	conversation, err := s.repository.GetConversation(ctx, userID, id)
	if err != nil {
		return Conversation{}, err
	}
	if title != "" {
		conversation.Title = title
	}
	if status != "" {
		conversation.Status = status
	}
	conversation.UpdatedAt = s.now().UTC()
	return s.repository.UpdateConversation(ctx, userID, conversation)
}

func (s *QAService) DeleteConversation(ctx context.Context, userID, id string) error {
	return s.repository.DeleteConversation(ctx, userID, id)
}

func (s *QAService) ListMessages(ctx context.Context, userID, conversationID string, options MessageListOptions) (Page[Message], error) {
	if strings.TrimSpace(userID) == "" {
		return Page[Message]{}, NewError(CodeUnauthorized, "authentication required", nil)
	}
	if strings.TrimSpace(conversationID) == "" {
		return Page[Message]{}, ValidationError(map[string]string{"sessionId": "is required"})
	}
	normalized, err := normalizeMessageListOptions(options)
	if err != nil {
		return Page[Message]{}, err
	}
	return s.repository.ListMessages(ctx, userID, conversationID, normalized)
}

func (s *QAService) Ask(ctx context.Context, userID, conversationID string, input AskInput, observe ProgressObserver) (AskResult, error) {
	if err := validateAskInput(input); err != nil {
		return AskResult{}, err
	}
	conversation, err := s.repository.GetConversation(ctx, userID, conversationID)
	if err != nil {
		return AskResult{}, err
	}
	history, err := s.repository.ListMessages(ctx, userID, conversationID, MessageListOptions{Page: 1, PageSize: 100})
	if err != nil {
		return AskResult{}, err
	}

	runtime, release, err := s.runtime.Acquire()
	if err != nil {
		return AskResult{}, NewError(CodeDependency, "agent runtime is unavailable", err)
	}
	defer release()

	now := s.now().UTC()
	intent := input.Mode
	if intent == "" {
		intent = "unknown"
	}
	userMessage := Message{ID: newID("msg"), ConversationID: conversationID, Role: agent.RoleUser, Content: strings.TrimSpace(input.Message), Intent: intent, Status: "completed", CreatedAt: now}
	assistantMessage := Message{ID: newID("msg"), ConversationID: conversationID, Role: agent.RoleAssistant, Intent: intent, Status: "streaming", CreatedAt: now}
	run, err := s.repository.AppendMessages(ctx, userID, conversationID, ResponseRunStart{
		RequestID:          RequestIDFromContext(ctx),
		QAConfigVersionID:  runtime.QAConfigVersionID,
		LLMConfigVersionID: runtime.LLMConfigVersionID,
		MaxIterations:      runtime.MaxIterations,
	}, userMessage, assistantMessage)
	if err != nil {
		return AskResult{}, err
	}
	baseCtx := ctx
	cancelBase := func() {}
	if runtime.OverallTimeout > 0 {
		var cancel context.CancelFunc
		baseCtx, cancel = context.WithTimeout(ctx, runtime.OverallTimeout)
		cancelBase = cancel
	}
	runCtx, cancelRun := context.WithCancel(baseCtx)
	s.activeMu.Lock()
	s.activeRuns[run.ID] = cancelRun
	s.activeMu.Unlock()
	defer func() {
		cancelRun()
		cancelBase()
		s.activeMu.Lock()
		delete(s.activeRuns, run.ID)
		s.activeMu.Unlock()
	}()
	if conversation.Title == "" || conversation.Title == "新对话" {
		conversation.Title = truncateRunes(userMessage.Content, 40)
		conversation.UpdatedAt = now
		_, _ = s.repository.UpdateConversation(ctx, userID, conversation)
	}
	events := make([]StreamEvent, 0, 12)
	emit := func(eventType string, payload map[string]any) {
		event := StreamEvent{EventSeq: len(events) + 1, EventType: eventType, Payload: payload, CreatedAt: s.now().UTC()}
		events = append(events, event)
		emitProgress(observe, ProgressEvent{Type: eventType, Sequence: event.EventSeq, Payload: payload, UserMessageID: userMessage.ID, AssistantMessage: assistantMessage.ID, Intent: intent})
	}
	emit("message.created", map[string]any{"responseRunId": run.ID, "userMessageId": userMessage.ID, "assistantMessageId": assistantMessage.ID, "status": "running"})
	messages := make([]agent.Message, 0, len(history.Items)+3)
	messages = append(messages, agent.Message{Role: agent.RoleSystem, Content: runtime.SystemPrompt})
	if directive := requestDirective(input); directive != "" {
		messages = append(messages, agent.Message{Role: agent.RoleSystem, Content: directive})
	}
	for _, item := range history.Items {
		if item.Status == "completed" && (item.Role == agent.RoleUser || item.Role == agent.RoleAssistant) {
			messages = append(messages, agent.Message{Role: item.Role, Content: item.Content})
		}
	}
	messages = append(messages, agent.Message{Role: agent.RoleUser, Content: userMessage.Content})

	steps := make([]ReasoningStep, 0, 4)
	iterationStartedAt := map[int]time.Time{}
	completedIterations := map[int]struct{}{}
	usage := agent.TokenUsage{}
	var invocationErr error
	profileID := runtime.LLMProfileID
	if profileID == "" {
		profileID = "default"
	}
	result, runErr := runtime.Runner.RunWithObserver(runCtx, messages, func(event agent.Event) {
		switch event.Type {
		case agent.EventModelStarted:
			iterationStartedAt[event.Iteration] = s.now().UTC()
			emit("agent.iteration.started", map[string]any{"responseRunId": run.ID, "iterationNo": event.Iteration})
		case agent.EventModelCompleted:
			startedAt := iterationStartedAt[event.Iteration]
			if startedAt.IsZero() {
				startedAt = s.now().UTC()
			}
			finishedAt := s.now().UTC()
			accumulateUsage(&usage, event.Usage)
			completedIterations[event.Iteration] = struct{}{}
			saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_, err := s.repository.SaveModelInvocation(saveCtx, userID, ModelInvocation{
				ResponseRunID:    run.ID,
				IterationNo:      event.Iteration,
				Provider:         "ai-gateway",
				ProfileID:        profileID,
				ModelName:        runtime.LLMModel,
				FinishReason:     event.FinishReason,
				Status:           "completed",
				PromptTokens:     event.Usage.PromptTokens,
				CompletionTokens: event.Usage.CompletionTokens,
				ReasoningTokens:  event.Usage.ReasoningTokens,
				TotalTokens:      event.Usage.TotalTokens,
				StartedAt:        startedAt,
				FinishedAt:       &finishedAt,
				LatencyMS:        finishedAt.Sub(startedAt).Milliseconds(),
			})
			if err != nil && invocationErr == nil {
				invocationErr = err
			}
		case agent.EventToolStarted, agent.EventToolCompleted, agent.EventToolFailed:
			emit(string(event.Type), map[string]any{"toolCallId": event.ToolCallID, "tool": event.ToolName, "iterationNo": event.Iteration})
		}
		step, ok := stepFromAgentEvent(assistantMessage.ID, event, s.now().UTC())
		if !ok {
			return
		}
		steps = append(steps, step)
		emit("reasoning.step", map[string]any{"type": publicStepType(step.Type), "label": step.Title, "status": publicStepStatus(step.Status), "detail": step.Summary})
	})
	if runErr == nil && invocationErr != nil {
		runErr = fmt.Errorf("save model invocation: %w", invocationErr)
	}
	if runErr != nil {
		status, reason, publicMessage := classifyRunError(runErr)
		assistantMessage.Status = "failed"
		if status == "cancelled" {
			assistantMessage.Status = "cancelled"
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if shouldRecordFailedModelInvocation(reason, iterationStartedAt, completedIterations) {
			s.saveFailedModelInvocation(cleanupCtx, userID, run.ID, runtime, profileID, reason, iterationStartedAt)
		}
		emit("error", map[string]any{"responseRunId": run.ID, "code": "dependency_error", "message": publicMessage})
		finalized, finalizeErr := s.repository.FinalizeResponseRun(cleanupCtx, userID, ResponseRunFinalization{
			RunID: run.ID, AssistantMessage: assistantMessage, ReasoningSteps: steps, StreamEvents: events,
			Status: status, TerminationReason: reason, CurrentIteration: maxStartedIteration(iterationStartedAt),
			PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
			ReasoningTokens: usage.ReasoningTokens, TotalTokens: usage.TotalTokens,
			CompletedAt: s.now().UTC(),
		})
		if finalizeErr != nil {
			if appErr, ok := Classify(finalizeErr); ok && appErr.Code == CodeConflict {
				if finalized.ID == "" {
					loaded, loadErr := s.repository.GetResponseRun(cleanupCtx, userID, run.ID)
					if loadErr != nil {
						return AskResult{}, NewError(CodeDependency, "answer state persistence failed", fmt.Errorf("load response run after finalization conflict: %w", loadErr))
					}
					finalized = loaded
				}
				if saveErr := s.saveReplayRecords(cleanupCtx, userID, run.ID, assistantMessage.ID, steps, events); saveErr != nil {
					return AskResult{}, NewError(CodeDependency, "answer state persistence failed", fmt.Errorf("save replay records after finalization conflict: %w", saveErr))
				}
				run = finalized
				return AskResult{UserMessage: userMessage, AssistantMessage: assistantMessage, ResponseRun: run, Citations: []any{}, ReasoningSteps: steps}, NewError(CodeDependency, publicMessage, runErr)
			}
			return AskResult{}, NewError(CodeDependency, "answer state persistence failed", fmt.Errorf("finalize failed response run after agent error: %w", finalizeErr))
		}
		run = finalized
		return AskResult{UserMessage: userMessage, AssistantMessage: assistantMessage, ResponseRun: run, Citations: []any{}, ReasoningSteps: steps}, NewError(CodeDependency, publicMessage, runErr)
	}
	assistantMessage.Content = result.Final.Content
	assistantMessage.Status = "completed"
	emit("answer.delta", map[string]any{"messageId": assistantMessage.ID, "text": assistantMessage.Content, "index": 0})
	emit("answer.completed", map[string]any{"responseRunId": run.ID, "messageId": assistantMessage.ID})
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	run, err = s.repository.FinalizeResponseRun(cleanupCtx, userID, ResponseRunFinalization{
		RunID: run.ID, AssistantMessage: assistantMessage, ReasoningSteps: steps, StreamEvents: events,
		Status: "completed", TerminationReason: "completed", CurrentIteration: result.Iterations,
		PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
		ReasoningTokens: usage.ReasoningTokens, TotalTokens: usage.TotalTokens,
		CompletedAt: s.now().UTC(),
	})
	if err != nil {
		return AskResult{}, fmt.Errorf("finalize response run: %w", err)
	}
	return AskResult{UserMessage: userMessage, AssistantMessage: assistantMessage, ResponseRun: run, Citations: []any{}, ReasoningSteps: steps}, nil
}

func (s *QAService) saveReplayRecords(ctx context.Context, userID, runID, assistantMessageID string, steps []ReasoningStep, events []StreamEvent) error {
	if err := s.repository.SaveReasoningSteps(ctx, userID, assistantMessageID, steps); err != nil {
		return fmt.Errorf("save reasoning steps: %w", err)
	}
	if err := s.repository.SaveStreamEvents(ctx, userID, runID, events); err != nil {
		return fmt.Errorf("save stream events: %w", err)
	}
	return nil
}

func shouldRecordFailedModelInvocation(reason string, started map[int]time.Time, completed map[int]struct{}) bool {
	if reason == "max_iterations" {
		return false
	}
	iteration := maxStartedIteration(started)
	if iteration == 0 {
		return false
	}
	_, done := completed[iteration]
	return !done
}

func (s *QAService) saveFailedModelInvocation(ctx context.Context, userID, runID string, runtime RuntimeSnapshot, profileID string, reason string, started map[int]time.Time) {
	iteration := maxStartedIteration(started)
	if iteration == 0 {
		iteration = 1
	}
	startedAt := started[iteration]
	if startedAt.IsZero() {
		startedAt = s.now().UTC()
	}
	finishedAt := s.now().UTC()
	status := "failed"
	if reason == "cancelled" {
		status = "cancelled"
	}
	_, _ = s.repository.SaveModelInvocation(ctx, userID, ModelInvocation{
		ResponseRunID: runID, IterationNo: iteration, Provider: "ai-gateway",
		ProfileID: profileID, ModelName: runtime.LLMModel, Status: status,
		ErrorCode: "dependency_error", ErrorMessage: publicRunErrorMessage(reason),
		StartedAt: startedAt, FinishedAt: &finishedAt, LatencyMS: finishedAt.Sub(startedAt).Milliseconds(),
	})
}

func classifyRunError(err error) (status, reason, publicMessage string) {
	switch {
	case errors.Is(err, context.Canceled):
		return "cancelled", "cancelled", publicRunErrorMessage("cancelled")
	case errors.Is(err, context.DeadlineExceeded):
		return "failed", "timeout", publicRunErrorMessage("timeout")
	case errors.Is(err, agent.ErrMaxIterations):
		return "failed", "max_iterations", publicRunErrorMessage("max_iterations")
	default:
		return "failed", "model_error", publicRunErrorMessage("model_error")
	}
}

func publicRunErrorMessage(reason string) string {
	switch reason {
	case "cancelled":
		return "answer generation cancelled"
	case "timeout":
		return "answer generation timed out"
	case "max_iterations":
		return "answer generation reached the maximum iterations"
	default:
		return "answer generation failed"
	}
}

func accumulateUsage(total *agent.TokenUsage, next agent.TokenUsage) {
	if next.TotalTokens == 0 {
		next.TotalTokens = next.PromptTokens + next.CompletionTokens + next.ReasoningTokens
	}
	total.PromptTokens += next.PromptTokens
	total.CompletionTokens += next.CompletionTokens
	total.ReasoningTokens += next.ReasoningTokens
	total.TotalTokens += next.TotalTokens
}

func maxStartedIteration(started map[int]time.Time) int {
	maxIteration := 0
	for iteration := range started {
		if iteration > maxIteration {
			maxIteration = iteration
		}
	}
	return maxIteration
}

func (s *QAService) CancelActiveRun(runID string) {
	s.activeMu.Lock()
	cancel := s.activeRuns[runID]
	s.activeMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func publicStepType(value string) string {
	if value == "tool" {
		return "tool_call"
	}
	return value
}

func publicStepStatus(value string) string {
	if value == "completed" {
		return "done"
	}
	return value
}

func validateAskInput(input AskInput) error {
	message := strings.TrimSpace(input.Message)
	if message == "" || utf8.RuneCountInString(message) > 32000 {
		return ValidationError(map[string]string{"message": "must be between 1 and 32000 characters"})
	}
	allowedModes := map[string]bool{"": true, "knowledge_qa": true, "general_chat": true, "report_generation": true, "data_analysis": true, "unknown": true}
	if !allowedModes[input.Mode] {
		return ValidationError(map[string]string{"mode": "is not supported"})
	}
	if input.Mode == "data_analysis" {
		return NewError(CodeUnsupportedIntent, "data analysis is not supported", nil)
	}
	if len(input.KnowledgeBaseIDs) > 50 {
		return ValidationError(map[string]string{"knowledgeBaseIds": "must not contain more than 50 items"})
	}
	return nil
}

func requestDirective(input AskInput) string {
	var parts []string
	if input.Mode != "" && input.Mode != "unknown" {
		parts = append(parts, "The requested QA mode is "+input.Mode+".")
	}
	if len(input.KnowledgeBaseIDs) > 0 {
		parts = append(parts, "When a knowledge tool supports knowledge-base filtering, restrict it to: "+strings.Join(input.KnowledgeBaseIDs, ", ")+".")
	}
	return strings.Join(parts, " ")
}

func stepFromAgentEvent(messageID string, event agent.Event, now time.Time) (ReasoningStep, bool) {
	step := ReasoningStep{ID: newID("step"), MessageID: messageID, Status: "completed", CreatedAt: now}
	switch event.Type {
	case agent.EventModelStarted:
		step.Type, step.Title, step.Summary, step.Status = "generation", "生成回答", "模型开始处理当前步骤", "running"
	case agent.EventModelCompleted:
		step.Type, step.Title, step.Summary = "generation", "生成回答", "模型完成当前步骤"
	case agent.EventToolStarted:
		step.Type, step.Title, step.Summary, step.Status = "tool", "调用工具", "开始调用工具 "+event.ToolName, "running"
	case agent.EventToolCompleted:
		step.Type, step.Title, step.Summary = "tool", "调用工具", "工具 "+event.ToolName+" 调用完成"
	case agent.EventToolFailed:
		step.Type, step.Title, step.Summary, step.Status = "tool", "调用工具", "工具 "+event.ToolName+" 调用失败", "failed"
	default:
		return ReasoningStep{}, false
	}
	return step, true
}

func emitProgress(observer ProgressObserver, event ProgressEvent) {
	if observer != nil {
		observer(event)
	}
}

func newID(prefix string) string {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return fmt.Sprintf("00000000-0000-4000-8000-%012x", time.Now().UnixNano()&0xffffffffffff)
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(data)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func normalizeConversationListOptions(options ConversationListOptions) (ConversationListOptions, error) {
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.PageSize <= 0 {
		options.PageSize = 20
	}
	options.Status = strings.TrimSpace(options.Status)
	if options.Status == "" {
		options.Status = "active"
	}
	if options.Status != "active" && options.Status != "archived" {
		return ConversationListOptions{}, ValidationError(map[string]string{"status": "must be active or archived"})
	}
	options.Sort = strings.TrimSpace(options.Sort)
	if options.Sort == "" {
		options.Sort = "-updatedAt"
	}
	switch options.Sort {
	case "-updatedAt", "updatedAt", "-createdAt", "createdAt":
	default:
		return ConversationListOptions{}, ValidationError(map[string]string{"sort": "must be updatedAt, -updatedAt, createdAt, or -createdAt"})
	}
	return options, nil
}

func normalizeMessageListOptions(options MessageListOptions) (MessageListOptions, error) {
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.PageSize <= 0 {
		options.PageSize = 50
	}
	if options.PageSize > 100 {
		return MessageListOptions{}, ValidationError(map[string]string{"pageSize": "must be between 1 and 100"})
	}
	return options, nil
}
