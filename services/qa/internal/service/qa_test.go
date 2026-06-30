package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

type fakeRepository struct {
	conversation             Conversation
	getConversationErr       error
	deleteErr                error
	messages                 []Message
	listMessagesErr          error
	messageOptions           MessageListOptions
	savedSteps               []ReasoningStep
	savedEvents              []StreamEvent
	invocations              []ModelInvocation
	finalization             ResponseRunFinalization
	run                      ResponseRun
	finalizeErr              error
	finalizeErrRun           ResponseRun
	failOnCanceledFinalizing bool
	failOnCanceledInvocation bool
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func (r *fakeRepository) CreateConversation(_ context.Context, value Conversation) (Conversation, error) {
	r.conversation = value
	return value, nil
}
func (r *fakeRepository) ListConversations(_ context.Context, _ string, options ConversationListOptions) (Page[Conversation], error) {
	return Page[Conversation]{Items: []Conversation{r.conversation}, Page: options.Page, PageSize: options.PageSize, Total: 1}, nil
}
func (r *fakeRepository) GetConversation(context.Context, string, string) (Conversation, error) {
	if r.getConversationErr != nil {
		return Conversation{}, r.getConversationErr
	}
	return r.conversation, nil
}
func (r *fakeRepository) UpdateConversation(_ context.Context, _ string, value Conversation) (Conversation, error) {
	r.conversation = value
	return value, nil
}
func (r *fakeRepository) DeleteConversation(context.Context, string, string) error {
	return r.deleteErr
}
func (r *fakeRepository) ListMessages(_ context.Context, _ string, _ string, options MessageListOptions) (Page[Message], error) {
	if r.listMessagesErr != nil {
		return Page[Message]{}, r.listMessagesErr
	}
	r.messageOptions = options
	return Page[Message]{Items: append([]Message(nil), r.messages...), Page: options.Page, PageSize: options.PageSize, Total: len(r.messages)}, nil
}
func (r *fakeRepository) AppendMessages(_ context.Context, _, sessionID string, start ResponseRunStart, values ...Message) (ResponseRun, error) {
	r.messages = append(r.messages, values...)
	maxIterations := start.MaxIterations
	if maxIterations == 0 {
		maxIterations = 5
	}
	r.run = ResponseRun{ID: "run-id", SessionID: sessionID, UserMessageID: values[0].ID, AssistantMessageID: values[1].ID, Status: "running", MaxIterations: maxIterations, CreatedAt: values[0].CreatedAt}
	return r.run, nil
}
func (r *fakeRepository) SaveStreamEvents(_ context.Context, _, _ string, events []StreamEvent) error {
	r.savedEvents = append([]StreamEvent(nil), events...)
	return nil
}
func (r *fakeRepository) GetResponseRun(context.Context, string, string) (ResponseRun, error) {
	r.run.Status = "completed"
	return r.run, nil
}
func (r *fakeRepository) UpdateMessage(_ context.Context, _ string, value Message) error {
	for index := range r.messages {
		if r.messages[index].ID == value.ID {
			r.messages[index] = value
			return nil
		}
	}
	return errors.New("message not found")
}
func (r *fakeRepository) FinalizeResponseRun(ctx context.Context, _ string, final ResponseRunFinalization) (ResponseRun, error) {
	if r.failOnCanceledFinalizing {
		if err := ctx.Err(); err != nil {
			return ResponseRun{}, err
		}
	}
	if r.finalizeErr != nil {
		if r.finalizeErrRun.ID != "" {
			return r.finalizeErrRun, r.finalizeErr
		}
		return r.run, r.finalizeErr
	}
	r.finalization = final
	if err := r.UpdateMessage(context.Background(), "", final.AssistantMessage); err != nil {
		return ResponseRun{}, err
	}
	r.savedSteps = append([]ReasoningStep(nil), final.ReasoningSteps...)
	r.savedEvents = append([]StreamEvent(nil), final.StreamEvents...)
	r.run.Status = final.Status
	r.run.CurrentIteration = final.CurrentIteration
	r.run.TotalTokens = final.TotalTokens
	r.run.CompletedAt = &final.CompletedAt
	if final.TerminationReason != "" {
		reason := final.TerminationReason
		r.run.TerminationReason = &reason
	}
	return r.run, nil
}
func (r *fakeRepository) SaveReasoningSteps(_ context.Context, _, _ string, steps []ReasoningStep) error {
	r.savedSteps = append([]ReasoningStep(nil), steps...)
	return nil
}
func (r *fakeRepository) SaveModelInvocation(ctx context.Context, _ string, invocation ModelInvocation) (string, error) {
	if r.failOnCanceledInvocation {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}
	r.invocations = append(r.invocations, invocation)
	return fmt.Sprintf("invocation-%d", invocation.IterationNo), nil
}

type fakeAgentRunner struct {
	input  []agent.Message
	userID string
}
type blockingAgentRunner struct{ started chan struct{} }

func (r blockingAgentRunner) RunWithObserver(ctx context.Context, _ []agent.Message, _ agent.Observer) (agent.Result, error) {
	close(r.started)
	<-ctx.Done()
	return agent.Result{}, ctx.Err()
}

type completedThenCancelledRunner struct{ completed chan struct{} }

func (r completedThenCancelledRunner) RunWithObserver(ctx context.Context, _ []agent.Message, observer agent.Observer) (agent.Result, error) {
	observer(agent.Event{Type: agent.EventModelStarted, Iteration: 1})
	observer(agent.Event{Type: agent.EventModelCompleted, Iteration: 1, Usage: agent.TokenUsage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8}})
	close(r.completed)
	<-ctx.Done()
	return agent.Result{}, ctx.Err()
}

