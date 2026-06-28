package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type QdrantClient struct {
	baseURL    string
	apiKey     string
	collection string
	dimension  int
	client     *http.Client
}

type QdrantConfig struct {
	BaseURL    string
	APIKey     string
	Collection string
	Dimension  int
	HTTPClient *http.Client
}

func NewQdrantClient(cfg QdrantConfig) (*QdrantClient, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("qdrant base URL is required")
	}
	collection := strings.TrimSpace(cfg.Collection)
	if collection == "" {
		return nil, fmt.Errorf("qdrant collection is required")
	}
	if cfg.Dimension <= 0 {
		return nil, fmt.Errorf("qdrant vector dimension must be positive")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &QdrantClient{
		baseURL:    baseURL,
		apiKey:     strings.TrimSpace(cfg.APIKey),
		collection: collection,
		dimension:  cfg.Dimension,
		client:     client,
	}, nil
}

func (c *QdrantClient) EnsureCollection(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodGet, "/collections/"+c.collection, nil)
	if err != nil {
		return err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant collection check failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return nil
	}
	if res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("qdrant collection check returned HTTP %d", res.StatusCode)
	}

	body := map[string]any{
		"vectors": map[string]any{
			"size":     c.dimension,
			"distance": "Cosine",
		},
	}
	req, err = c.newRequest(ctx, http.MethodPut, "/collections/"+c.collection, body)
	if err != nil {
		return err
	}
	res, err = c.client.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant collection create failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("qdrant collection create returned HTTP %d", res.StatusCode)
	}
	return nil
}

func (c *QdrantClient) Upsert(ctx context.Context, points []service.VectorPoint) error {
	if len(points) == 0 {
		return nil
	}
	payload := struct {
		Points []qdrantPoint `json:"points"`
	}{Points: make([]qdrantPoint, 0, len(points))}
	for _, point := range points {
		payload.Points = append(payload.Points, qdrantPoint{
			ID:      point.ID,
			Vector:  point.Vector,
			Payload: clonePayload(point.Payload),
		})
	}
	req, err := c.newRequest(ctx, http.MethodPut, "/collections/"+c.collection+"/points?wait=true", payload)
	if err != nil {
		return err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant points upsert failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("qdrant points upsert returned HTTP %d", res.StatusCode)
	}
	return nil
}

func (c *QdrantClient) DeleteByDocument(ctx context.Context, documentID string) error {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return nil
	}
	payload := map[string]any{
		"filter": qdrantFilter{
			Must: []qdrantCondition{{
				Key:   "document_id",
				Match: map[string]any{"value": documentID},
			}},
		},
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/collections/"+c.collection+"/points/delete?wait=true", payload)
	if err != nil {
		return err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant points delete failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("qdrant points delete returned HTTP %d", res.StatusCode)
	}
	return nil
}

func (c *QdrantClient) Search(ctx context.Context, request service.VectorSearchRequest) ([]service.VectorSearchHit, error) {
	limit := request.Limit
	if limit < 1 {
		limit = 10
	}
	payload := map[string]any{
		"vector":       request.Vector,
		"limit":        limit,
		"with_payload": true,
	}
	if request.ScoreThreshold > 0 {
		payload["score_threshold"] = request.ScoreThreshold
	}
	if filter := buildQdrantFilter(request); len(filter.Must) > 0 {
		payload["filter"] = filter
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/collections/"+c.collection+"/points/search", payload)
	if err != nil {
		return nil, err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qdrant points search failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("qdrant points search returned HTTP %d", res.StatusCode)
	}
	var decoded struct {
		Result []struct {
			ID      string         `json:"id"`
			Score   float64        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("qdrant points search decode failed: %w", err)
	}
	hits := make([]service.VectorSearchHit, 0, len(decoded.Result))
	for _, hit := range decoded.Result {
		hits = append(hits, service.VectorSearchHit{
			ID:      hit.ID,
			Score:   hit.Score,
			Payload: clonePayload(hit.Payload),
		})
	}
	return hits, nil
}

func (c *QdrantClient) newRequest(ctx context.Context, method string, path string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal qdrant request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("api-key", c.apiKey)
	}
	return req, nil
}

type qdrantPoint struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload,omitempty"`
}

type qdrantFilter struct {
	Must []qdrantCondition `json:"must,omitempty"`
}

type qdrantCondition struct {
	Key   string         `json:"key"`
	Match map[string]any `json:"match"`
}

func buildQdrantFilter(request service.VectorSearchRequest) qdrantFilter {
	filter := qdrantFilter{}
	if len(request.KnowledgeBaseIDs) > 0 {
		filter.Must = append(filter.Must, qdrantCondition{
			Key:   "knowledge_base_id",
			Match: map[string]any{"any": request.KnowledgeBaseIDs},
		})
	}
	for _, tag := range request.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		filter.Must = append(filter.Must, qdrantCondition{
			Key:   "tags",
			Match: map[string]any{"value": tag},
		})
	}
	for key, value := range request.MetadataFilter {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		filter.Must = append(filter.Must, qdrantCondition{
			Key:   "metadata." + key,
			Match: map[string]any{"value": value},
		})
	}
	return filter
}
