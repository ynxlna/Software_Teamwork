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

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

func TestCreateFileSendsMultipartAndContextHeaders(t *testing.T) {
	var sawRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest = true
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v1/files" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Request-Id"); got != "req_file" {
			t.Fatalf("X-Request-Id = %q", got)
		}
		if got := r.Header.Get("X-User-Id"); got != "usr_test" {
			t.Fatalf("X-User-Id = %q", got)
		}
		if got := r.Header.Get("X-Caller-Service"); got != "document" {
			t.Fatalf("X-Caller-Service = %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm() error = %v", err)
		}
		if got := r.FormValue("checksumSha256"); got != "abc123" {
			t.Fatalf("checksumSha256 = %q", got)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile() error = %v", err)
		}
		defer file.Close()
		if header.Filename != "template.docx" {
			t.Fatalf("filename = %q", header.Filename)
		}
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if string(body) != "docx-bytes" {
			t.Fatalf("file body = %q", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":             "file_internal_001",
				"filename":       "template.docx",
				"contentType":    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
				"sizeBytes":      10,
				"checksumSha256": "abc123",
				"createdAt":      time.Date(2026, 6, 29, 15, 0, 0, 0, time.UTC).Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	client, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	file, err := client.CreateFile(context.Background(), service.RequestContext{
		RequestID: "req_file",
		UserID:    "usr_test",
	}, service.UploadedFile{
		Filename:       "template.docx",
		ContentType:    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		SizeBytes:      10,
		ChecksumSHA256: "abc123",
		Content:        bytes.NewReader([]byte("docx-bytes")),
	})
	if err != nil {
		t.Fatalf("CreateFile() error = %v", err)
	}
	if !sawRequest {
		t.Fatal("file service was not called")
	}
	if file.ID != "file_internal_001" || file.Filename != "template.docx" || file.SizeBytes != 10 || file.ChecksumSHA256 != "abc123" {
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

			client, err := New(server.URL, server.Client())
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			_, err = client.CreateFile(context.Background(), service.RequestContext{UserID: "usr_test"}, service.UploadedFile{
				Filename: "template.docx",
				Content:  strings.NewReader("docx"),
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

func TestDeleteFileTreatsMissingFileAsCleanedUp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/internal/v1/files/file_001" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Caller-Service"); got != "document" {
			t.Fatalf("X-Caller-Service = %q", got)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := client.DeleteFile(context.Background(), service.RequestContext{}, "file_001"); err != nil {
		t.Fatalf("DeleteFile() error = %v", err)
	}
}

func TestReadFileContentStreamsBodyAndHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/internal/v1/files/file_001/content" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Request-Id"); got != "req_file" {
			t.Fatalf("X-Request-Id = %q", got)
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		w.Header().Set("Content-Disposition", `attachment; filename="report.docx"`)
		_, _ = w.Write([]byte("docx-bytes"))
	}))
	defer server.Close()

	client, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	content, err := client.ReadFileContent(context.Background(), service.RequestContext{RequestID: "req_file"}, "file_001")
	if err != nil {
		t.Fatalf("ReadFileContent() error = %v", err)
	}
	defer content.Content.Close()
	body, err := io.ReadAll(content.Content)
	if err != nil {
		t.Fatalf("read content: %v", err)
	}
	if string(body) != "docx-bytes" || content.Filename != "report.docx" {
		t.Fatalf("unexpected content: filename=%q body=%q", content.Filename, string(body))
	}
}

func TestReadFileContentMapsMissingFileToNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"hidden"}}`))
	}))
	defer server.Close()

	client, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = client.ReadFileContent(context.Background(), service.RequestContext{}, "file_001")
	appErr, ok := service.Classify(err)
	if !ok || appErr.Code != service.CodeNotFound {
		t.Fatalf("error = %#v, want not_found", err)
	}
	if strings.Contains(appErr.Message, "hidden") {
		t.Fatalf("downstream body leaked into message: %q", appErr.Message)
	}
}