type cancelAfterCompletedRunner struct{ cancel context.CancelFunc }

func (r cancelAfterCompletedRunner) RunWithObserver(_ context.Context, input []agent.Message, observer agent.Observer) (agent.Result, error) {
	observer(agent.Event{Type: agent.EventModelStarted, Iteration: 1})
	observer(agent.Event{Type: agent.EventModelCompleted, Iteration: 1, Usage: agent.TokenUsage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10}})
	r.cancel()
	final := agent.Message{Role: agent.RoleAssistant, Content: "answer after disconnect"}
	return agent.Result{Final: final, Messages: append(input, final), Iterations: 1}, nil
}

type cancelBeforeCompletedObserverRunner struct{ cancel context.CancelFunc }

func (r cancelBeforeCompletedObserverRunner) RunWithObserver(_ context.Context, input []agent.Message, observer agent.Observer) (agent.Result, error) {
	observer(agent.Event{Type: agent.EventModelStarted, Iteration: 1})
	r.cancel()
	observer(agent.Event{Type: agent.EventModelCompleted, Iteration: 1, Usage: agent.TokenUsage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10}})
	final := agent.Message{Role: agent.RoleAssistant, Content: "answer after early disconnect"}
	return agent.Result{Final: final, Messages: append(input, final), Iterations: 1}, nil
}

type toolProgressRunner struct{}

func (toolProgressRunner) RunWithObserver(_ context.Context, input []agent.Message, observer agent.Observer) (agent.Result, error) {
	observer(agent.Event{Type: agent.EventModelStarted, Iteration: 1})
	observer(agent.Event{Type: agent.EventToolStarted, Iteration: 1, ToolCallID: "call-1", ToolName: "search_knowledge"})
	observer(agent.Event{Type: agent.EventToolCompleted, Iteration: 1, ToolCallID: "call-1", ToolName: "search_knowledge"})
	observer(agent.Event{Type: agent.EventModelCompleted, Iteration: 1, Usage: agent.TokenUsage{PromptTokens: 7, CompletionTokens: 5, TotalTokens: 12}})
	final := agent.Message{Role: agent.RoleAssistant, Content: "tool answer"}
	return agent.Result{Final: final, Messages: append(input, final), Iterations: 1}, nil
}

type citationToolRunner struct{}

func (citationToolRunner) RunWithObserver(_ context.Context, input []agent.Message, observer agent.Observer) (agent.Result, error) {
	observer(agent.Event{Type: agent.EventModelStarted, Iteration: 1})
	observer(agent.Event{Type: agent.EventToolStarted, Iteration: 1, ToolCallID: "call-1", ToolName: "search_knowledge"})
	observer(agent.Event{Type: agent.EventToolCompleted, Iteration: 1, ToolCallID: "call-1", ToolName: "search_knowledge"})
	observer(agent.Event{Type: agent.EventModelCompleted, Iteration: 1, Usage: agent.TokenUsage{PromptTokens: 8, CompletionTokens: 6, TotalTokens: 14}})
	toolResult := agent.Message{
		Role:       agent.RoleTool,
		Name:       "search_knowledge",
		ToolCallID: "call-1",
		Content:    `{"data":{"results":[{"documentId":"doc-1","documentName":"Boiler Manual","knowledgeBaseId":"kb-1","chunkId":"chunk-7","sectionPath":"3.1","quoteText":"inspect the valve before startup","contentPreview":"inspect the valve before startup","context":"Operators inspect the valve before startup.","content":"FULL RAW DOCUMENT BODY MUST NOT LEAK","fullText":"FULL RAW DOCUMENT BODY MUST NOT LEAK EITHER","pageNumber":12,"score":0.91,"rerankScore":0.88,"chunkType":"paragraph","metadata":{"pageLabel":"12","objectKey":"secret","internalUrl":"http://internal/doc","vector":[0.1,0.2]}}]}}`,
	}
	final := agent.Message{Role: agent.RoleAssistant, Content: "answer with citation [1]"}
	messages := append([]agent.Message{}, input...)
	messages = append(messages, toolResult, final)
	return agent.Result{Final: final, Messages: messages, Iterations: 1}, nil
}

