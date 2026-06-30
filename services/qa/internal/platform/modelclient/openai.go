package modelclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/httpclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

const maxResponseBytes = 2 << 20

type Config struct {
	Endpoint    string
	Token       string
	TokenHeader string
	Model       string
	ProfileID   string
	MaxTokens   int
	Timeout     time.Duration
}

type Client struct {
	endpoint  string
	model     string
	profileID string
	maxTokens int
	http      *http.Client
}

func New(cfg Config) (*Client, error) {
	parsed, err := url.Parse(cfg.Endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("model endpoint must be an absolute http(s) URL")
	}
	if cfg.Model == "" {
		return nil, errors.New("model is required")
	}
	if cfg.MaxTokens <= 0 {
		return nil, errors.New("max tokens must be positive")
	}
	if cfg.Timeout <= 0 {
		return nil, errors.New("model timeout must be positive")
	}
	return &Client{
		endpoint:  cfg.Endpoint,
		model:     cfg.Model,
		profileID: cfg.ProfileID,
		maxTokens: cfg.MaxTokens,
		http: &http.Client{
			Timeout: cfg.Timeout,
			Transport: httpclient.HeaderTransport{
				Header: cfg.TokenHeader,
				Token:  cfg.Token,
			},
		},
	}, nil
}

type completionRequest struct {
	Model      string                 `json:"model"`
	ProfileID  string                 `json:"profile_id,omitempty"`
	Messages   []agent.Message        `json:"messages"`
	Tools      []agent.ToolDefinition `json:"tools,omitempty"`
	ToolChoice string                 `json:"tool_choice,omitempty"`
	MaxTokens  int                    `json:"max_tokens"`
	Stream     bool                   `json:"stream"`
}

type completionResponse struct {
	Choices []struct {
		Message      agent.Message `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens            int `json:"prompt_tokens"`
		CompletionTokens        int `json:"completion_tokens"`
		TotalTokens             int `json:"total_tokens"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
}

func (c *Client) Complete(ctx context.Context, messages []agent.Message, tools []agent.ToolDefinition) (agent.Completion, error) {
	payload := completionRequest{
		Model:     c.model,
		ProfileID: c.profileID,
		Messages:  messages,
		Tools:     tools,
		MaxTokens: c.maxTokens,
		Stream:    false,
	}
	if len(tools) > 0 {
		payload.ToolChoice = "auto"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return agent.Completion{}, fmt.Errorf("marshal completion request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return agent.Completion{}, fmt.Errorf("create completion request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Caller-Service", "qa")
	if requestID := service.RequestIDFromContext(ctx); requestID != "" {
		request.Header.Set("X-Request-Id", requestID)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return agent.Completion{}, fmt.Errorf("call AI gateway: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return agent.Completion{}, fmt.Errorf("AI gateway returned HTTP %d", response.StatusCode)
	}
	limited := io.LimitReader(response.Body, maxResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return agent.Completion{}, fmt.Errorf("read completion response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return agent.Completion{}, errors.New("completion response exceeds size limit")
	}
	var decoded completionResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return agent.Completion{}, fmt.Errorf("decode completion response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return agent.Completion{}, errors.New("completion response has no choices")
	}
	choice := decoded.Choices[0]
	reasoningTokens := decoded.Usage.CompletionTokensDetails.ReasoningTokens
	completionTokens := decoded.Usage.CompletionTokens
	if reasoningTokens > 0 && completionTokens >= reasoningTokens {
		completionTokens -= reasoningTokens
	}
	usage := agent.TokenUsage{
		PromptTokens:     decoded.Usage.PromptTokens,
		CompletionTokens: completionTokens,
		ReasoningTokens:  reasoningTokens,
		TotalTokens:      decoded.Usage.TotalTokens,
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens + usage.ReasoningTokens
	}
	return agent.Completion{Message: choice.Message, FinishReason: choice.FinishReason, Usage: usage}, nil
}
