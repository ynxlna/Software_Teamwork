package aigateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

const (
	defaultTimeout = 10 * time.Second
	callerService  = "document"
)

type ProfileClient struct {
	baseURL      string
	serviceToken string
	httpClient   *http.Client
}

func NewProfileClient(baseURL, serviceToken string, httpClient *http.Client) (*ProfileClient, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("DOCUMENT_AI_GATEWAY_URL must be an absolute http(s) URL")
	}
	if parsed.User != nil {
		return nil, errors.New("DOCUMENT_AI_GATEWAY_URL must not contain credentials")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &ProfileClient{
		baseURL:      strings.TrimRight(parsed.String(), "/"),
		serviceToken: strings.TrimSpace(serviceToken),
		httpClient:   httpClient,
	}, nil
}

func (c *ProfileClient) GetModelProfile(ctx context.Context, reqCtx service.RequestContext, id string) (service.ModelProfileReference, error) {
	profileID := strings.TrimSpace(id)
	if profileID == "" {
		return service.ModelProfileReference{}, service.ValidationError(map[string]string{"llm.profileId": "is required"})
	}
	endpoint, err := url.JoinPath(c.baseURL, "internal/v1/model-profiles", profileID)
	if err != nil {
		return service.ModelProfileReference{}, service.NewError(service.CodeDependency, "build ai gateway profile request", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return service.ModelProfileReference{}, service.NewError(service.CodeDependency, "build ai gateway profile request", err)
	}
	if c.serviceToken != "" {
		req.Header.Set("X-Service-Token", c.serviceToken)
	}
	req.Header.Set("X-Caller-Service", callerService)
	if strings.TrimSpace(reqCtx.RequestID) != "" {
		req.Header.Set("X-Request-Id", strings.TrimSpace(reqCtx.RequestID))
	}
	if strings.TrimSpace(reqCtx.UserID) != "" {
		req.Header.Set("X-User-Id", strings.TrimSpace(reqCtx.UserID))
	}
	if len(reqCtx.Roles) > 0 {
		req.Header.Set("X-User-Roles", strings.Join(reqCtx.Roles, ","))
	}
	if len(reqCtx.Permissions) > 0 {
		req.Header.Set("X-User-Permissions", strings.Join(reqCtx.Permissions, ","))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return service.ModelProfileReference{}, service.NewError(service.CodeDependency, "ai gateway profile lookup failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return service.ModelProfileReference{}, service.NewError(service.CodeNotFound, "model profile not found", nil)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return service.ModelProfileReference{}, service.NewError(service.CodeDependency, "ai gateway profile lookup failed", fmt.Errorf("status %d", resp.StatusCode))
	}

	var envelope struct {
		Data struct {
			ID        string `json:"id"`
			Purpose   string `json:"purpose"`
			Provider  string `json:"provider"`
			Model     string `json:"model"`
			Enabled   bool   `json:"enabled"`
			TimeoutMS int    `json:"timeoutMs"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return service.ModelProfileReference{}, service.NewError(service.CodeDependency, "decode ai gateway profile response", err)
	}
	return service.ModelProfileReference{
		ID:             envelope.Data.ID,
		Purpose:        envelope.Data.Purpose,
		Provider:       envelope.Data.Provider,
		Model:          envelope.Data.Model,
		Enabled:        envelope.Data.Enabled,
		TimeoutSeconds: timeoutMSToSeconds(envelope.Data.TimeoutMS),
	}, nil
}

func timeoutMSToSeconds(timeoutMS int) int {
	if timeoutMS <= 0 {
		return 0
	}
	return (timeoutMS + 999) / 1000
}