func (r *fakeAgentRunner) RunWithObserver(ctx context.Context, input []agent.Message, observer agent.Observer) (agent.Result, error) {
	r.userID = UserIDFromContext(ctx)
	r.input = append([]agent.Message(nil), input...)
	observer(agent.Event{Type: agent.EventModelStarted, Iteration: 1})
	observer(agent.Event{Type: agent.EventModelCompleted, Iteration: 1, Usage: agent.TokenUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14}})
	final := agent.Message{Role: agent.RoleAssistant, Content: "测试回答"}
	return agent.Result{Final: final, Messages: append(input, final), Iterations: 1}, nil
}

type errorAgentRunner struct{ err error }

func (r errorAgentRunner) RunWithObserver(_ context.Context, _ []agent.Message, observer agent.Observer) (agent.Result, error) {
	observer(agent.Event{Type: agent.EventModelStarted, Iteration: 1})
	return agent.Result{}, r.err
}

type maxIterationsAgentRunner struct{}

func (maxIterationsAgentRunner) RunWithObserver(_ context.Context, _ []agent.Message, observer agent.Observer) (agent.Result, error) {
	observer(agent.Event{Type: agent.EventModelStarted, Iteration: 2})
	observer(agent.Event{Type: agent.EventModelCompleted, Iteration: 2, Usage: agent.TokenUsage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5}})
	return agent.Result{Iterations: 2}, agent.ErrMaxIterations
}

type fakeRuntimeProvider struct {
	runner         AgentRunner
	prompt         string
	maxIterations  int
	overallTimeout time.Duration
}

func (p fakeRuntimeProvider) Acquire() (RuntimeSnapshot, func(), error) {
	maxIterations := p.maxIterations
	if maxIterations == 0 {
		maxIterations = 5
	}
	return RuntimeSnapshot{
		Runner: p.runner, SystemPrompt: p.prompt, LLMModel: "deepseek-v4-pro", LLMProfileID: "default",
		QAConfigVersionID: "qa-config-id", LLMConfigVersionID: "llm-config-id",
		MaxIterations: maxIterations, OverallTimeout: p.overallTimeout,
	}, func() {}, nil
}

type fakeCitationSourceChecker struct {
	availability map[string]bool
	userID       string
	documentIDs  []string
}

func (c *fakeCitationSourceChecker) CheckCitationSources(_ context.Context, userID string, documentIDs []string) (map[string]bool, error) {
	c.userID = userID
	c.documentIDs = append([]string(nil), documentIDs...)
	return c.availability, nil
}

