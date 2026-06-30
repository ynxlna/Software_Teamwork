package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrMaxIterations   = errors.New("agent reached maximum iterations")
	ErrInvalidResponse = errors.New("model returned an invalid response")
)

type EventType string

const (
	EventModelStarted   EventType = "model.started"
	EventModelCompleted EventType = "model.completed"
	EventToolStarted    EventType = "tool.started"
	EventToolCompleted  EventType = "tool.completed"
	EventToolFailed     EventType = "tool.failed"
	EventAgentCompleted EventType = "agent.completed"
)

// Event intentionally excludes tool arguments, tool results, prompts, and
// credentials. It is safe to adapt into logs or public progress summaries.
type Event struct {
	Type         EventType
	Iteration    int
	ToolCallID   string
	ToolName     string
	FinishReason string
	Usage        TokenUsage
	Err          error
}

type Observer func(Event)

type Config struct {
	MaxIterations      int
	ToolTimeout        time.Duration
	MaxToolResultBytes int
	Observer           Observer
}

type Runner struct {
	model ModelClient
	tools ToolClient
	cfg   Config
}

type Result struct {
	Messages   []Message
	Final      Message
	Iterations int
}

func NewRunner(model ModelClient, tools ToolClient, cfg Config) (*Runner, error) {
	if model == nil {
		return nil, errors.New("model client is required")
	}
	if tools == nil {
		return nil, errors.New("tool client is required")
	}
	if cfg.MaxIterations <= 0 {
		return nil, errors.New("max iterations must be positive")
	}
	if cfg.ToolTimeout <= 0 {
		return nil, errors.New("tool timeout must be positive")
	}
	if cfg.MaxToolResultBytes <= 0 {
		return nil, errors.New("max tool result bytes must be positive")
	}
	return &Runner{model: model, tools: tools, cfg: cfg}, nil
}

func (r *Runner) Run(ctx context.Context, input []Message) (Result, error) {
	return r.RunWithObserver(ctx, input, r.cfg.Observer)
}

// RunWithObserver executes one agent run with a request-scoped observer. This
// keeps concurrent HTTP streams isolated while preserving Run for CLI users.
func (r *Runner) RunWithObserver(ctx context.Context, input []Message, observer Observer) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if len(input) == 0 {
		return Result{}, errors.New("at least one message is required")
	}

	toolDefs, err := r.tools.ListTools(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("list MCP tools: %w", err)
	}
	allowed := make(map[string]struct{}, len(toolDefs))
	for _, tool := range toolDefs {
		name := strings.TrimSpace(tool.Function.Name)
		if name == "" {
			return Result{}, errors.New("MCP server returned a tool with an empty name")
		}
		if _, exists := allowed[name]; exists {
			return Result{}, fmt.Errorf("MCP server returned duplicate tool %q", name)
		}
		allowed[name] = struct{}{}
	}

	messages := append([]Message(nil), input...)
	for iteration := 1; iteration <= r.cfg.MaxIterations; iteration++ {
		emit(observer, Event{Type: EventModelStarted, Iteration: iteration})
		completion, err := r.model.Complete(ctx, messages, toolDefs)
		if err != nil {
			return Result{}, fmt.Errorf("complete model iteration %d: %w", iteration, err)
		}
		assistant := completion.Message
		if assistant.Role == "" {
			assistant.Role = RoleAssistant
		}
		if assistant.Role != RoleAssistant {
			return Result{}, fmt.Errorf("%w: expected assistant role, got %q", ErrInvalidResponse, assistant.Role)
		}
		messages = append(messages, assistant)
		emit(observer, Event{Type: EventModelCompleted, Iteration: iteration, FinishReason: completion.FinishReason, Usage: completion.Usage})

		if len(assistant.ToolCalls) == 0 {
			if strings.TrimSpace(assistant.Content) == "" {
				return Result{}, fmt.Errorf("%w: empty final assistant message", ErrInvalidResponse)
			}
			emit(observer, Event{Type: EventAgentCompleted, Iteration: iteration, FinishReason: completion.FinishReason})
			return Result{Messages: messages, Final: assistant, Iterations: iteration}, nil
		}

		for _, call := range assistant.ToolCalls {
			resultMessage := r.executeTool(ctx, iteration, allowed, call, observer)
			messages = append(messages, resultMessage)
		}
	}

	return Result{Messages: messages, Iterations: r.cfg.MaxIterations}, ErrMaxIterations
}

func (r *Runner) executeTool(ctx context.Context, iteration int, allowed map[string]struct{}, call ToolCall, observer Observer) Message {
	name := strings.TrimSpace(call.Function.Name)
	base := Message{Role: RoleTool, ToolCallID: call.ID, Name: name}
	if call.ID == "" || name == "" {
		base.Content = toolErrorJSON("invalid_tool_call", "model returned an invalid tool call")
		emit(observer, Event{Type: EventToolFailed, Iteration: iteration, ToolCallID: call.ID, ToolName: name, Err: ErrInvalidResponse})
		return base
	}
	if _, ok := allowed[name]; !ok {
		base.Content = toolErrorJSON("unknown_tool", "requested tool is not available")
		emit(observer, Event{Type: EventToolFailed, Iteration: iteration, ToolCallID: call.ID, ToolName: name, Err: errors.New("unknown tool")})
		return base
	}

	arguments := json.RawMessage(call.Function.Arguments)
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	var object map[string]any
	if err := json.Unmarshal(arguments, &object); err != nil {
		base.Content = toolErrorJSON("invalid_tool_arguments", "tool arguments must be a JSON object")
		emit(observer, Event{Type: EventToolFailed, Iteration: iteration, ToolCallID: call.ID, ToolName: name, Err: err})
		return base
	}

	emit(observer, Event{Type: EventToolStarted, Iteration: iteration, ToolCallID: call.ID, ToolName: name})
	toolCtx, cancel := context.WithTimeout(ctx, r.cfg.ToolTimeout)
	defer cancel()
	result, err := r.tools.CallTool(toolCtx, name, arguments)
	if err != nil {
		base.Content = toolErrorJSON("tool_execution_failed", "tool execution failed")
		emit(observer, Event{Type: EventToolFailed, Iteration: iteration, ToolCallID: call.ID, ToolName: name, Err: err})
		return base
	}
	content := result.Content
	if result.IsError && strings.TrimSpace(content) == "" {
		content = toolErrorJSON("tool_execution_failed", "tool reported an error")
	}
	base.Content = truncateUTF8(content, r.cfg.MaxToolResultBytes)
	if result.IsError {
		emit(observer, Event{Type: EventToolFailed, Iteration: iteration, ToolCallID: call.ID, ToolName: name, Err: errors.New("tool reported an error")})
	} else {
		emit(observer, Event{Type: EventToolCompleted, Iteration: iteration, ToolCallID: call.ID, ToolName: name})
	}
	return base
}

func emit(observer Observer, event Event) {
	if observer != nil {
		observer(event)
	}
}

func toolErrorJSON(code, message string) string {
	payload, _ := json.Marshal(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
	return string(payload)
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	suffix := "\n...[tool result truncated]"
	limit := maxBytes - len(suffix)
	if limit <= 0 {
		return suffix[:maxBytes]
	}
	for limit > 0 && (value[limit]&0xC0) == 0x80 {
		limit--
	}
	return value[:limit] + suffix
}
