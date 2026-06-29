package parser

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPOCRConfig struct {
	BaseURL       string
	ServiceToken  string
	CallerService string
	Timeout       time.Duration
	Client        *http.Client
}

type HTTPOCRClient struct {
	baseURL       string
	serviceToken  string
	callerService string
	client        *http.Client
}

func NewHTTPOCRClient(cfg HTTPOCRConfig) (*HTTPOCRClient, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("OCR service base URL is required")
	}
	caller := strings.TrimSpace(cfg.CallerService)
	if caller == "" {
		caller = "knowledge"
	}
	client := cfg.Client
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	} else {
		copied := *client
		client = &copied
	}
	// OCR requests may include service credentials and document bytes. Treat
	// redirects as an error response so custom headers cannot be forwarded to
	// another host.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &HTTPOCRClient{
		baseURL:       baseURL,
		serviceToken:  strings.TrimSpace(cfg.ServiceToken),
		callerService: caller,
		client:        client,
	}, nil
}

func (c *HTTPOCRClient) ExtractText(ctx context.Context, request OCRRequest) (OCRResult, error) {
	payload := ocrHTTPRequest{
		DocumentName: strings.TrimSpace(request.DocumentName),
		ContentType:  strings.TrimSpace(request.ContentType),
		DataBase64:   base64.StdEncoding.EncodeToString(request.Data),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return OCRResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/ocr", bytes.NewReader(body))
	if err != nil {
		return OCRResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Caller-Service", c.callerService)
	if strings.TrimSpace(request.RequestID) != "" {
		req.Header.Set("X-Request-Id", strings.TrimSpace(request.RequestID))
	}
	if strings.TrimSpace(request.UserID) != "" {
		req.Header.Set("X-User-Id", strings.TrimSpace(request.UserID))
	}
	if c.serviceToken != "" {
		req.Header.Set("X-Service-Token", c.serviceToken)
	}

	res, err := c.client.Do(req)
	if err != nil {
		return OCRResult{}, fmt.Errorf("ocr service request failed")
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1024))
		return OCRResult{}, fmt.Errorf("ocr service returned HTTP %d", res.StatusCode)
	}
	var decoded ocrHTTPResponse
	if err := json.NewDecoder(io.LimitReader(res.Body, maxParsedTextBytes+1)).Decode(&decoded); err != nil {
		return OCRResult{}, fmt.Errorf("ocr service response could not be decoded")
	}
	if len(decoded.Text) > maxParsedTextBytes {
		return OCRResult{}, fmt.Errorf("ocr service response is too large")
	}
	return OCRResult{Text: decoded.Text}, nil
}

type ocrHTTPRequest struct {
	DocumentName string `json:"documentName,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	DataBase64   string `json:"dataBase64"`
}

type ocrHTTPResponse struct {
	Text string `json:"text"`
}