func TestAskPersistsConversationMessagesAndDisplayableSteps(t *testing.T) {
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	repository := &fakeRepository{conversation: Conversation{ID: "conversation-id", OwnerUserID: "user-id", Title: "新对话", Status: "active", CreatedAt: now, UpdatedAt: now}}
	runner := &fakeAgentRunner{}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: runner, prompt: "system prompt"})
	if err != nil {
		t.Fatal(err)
	}
	qa.now = func() time.Time { return now }
	var events []ProgressEvent
	result, err := qa.Ask(context.Background(), "user-id", "conversation-id", AskInput{Message: "锅炉检查要求", Mode: "knowledge_qa"}, func(event ProgressEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AssistantMessage.Content != "测试回答" || result.AssistantMessage.Status != "completed" {
		t.Fatalf("unexpected answer: %+v", result.AssistantMessage)
	}
	if repository.conversation.Title != "锅炉检查要求" {
		t.Fatalf("automatic title = %q", repository.conversation.Title)
	}
	if len(repository.messages) != 2 || repository.messages[1].Content != "测试回答" {
		t.Fatalf("unexpected persisted messages: %+v", repository.messages)
	}
	if len(repository.savedSteps) != 2 || len(events) != 6 || len(repository.savedEvents) != 6 {
		t.Fatalf("steps=%d events=%d", len(repository.savedSteps), len(events))
	}
	if result.ResponseRun.Status != "completed" || result.ResponseRun.TerminationReason == nil || *result.ResponseRun.TerminationReason != "completed" || result.ResponseRun.TotalTokens != 14 {
		t.Fatalf("unexpected response run: %+v", result.ResponseRun)
	}
	if len(repository.invocations) != 1 || repository.invocations[0].TotalTokens != 14 {
		t.Fatalf("unexpected model invocations: %+v", repository.invocations)
	}
	if len(runner.input) < 2 || runner.input[0].Role != agent.RoleSystem || runner.input[len(runner.input)-1].Content != "锅炉检查要求" {
		t.Fatalf("unexpected agent input: %+v", runner.input)
	}
	if runner.userID != "user-id" {
		t.Fatalf("agent context userID = %q", runner.userID)
	}
}

func TestAskRejectsUnsupportedDataAnalysis(t *testing.T) {
	err := validateAskInput(AskInput{Message: "分析表格", Mode: "data_analysis"})
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeUnsupportedIntent {
		t.Fatalf("error = %v, want unsupported_intent", err)
	}
}

func TestListConversationsNormalizesDocumentedOptions(t *testing.T) {
	repository := &fakeRepository{conversation: Conversation{ID: "conversation-id", Status: "active"}}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: &fakeAgentRunner{}, prompt: "system"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := qa.ListConversations(context.Background(), "user-id", ConversationListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Page != 1 || result.PageSize != 20 {
		t.Fatalf("page=%d pageSize=%d", result.Page, result.PageSize)
	}
	if _, err = qa.ListConversations(context.Background(), "user-id", ConversationListOptions{Status: "deleted"}); err == nil {
		t.Fatal("expected invalid status to fail")
	}
	if _, err = qa.ListConversations(context.Background(), "user-id", ConversationListOptions{Sort: "title"}); err == nil {
		t.Fatal("expected invalid sort to fail")
	}
}

func TestListMessagesNormalizesDocumentedOptions(t *testing.T) {
	repository := &fakeRepository{conversation: Conversation{ID: "conversation-id", Status: "active"}}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: &fakeAgentRunner{}, prompt: "system"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = qa.ListMessages(context.Background(), "user-id", "conversation-id", MessageListOptions{IncludeThinking: true, IncludeCitations: true})
	if err != nil {
		t.Fatal(err)
	}
	want := MessageListOptions{Page: 1, PageSize: 50, IncludeThinking: true, IncludeCitations: true}
	if repository.messageOptions != want {
		t.Fatalf("options=%+v want %+v", repository.messageOptions, want)
	}
	if _, err = qa.ListMessages(context.Background(), "user-id", "conversation-id", MessageListOptions{Page: 1, PageSize: 101}); err == nil {
		t.Fatal("expected invalid page size to fail")
	}
	if _, err = qa.ListMessages(context.Background(), "", "conversation-id", MessageListOptions{}); err == nil {
		t.Fatal("expected missing user to fail")
	}
}

func TestListMessagesRevalidatesEmbeddedCitationSources(t *testing.T) {
	repository := &fakeRepository{
		conversation: Conversation{ID: "conversation-id", OwnerUserID: "user-id", Status: "active"},
		messages: []Message{{
			ID:             "assistant-message-id",
			ConversationID: "conversation-id",
			Role:           agent.RoleAssistant,
			Citations: []Citation{{
				ID:         "citation-id",
				MessageID:  "assistant-message-id",
				CitationNo: 1,
				DocumentID: "doc-1",
				Text:       "saved quote",
			}},
		}},
	}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: &fakeAgentRunner{}, prompt: "system"})
	if err != nil {
		t.Fatal(err)
	}
	checker := &fakeCitationSourceChecker{availability: map[string]bool{"doc-1": true}}
	qa.SetCitationSourceChecker(checker)

	page, err := qa.ListMessages(context.Background(), "user-id", "conversation-id", MessageListOptions{IncludeCitations: true})
	if err != nil {
		t.Fatal(err)
	}
	if checker.userID != "user-id" || len(checker.documentIDs) != 1 || checker.documentIDs[0] != "doc-1" {
		t.Fatalf("source checker called with user=%q documents=%v", checker.userID, checker.documentIDs)
	}
	citation := page.Items[0].Citations[0]
	if !citation.IsSourceAvailable || citation.Source == nil || !citation.Source.Available || citation.Source.DownloadEndpoint != "/api/v1/documents/doc-1/content" {
		t.Fatalf("embedded citation source was not revalidated: %+v", citation)
	}
}

