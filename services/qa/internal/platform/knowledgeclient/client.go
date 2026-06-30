package knowledgeclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
)

type Client struct {
	baseURL      string
	endpoint     string
	serviceToken string
	http         *http.Client
}

func New(baseURL, serviceToken string, timeout time.Duration) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("knowledge service URL must be absolute http(s)")
	}
	if strings.TrimSpace(serviceToken) == "" {
		return nil, errors.New("service token is required")
	}
	if timeout <= 0 {
		return nil, errors.New("knowledge request timeout must be positive")
	}
	normalizedBaseURL := strings.TrimRight(parsed.String(), "/")
	return &Client{baseURL: normalizedBaseURL, endpoint: normalizedBaseURL + "/internal/v1/knowledge-queries", serviceToken: serviceToken, http: &http.Client{Timeout: timeout}}, nil
}

func (c *Client) Retrieve(ctx context.Context, userID string, input service.RetrievalTestInput) ([]service.RetrievalTestResult, error) {
	payload := map[string]any{"query": input.Question, "knowledgeBaseIds": input.KnowledgeBaseIDs}
	retrieval := input.Retrieval
	if retrieval.TopK == 0 {
		retrieval = input.Overrides
	}
	if retrieval.TopK > 0 {
		payload["topK"] = retrieval.TopK
	}
	if retrieval.ScoreThreshold > 0 {
		payload["scoreThreshold"] = retrieval.ScoreThreshold
	}
	payload["rerank"] = retrieval.EnableRerank
	if retrieval.RerankTopN > 0 {
		payload["rerankTopN"] = retrieval.RerankTopN
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode knowledge query: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create knowledge query: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setTrustedHeaders(ctx, req, userID)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call knowledge service: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("knowledge service returned HTTP %d", resp.StatusCode)
	}
	var decoded struct {
		Data struct {
			Results []struct {
				Score           float64 `json:"score"`
				KnowledgeBaseID string  `json:"knowledgeBaseId"`
				DocumentID      string  `json:"documentId"`
				ChunkID         string  `json:"chunkId"`
				DocumentName    string  `json:"documentName"`
				SectionPath     string  `json:"sectionPath"`
				ContentPreview  string  `json:"contentPreview"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode knowledge response: %w", err)
	}
	results := make([]service.RetrievalTestResult, 0, len(decoded.Data.Results))
	for i, item := range decoded.Data.Results {
		results = append(results, service.RetrievalTestResult{RankNo: i + 1, KnowledgeBaseID: item.KnowledgeBaseID, DocumentID: item.DocumentID, ChunkID: item.ChunkID, DocumentName: item.DocumentName, SectionPath: item.SectionPath, Score: item.Score, VectorScore: item.Score, ContentPreview: item.ContentPreview, Metadata: map[string]any{}})
	}
	return results, nil
}

func (c *Client) CheckCitationSources(ctx context.Context, userID string, documentIDs []string) (map[string]bool, error) {
	availability := make(map[string]bool, len(documentIDs))
	for _, rawID := range documentIDs {
		documentID := strings.TrimSpace(rawID)
		if documentID == "" {
			continue
		}
		if _, exists := availability[documentID]; exists {
			continue
		}
		availability[documentID] = false
		endpoint := c.baseURL + "/internal/v1/documents/" + url.PathEscape(documentID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return availability, fmt.Errorf("create document visibility check: %w", err)
		}
		c.setTrustedHeaders(ctx, req, userID)
		resp, err := c.http.Do(req)
		if err != nil {
			return availability, fmt.Errorf("call knowledge document visibility: %w", err)
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			availability[documentID] = true
		case resp.StatusCode >= 400 && resp.StatusCode < 500:
			availability[documentID] = false
		default:
			return availability, fmt.Errorf("knowledge document visibility returned HTTP %d", resp.StatusCode)
		}
	}
	return availability, nil
}

func (c *Client) setTrustedHeaders(ctx context.Context, req *http.Request, userID string) {
	req.Header.Set("X-Service-Token", c.serviceToken)
	req.Header.Set("X-Caller-Service", "qa")
	req.Header.Set("X-User-Id", userID)
	if requestID := service.RequestIDFromContext(ctx); requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}
	if roles := service.UserRolesFromContext(ctx); roles != "" {
		req.Header.Set("X-User-Roles", roles)
	}
	if permissions := service.UserPermissionsFromContext(ctx); permissions != "" {
		req.Header.Set("X-User-Permissions", permissions)
	}
}
