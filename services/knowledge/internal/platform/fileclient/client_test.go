package fileclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestCreateFileSendsMultipartAndContextHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v1/files" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Request-Id"); got != "req_upload" {
			t.Fatalf("X-Request-Id = %q", got)
		}
		if got := r.Header.Get("X-User-Id"); got != "usr_test" {
			t.Fatalf("X-User-Id = %q", got)
		}
		if got := r.Header.Get("X-Caller-Service"); got != "knowledge" {
			t.Fatalf("X-Caller-Service = %q", got)
		}
		if got := r.Header.Get("X-Service-Token"); got != "svc-token" {
			t.Fatalf("X-Service-Token = %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm() error = %v", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile() error = %v", err)
		}
		defer file.Close()
		if header.Filename != "knowledge-guide.pdf" {
			t.Fatalf("filename = %q", header.Filename)
		}
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if string(body) != "pdf-bytes" {
			t.Fatalf("file body = %q", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":             "file_internal_001",
				"filename":       "knowledge-guide.pdf",
				"contentType":    "application/pdf",
				"sizeBytes":      9,
				"checksumSha256": "abc123",
				"createdAt":      time.Date(2026, 6, 29, 15, 0, 0, 0, time.UTC).Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL, "svc-token", server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	file, err := client.CreateFile(context.Background(), service.RequestContext{
		RequestID: "req_upload",
		UserID:    "usr_test",
	}, service.UploadedFile{
		Filename:    "knowledge-guide.pdf",
		ContentType: "application/pdf",
		SizeBytes:   9,
		Content:     bytes.NewReader([]byte("pdf-bytes")),
	})
	if err != nil {
		t.Fatalf("CreateFile() error = %v", err)
	}
	if file.ID != "file_internal_001" || file.Filename != "knowledge-guide.pdf" || file.SizeBytes != 9 || file.ChecksumSHA256 != "abc123" {
		t.Fatalf("unexpected file object: %+v", file)
	}
}

func TestCreateFileClassifiesDownstreamErrors(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		wantCode service.Code
	}{
		{name: "validation", status: http.StatusBadRequest, wantCode: service.CodeValidation},
		{name: "dependency", status: http.StatusInternalServerError, wantCode: service.CodeDependency},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"error":{"message":"hidden downstream detail"}}`))
			}))
			defer server.Close()

			client, err := New(server.URL, "", server.Client())
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			_, err = client.CreateFile(context.Background(), service.RequestContext{UserID: "usr_test"}, service.UploadedFile{
				Filename: "knowledge-guide.pdf",
				Content:  strings.NewReader("pdf"),
			})
			if err == nil {
				t.Fatal("CreateFile() error = nil")
			}
			appErr, ok := service.Classify(err)
			if !ok || appErr.Code != tt.wantCode {
				t.Fatalf("error = %#v, want code %q", err, tt.wantCode)
			}
			if strings.Contains(appErr.Message, "hidden downstream detail") {
				t.Fatalf("downstream error leaked into message: %q", appErr.Message)
			}
		})
	}
}

func TestReadSourceSendsContextHeadersAndReturnsContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/internal/v1/files/file_001/content" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Request-Id"); got != "req_read" {
			t.Fatalf("X-Request-Id = %q", got)
		}
		if got := r.Header.Get("X-User-Id"); got != "usr_test" {
			t.Fatalf("X-User-Id = %q", got)
		}
		if got := r.Header.Get("X-Caller-Service"); got != "knowledge" {
			t.Fatalf("X-Caller-Service = %q", got)
		}
		if got := r.Header.Get("X-Service-Token"); got != "svc-token" {
			t.Fatalf("X-Service-Token = %q", got)
		}
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte("# Intro"))
	}))
	defer server.Close()

	client, err := New(server.URL, "svc-token", server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	source, err := client.ReadSource(context.Background(), service.RequestContext{
		RequestID: "req_read",
		UserID:    "usr_test",
	}, "file_001")
	if err != nil {
		t.Fatalf("ReadSource() error = %v", err)
	}
	defer source.Body.Close()
	body, err := io.ReadAll(source.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != "# Intro" || !strings.HasPrefix(source.ContentType, "text/markdown") {
		t.Fatalf("source = %q %q", string(body), source.ContentType)
	}
}

func TestReadSourceMapsDownstreamFailuresToSanitizedDependencyError(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"objectKey":"secret/object","url":"http://internal/files"}`))
			}))
			defer server.Close()

			client, err := New(server.URL, "svc-token", server.Client())
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			_, err = client.ReadSource(context.Background(), service.RequestContext{RequestID: "req", UserID: "usr"}, "file_001")
			appErr, ok := service.Classify(err)
			if !ok || appErr.Code != service.CodeDependency {
				t.Fatalf("error = %#v, want dependency", err)
			}
			if strings.Contains(appErr.Message, "secret") || strings.Contains(appErr.Message, "internal/files") {
				t.Fatalf("error leaked downstream detail: %q", appErr.Message)
			}
		})
	}
}