func TestSessionOperationsPropagateForbidden(t *testing.T) {
	forbidden := NewError(CodeForbidden, "conversation access denied", nil)
	repository := &fakeRepository{getConversationErr: forbidden, deleteErr: forbidden, listMessagesErr: forbidden}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: &fakeAgentRunner{}, prompt: "system"})
	if err != nil {
		t.Fatal(err)
	}

	operations := []struct {
		name string
		call func() error
	}{
		{name: "detail", call: func() error {
			_, err := qa.GetConversation(context.Background(), "user-id", "other-session")
			return err
		}},
		{name: "update", call: func() error {
			_, err := qa.UpdateConversation(context.Background(), "user-id", "other-session", "private", "active")
			return err
		}},
		{name: "delete", call: func() error {
			return qa.DeleteConversation(context.Background(), "user-id", "other-session")
		}},
		{name: "list messages", call: func() error {
			_, err := qa.ListMessages(context.Background(), "user-id", "other-session", MessageListOptions{})
			return err
		}},
		{name: "create message", call: func() error {
			_, err := qa.Ask(context.Background(), "user-id", "other-session", AskInput{Message: "private question"}, nil)
			return err
		}},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			appErr, ok := Classify(operation.call())
			if !ok || appErr.Code != CodeForbidden {
				t.Fatalf("error=%v, want forbidden", appErr)
			}
		})
	}
}

func TestCancelActiveRunCancelsAgentAndPersistsCancelledMessage(t *testing.T) {
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	repository := &fakeRepository{conversation: Conversation{ID: "conversation-id", OwnerUserID: "user-id", Status: "active", CreatedAt: now, UpdatedAt: now}}
	runner := blockingAgentRunner{started: make(chan struct{})}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: runner, prompt: "system"})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := qa.Ask(context.Background(), "user-id", "conversation-id", AskInput{Message: "cancel me"}, nil)
		done <- err
	}()
	<-runner.started
	qa.CancelActiveRun("run-id")
	if err := <-done; err == nil {
		t.Fatal("expected cancelled ask to fail")
	}
	if got := repository.messages[1].Status; got != "cancelled" {
		t.Fatalf("assistant status=%q", got)
	}
	if repository.finalization.TerminationReason != "cancelled" || repository.finalization.Status != "cancelled" {
		t.Fatalf("finalization=%+v", repository.finalization)
	}
	if len(repository.invocations) != 0 {
		t.Fatalf("invocations=%+v", repository.invocations)
	}
	if len(repository.savedEvents) == 0 || repository.savedEvents[len(repository.savedEvents)-1].EventType != "error" {
		t.Fatalf("saved events=%+v, want replayable cancellation error event", repository.savedEvents)
	}
	if got := repository.savedEvents[len(repository.savedEvents)-1].Payload["code"]; got != string(CodeDependency) {
		t.Fatalf("cancel error code=%v, want %s", got, CodeDependency)
	}
}

func TestCancelAfterCompletedModelCallDoesNotCreateFailedInvocation(t *testing.T) {
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	repository := &fakeRepository{conversation: Conversation{ID: "conversation-id", OwnerUserID: "user-id", Status: "active", CreatedAt: now, UpdatedAt: now}}
	runner := completedThenCancelledRunner{completed: make(chan struct{})}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: runner, prompt: "system"})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := qa.Ask(context.Background(), "user-id", "conversation-id", AskInput{Message: "cancel after completion"}, nil)
		done <- err
	}()
	<-runner.completed
	qa.CancelActiveRun("run-id")
	if err := <-done; err == nil {
		t.Fatal("expected cancelled ask to fail")
	}
	if len(repository.invocations) != 1 || repository.invocations[0].Status != "completed" {
		t.Fatalf("invocations=%+v", repository.invocations)
	}
}

