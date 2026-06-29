package parser_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/parser"
)

func TestHTTPOCRClientPostsDocumentAndContextHeaders(t *testing.T) {
	var captured *http.Request
	var payload map[string]string
	client, err := parser.NewHTTPOCRClient(parser.HTTPOCRConfig{
		BaseURL:      "https://ocr.internal",
		ServiceToken: "secret-token",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			captured = req.Clone(req.Context())
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode request body error = %v", err)
			}
			return jsonResponse(http.StatusOK, `{"text":"Breaker OCR"}`), nil
		})},
	})
	if err != nil {
		t.Fatalf("NewHTTPOCRClient() error = %v", err)
	}

	result, err := client.ExtractText(context.Background(), parser.OCRRequest{
		DocumentName: "scan.pdf",
		ContentType:  "application/pdf",
		Data:         []byte("%PDF"),
		RequestID:    "req_123",
		UserID:       "usr_123",
	})
	if err != nil {
		t.Fatalf("ExtractText() error = %v", err)
	}
	if result.Text != "Breaker OCR" {
		t.Fatalf("result = %+v", result)
	}
	if captured.URL.String() != "https://ocr.internal/internal/v1/ocr" {
		t.Fatalf("url = %s", captured.URL.String())
	}
	if captured.Header.Get("X-Request-Id") != "req_123" ||
		captured.Header.Get("X-Caller-Service") != "knowledge" ||
		captured.Header.Get("X-User-Id") != "usr_123" ||
		captured.Header.Get("X-Service-Token") != "secret-token" {
		t.Fatalf("headers = %+v", captured.Header)
	}
	if payload["documentName"] != "scan.pdf" || payload["contentType"] != "application/pdf" {
		t.Fatalf("payload = %+v", payload)
	}
	decoded, err := base64.StdEncoding.DecodeString(payload["dataBase64"])
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	if !bytes.Equal(decoded, []byte("%PDF")) {
		t.Fatalf("decoded payload = %q", string(decoded))
	}
}

func TestHTTPOCRClientSanitizesFailure(t *testing.T) {
	client, err := parser.NewHTTPOCRClient(parser.HTTPOCRConfig{
		BaseURL: "https://ocr.internal/private-path",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusBadGateway, `{"error":"secret document text"}`), nil
		})},
	})
	if err != nil {
		t.Fatalf("NewHTTPOCRClient() error = %v", err)
	}

	_, err = client.ExtractText(context.Background(), parser.OCRRequest{
		DocumentName: "scan.pdf",
		ContentType:  "application/pdf",
		Data:         []byte("secret document text"),
	})
	if err == nil {
		t.Fatal("ExtractText() error = nil, want error")
	}
	if containsAny(err.Error(), "secret", "private-path", "scan.pdf") {
		t.Fatalf("error leaked sensitive detail: %v", err)
	}
}

func TestHTTPOCRClientDoesNotFollowRedirectWithServiceToken(t *testing.T) {
	requests := []*http.Request{}
	client, err := parser.NewHTTPOCRClient(parser.HTTPOCRConfig{
		BaseURL:      "https://ocr.internal",
		ServiceToken: "secret-token",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests = append(requests, req.Clone(req.Context()))
			if len(requests) == 1 {
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{"Location": []string{"https://evil.internal/steal"}},
					Body:       io.NopCloser(bytes.NewBufferString("redirect")),
					Request:    req,
				}, nil
			}
			return jsonResponse(http.StatusOK, `{"text":"redirected"}`), nil
		})},
	})
	if err != nil {
		t.Fatalf("NewHTTPOCRClient() error = %v", err)
	}

	_, err = client.ExtractText(context.Background(), parser.OCRRequest{
		DocumentName: "scan.pdf",
		ContentType:  "application/pdf",
		Data:         []byte("%PDF"),
	})
	if err == nil {
		t.Fatal("ExtractText() error = nil, want redirect status error")
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want no redirected request", len(requests))
	}
	if containsAny(err.Error(), "secret", "evil", "scan.pdf") {
		t.Fatalf("error leaked sensitive detail: %v", err)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func containsAny(value string, forbidden ...string) bool {
	for _, item := range forbidden {
		if item != "" && bytes.Contains([]byte(value), []byte(item)) {
			return true
		}
	}
	return false
}