func TestReadSourcePreservesUnknownContentLength(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("streamed content"))
	}))
	defer server.Close()

	client, err := New(server.URL, "svc-token", server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	source, err := client.ReadSource(context.Background(), service.RequestContext{RequestID: "req_read", UserID: "usr"}, "file_001")
	if err != nil {
		t.Fatalf("ReadSource() error = %v", err)
	}
	defer source.Body.Close()
	if source.SizeBytes != -1 {
		t.Fatalf("SizeBytes = %d, want unknown length -1", source.SizeBytes)
	}
	body, err := io.ReadAll(source.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != "streamed content" {
		t.Fatalf("body = %q", string(body))
	}
}

func TestReadSourceDoesNotFollowRedirects(t *testing.T) {
	redirectedHeaders := make(chan http.Header, 1)
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectedHeaders <- r.Header.Clone()
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("redirected content"))
	}))
	defer redirectTarget.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL+"/object/file_001", http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	client, err := New(server.URL, "svc-token", server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = client.ReadSource(context.Background(), service.RequestContext{
		RequestID: "req_redirect",
		UserID:    "usr_test",
	}, "file_001")
	if err == nil {
		t.Fatal("ReadSource() error = nil, want redirect response error")
	}
	appErr, ok := service.Classify(err)
	if !ok || appErr.Code != service.CodeDependency {
		t.Fatalf("error = %#v, want dependency", err)
	}

	select {
	case headers := <-redirectedHeaders:
		t.Fatalf("file client followed redirect and forwarded headers to target: X-Service-Token=%q X-User-Id=%q", headers.Get("X-Service-Token"), headers.Get("X-User-Id"))
	default:
	}
}

func TestDeleteFileTreatsMissingFileAsCleanedUp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/internal/v1/files/file_001" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Caller-Service"); got != "knowledge" {
			t.Fatalf("X-Caller-Service = %q", got)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := New(server.URL, "", server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := client.DeleteFile(context.Background(), service.RequestContext{}, "file_001"); err != nil {
		t.Fatalf("DeleteFile() error = %v", err)
	}
}

func TestGetFileContentStreamsContentAndContextHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/internal/v1/files/file_001/content" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Request-Id"); got != "req_content" {
			t.Fatalf("X-Request-Id = %q", got)
		}
		if got := r.Header.Get("X-User-Id"); got != "usr_test" {
			t.Fatalf("X-User-Id = %q", got)
		}
		if got := r.Header.Get("X-Caller-Service"); got != "knowledge" {
			t.Fatalf("X-Caller-Service = %q", got)
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Length", "9")
		_, _ = w.Write([]byte("pdf-bytes"))
	}))
	defer server.Close()

	client, err := New(server.URL, "svc-token", server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	content, err := client.GetFileContent(context.Background(), service.RequestContext{
		RequestID: "req_content",
		UserID:    "usr_test",
	}, "file_001")
	if err != nil {
		t.Fatalf("GetFileContent() error = %v", err)
	}
	defer content.Content.Close()

	body, err := io.ReadAll(content.Content)
	if err != nil {
		t.Fatalf("read content: %v", err)
	}
	if string(body) != "pdf-bytes" || content.ContentType != "application/pdf" || content.SizeBytes != 9 {
		t.Fatalf("content body=%q type=%q size=%d", string(body), content.ContentType, content.SizeBytes)
	}
}

func TestGetFileContentClassifiesDownstreamErrorsWithoutLeakingBody(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		wantCode service.Code
	}{
		{name: "not found", status: http.StatusNotFound, wantCode: service.CodeNotFound},
		{name: "dependency", status: http.StatusInternalServerError, wantCode: service.CodeDependency},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"error":{"message":"bucket hidden-bucket object obj-secret"}}`))
			}))
			defer server.Close()

			client, err := New(server.URL, "", server.Client())
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			_, err = client.GetFileContent(context.Background(), service.RequestContext{UserID: "usr_test"}, "file_001")
			if err == nil {
				t.Fatal("GetFileContent() error = nil")
			}
			appErr, ok := service.Classify(err)
			if !ok || appErr.Code != tt.wantCode {
				t.Fatalf("error = %#v, want code %q", err, tt.wantCode)
			}
			if strings.Contains(appErr.Message, "hidden-bucket") || strings.Contains(appErr.Message, "obj-secret") {
				t.Fatalf("downstream body leaked into message: %q", appErr.Message)
			}
		})
	}
}