func TestAskFinalizesSuccessfulRunAfterRequestContextCancelled(t *testing.T) {
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		conversation:             Conversation{ID: "conversation-id", OwnerUserID: "user-id", Status: "active", CreatedAt: now, UpdatedAt: now},
		failOnCanceledFinalizing: true,
	}
	ctx, cancel := context.WithCancel(context.Background())
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: cancelAfterCompletedRunner{cancel: cancel}, prompt: "system"})
	if err != nil {
		t.Fatal(err)
	}
	qa.now = func() time.Time { return now }
	result, err := qa.Ask(ctx, "user-id", "conversation-id", AskInput{Message: "disconnect after model"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.ResponseRun.Status != "completed" || result.AssistantMessage.Status != "completed" {
		t.Fatalf("result=%+v assistant=%+v", result.ResponseRun, result.AssistantMessage)
	}
	if repository.finalization.Status != "completed" || repository.finalization.TerminationReason != "completed" {
		t.Fatalf("finalization=%+v", repository.finalization)
	}
}

func TestAskPersistsCompletedInvocationAfterRequestContextCancelled(t *testing.T) {
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		conversation:             Conversation{ID: "conversation-id", OwnerUserID: "user-id", Status: "active", CreatedAt: now, UpdatedAt: now},
		failOnCanceledInvocation: true,
	}
	ctx, cancel := context.WithCancel(context.Background())
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: cancelBeforeCompletedObserverRunner{cancel: cancel}, prompt: "system"})
	if err != nil {
		t.Fatal(err)
	}
	qa.now = func() time.Time { return now }
	result, err := qa.Ask(ctx, "user-id", "conversation-id", AskInput{Message: "disconnect before invocation save"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.ResponseRun.Status != "completed" || result.AssistantMessage.Status != "completed" {
		t.Fatalf("result=%+v assistant=%+v", result.ResponseRun, result.AssistantMessage)
	}
	if len(repository.invocations) != 1 || repository.invocations[0].Status != "completed" {
		t.Fatalf("invocations=%+v", repository.invocations)
	}
}

func TestAskPersistsModelFailureReason(t *testing.T) {
	repository := &fakeRepository{conversation: Conversation{ID: "conversation-id", OwnerUserID: "user-id", Status: "active"}}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: errorAgentRunner{err: errors.New("provider secret detail")}, prompt: "system"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = qa.Ask(context.Background(), "user-id", "conversation-id", AskInput{Message: "hello"}, nil)
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeDependency {
		t.Fatalf("error=%v, want dependency_error", err)
	}
	if repository.finalization.Status != "failed" || repository.finalization.TerminationReason != "model_error" {
		t.Fatalf("finalization=%+v", repository.finalization)
	}
	if len(repository.invocations) != 1 || repository.invocations[0].Status != "failed" || repository.invocations[0].ErrorMessage != "answer generation failed" {
		t.Fatalf("invocations=%+v", repository.invocations)
	}
}

func TestAskPreservesGatewayValidationErrorClassification(t *testing.T) {
	repository := &fakeRepository{conversation: Conversation{ID: "conversation-id", OwnerUserID: "user-id", Status: "active"}}
	modelErr := NewError(CodeValidation, "AI gateway rejected model request", errors.New("AI gateway returned HTTP 400"))
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: errorAgentRunner{err: modelErr}, prompt: "system"})
	if err != nil {
		t.Fatal(err)
	}
	var events []ProgressEvent
	_, err = qa.Ask(context.Background(), "user-id", "conversation-id", AskInput{Message: "hello"}, func(event ProgressEvent) {
		events = append(events, event)
	})
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeValidation || appErr.Message != "AI gateway rejected model request" {
		t.Fatalf("error=%v, want validation_error", err)
	}
	if repository.finalization.Status != "failed" || repository.finalization.TerminationReason != string(CodeValidation) {
		t.Fatalf("finalization=%+v", repository.finalization)
	}
	if len(repository.invocations) != 1 || repository.invocations[0].ErrorCode != string(CodeValidation) {
		t.Fatalf("invocations=%+v", repository.invocations)
	}
	if len(repository.savedEvents) == 0 {
		t.Fatal("expected saved stream events")
	}
	last := repository.savedEvents[len(repository.savedEvents)-1]
	if last.EventType != "error" || last.Payload["code"] != string(CodeValidation) {
		t.Fatalf("last saved event=%+v, want validation error event", last)
	}
	if len(events) == 0 || events[len(events)-1].Payload["code"] != string(CodeValidation) {
		t.Fatalf("observed events=%+v, want validation error event", events)
	}
}

func TestAskReturnsPersistenceErrorWhenFailureFinalizationFails(t *testing.T) {
	repository := &fakeRepository{
		conversation: Conversation{ID: "conversation-id", OwnerUserID: "user-id", Status: "active"},
		finalizeErr:  errors.New("database timeout"),
	}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: errorAgentRunner{err: errors.New("provider failed")}, prompt: "system"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := qa.Ask(context.Background(), "user-id", "conversation-id", AskInput{Message: "hello"}, nil)
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeDependency || appErr.Message != "answer state persistence failed" {
		t.Fatalf("error=%v, want persistence dependency_error", err)
	}
	if result.ResponseRun.ID != "" {
		t.Fatalf("returned stale response run: %+v", result.ResponseRun)
	}
}

