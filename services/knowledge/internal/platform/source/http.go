package source

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type HTTPReader struct {
	baseURL string
	client  *http.Client
}

func NewHTTPReader(baseURL string, client *http.Client) *HTTPReader {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPReader{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  client,
	}
}

func (r *HTTPReader) ReadSource(ctx context.Context, fileID string) (service.SourceDocument, error) {
	if r.baseURL == "" {
		return service.SourceDocument{}, fmt.Errorf("file service base URL is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/internal/v1/documents/"+strings.TrimSpace(fileID)+"/content", nil)
	if err != nil {
		return service.SourceDocument{}, err
	}
	response, err := r.client.Do(req)
	if err != nil {
		return service.SourceDocument{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		response.Body.Close()
		return service.SourceDocument{}, fmt.Errorf("file service returned HTTP %d", response.StatusCode)
	}
	return service.SourceDocument{
		Body:        response.Body,
		ContentType: response.Header.Get("Content-Type"),
		SizeBytes:   response.ContentLength,
	}, nil
}