func TestAskKeepsCurrentRunWhenFailureFinalizationConflicts(t *testing.T) {
	cancelledAt := time.Date(2026, 6, 29, 11, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		conversation: Conversation{ID: "conversation-id", OwnerUserID: "user-id", Status: "active"},
		finalizeErr:  NewError(CodeConflict, "response run already finalized", nil),
		finalizeErrRun: ResponseRun{
			ID: "run-id", Status: "cancelled", CurrentIteration: 1, CompletedAt: &cancelledAt,
		},
	}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: errorAgentRunner{err: errors.New("provider failed")}, prompt: "system"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := qa.Ask(context.Background(), "user-id", "conversation-id", AskInput{Message: "hello"}, nil)
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeDependency {
		t.Fatalf("error=%v, want dependency_error", err)
	}
	if result.ResponseRun.Status != "cancelled" {
		t.Fatalf("response run=%+v, want cancelled state", result.ResponseRun)
	}
	if len(repository.savedSteps) == 0 {
		t.Fatal("expected reasoning steps to be saved after finalization conflict")
	}
	if len(repository.savedEvents) < 3 {
		t.Fatalf("saved events=%+v, want replayable cancellation events", repository.savedEvents)
	}
	if repository.savedEvents[len(repository.savedEvents)-1].EventType != "error" {
		t.Fatalf("last saved event=%+v, want error event", repository.savedEvents[len(repository.savedEvents)-1])
	}
}

func TestAskPersistsTimeoutReason(t *testing.T) {
	repository := &fakeRepository{conversation: Conversation{ID: "conversation-id", OwnerUserID: "user-id", Status: "active"}}
	runner := blockingAgentRunner{started: make(chan struct{})}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: runner, prompt: "system", overallTimeout: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	_, err = qa.Ask(context.Background(), "user-id", "conversation-id", AskInput{Message: "timeout"}, nil)
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeDependency {
		t.Fatalf("error=%v, want dependency_error", err)
	}
	if repository.finalization.Status != "failed" || repository.finalization.TerminationReason != "timeout" {
		t.Fatalf("finalization=%+v", repository.finalization)
	}
}

func TestAskPersistsMaxIterationsReason(t *testing.T) {
	repository := &fakeRepository{conversation: Conversation{ID: "conversation-id", OwnerUserID: "user-id", Status: "active"}}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: maxIterationsAgentRunner{}, prompt: "system", maxIterations: 2})
	if err != nil {
		t.Fatal(err)
	}
	_, err = qa.Ask(context.Background(), "user-id", "conversation-id", AskInput{Message: "loop"}, nil)
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeDependency {
		t.Fatalf("error=%v, want dependency_error", err)
	}
	if repository.finalization.Status != "failed" || repository.finalization.TerminationReason != "max_iterations" || repository.finalization.CurrentIteration != 2 {
		t.Fatalf("finalization=%+v", repository.finalization)
	}
	if len(repository.invocations) != 1 || repository.invocations[0].Status != "completed" || repository.invocations[0].TotalTokens != 5 {
		t.Fatalf("invocations=%+v", repository.invocations)
	}
}

func TestAskToolProgressEventsExposeOnlySafeSummaries(t *testing.T) {
	repository := &fakeRepository{conversation: Conversation{ID: "conversation-id", OwnerUserID: "user-id", Status: "active"}}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: toolProgressRunner{}, prompt: "system prompt with private instruction"})
	if err != nil {
		t.Fatal(err)
	}
	var events []ProgressEvent
	_, err = qa.Ask(context.Background(), "user-id", "conversation-id", AskInput{Message: "use tool"}, func(event ProgressEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatal(err)
	}
	seenToolEvent := false
	for _, event := range events {
		if event.Type != "tool.started" && event.Type != "tool.completed" && event.Type != "tool.failed" {
			continue
		}
		seenToolEvent = true
		for _, forbidden := range []string{"arguments", "args", "result", "rawResult", "internalUrl", "prompt"} {
			if _, ok := event.Payload[forbidden]; ok {
				t.Fatalf("tool event leaked %q in payload %#v", forbidden, event.Payload)
			}
		}
		if event.Payload["toolCallId"] != "call-1" || event.Payload["tool"] != "search_knowledge" || event.Payload["iterationNo"] != 1 {
			t.Fatalf("unexpected safe tool payload: %#v", event.Payload)
		}
	}
	if !seenToolEvent {
		t.Fatal("expected tool progress events")
	}
}

func TestAskPersistsCitationSnapshotsFromKnowledgeToolResults(t *testing.T) {
	now := time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC)
	repository := &fakeRepository{conversation: Conversation{ID: "conversation-id", OwnerUserID: "user-id", Status: "active", CreatedAt: now, UpdatedAt: now}}
	qa, err := NewQAService(repository, fakeRuntimeProvider{runner: citationToolRunner{}, prompt: "system"})
	if err != nil {
		t.Fatal(err)
	}
	qa.now = func() time.Time { return now }
	qa.SetCitationSourceChecker(&fakeCitationSourceChecker{availability: map[string]bool{"doc-1": true}})
	result, err := qa.Ask(context.Background(), "user-id", "conversation-id", AskInput{Message: "find citation", Mode: "knowledge_qa"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Citations) != 1 || len(repository.finalization.Citations) != 1 {
		t.Fatalf("result citations=%+v finalization=%+v", result.Citations, repository.finalization.Citations)
	}
	citation := repository.finalization.Citations[0]
	if citation.CitationNo != 1 || citation.MessageID != result.AssistantMessage.ID || citation.ResponseRunID != "run-id" {
		t.Fatalf("unexpected saved citation identity: %+v", citation)
	}
	if !uuidPattern.MatchString(citation.ID) {
		t.Fatalf("citation ID must be a UUID for postgres persistence: %q", citation.ID)
	}
	if citation.DocumentID != "doc-1" || citation.DocID != "doc-1" || citation.DocumentName != "Boiler Manual" || citation.DocName != "Boiler Manual" {
		t.Fatalf("unexpected citation document fields: %+v", citation)
	}
	if citation.Source == nil || !citation.Source.Available || citation.Source.DownloadEndpoint != "/api/v1/documents/doc-1/content" {
		t.Fatalf("unexpected citation source: %+v", citation.Source)
	}
	if strings.Contains(fmt.Sprintf("%#v", result.Citations), "FULL RAW DOCUMENT BODY") || strings.Contains(fmt.Sprintf("%#v", repository.savedEvents), "FULL RAW DOCUMENT BODY") {
		t.Fatalf("raw tool content leaked through Ask result or events: result=%+v events=%+v", result.Citations, repository.savedEvents)
	}
	if citation.Content != "inspect the valve before startup" || citation.ContentPreview != "inspect the valve before startup" {
		t.Fatalf("citation should expose only snapshot text, got %+v", citation)
	}
	if citation.Metadata["pageLabel"] != "12" {
		t.Fatalf("safe metadata not preserved: %#v", citation.Metadata)
	}
	for _, forbidden := range []string{"objectKey", "internalUrl", "vector"} {
		if _, ok := citation.Metadata[forbidden]; ok {
			t.Fatalf("citation metadata leaked %q: %#v", forbidden, citation.Metadata)
		}
	}
	citationSeq, completedSeq := 0, 0
	for _, event := range repository.savedEvents {
		if event.EventType == "citation.delta" {
			citationSeq = event.EventSeq
		}
		if event.EventType == "answer.completed" {
			completedSeq = event.EventSeq
		}
	}
	if citationSeq == 0 || completedSeq == 0 || citationSeq > completedSeq {
		t.Fatalf("citation event sequence=%d completed=%d events=%+v", citationSeq, completedSeq, repository.savedEvents)
	}
}

func TestNormalizeCitationMarksUnavailableSourceAndSanitizesMetadata(t *testing.T) {
	citation := NormalizeCitation(Citation{
		ID:           "citation-id",
		MessageID:    "message-id",
		CitationNo:   1,
		DocumentName: "Deleted source",
		Text:         "saved quote",
		Context:      "saved context",
		Metadata: map[string]any{
			"pageLabel": "8",
			"objectKey": "secret/object",
			"nested": map[string]any{
				"internalUrl": "http://internal/source",
				"safe":        "ok",
			},
		},
	})
	if citation.IsSourceAvailable || citation.Source == nil || citation.Source.Available || citation.Source.Reason != citationSourceUnavailableReason {
		t.Fatalf("unexpected unavailable source mapping: %+v", citation)
	}
	if citation.Content != "saved quote" || citation.ContentPreview != "saved quote" {
		t.Fatalf("snapshot text was not preserved: %+v", citation)
	}
	if _, ok := citation.Metadata["objectKey"]; ok {
		t.Fatalf("object key leaked in metadata: %#v", citation.Metadata)
	}
	nested, ok := citation.Metadata["nested"].(map[string]any)
	if !ok || nested["safe"] != "ok" {
		t.Fatalf("safe nested metadata not preserved: %#v", citation.Metadata)
	}
	if _, ok := nested["internalUrl"]; ok {
		t.Fatalf("internal URL leaked in nested metadata: %#v", nested)
	}
}
