package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

type pageResponse struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

func pageFromDomain(page service.PageMeta) pageResponse {
	return pageResponse{Page: page.Page, PageSize: page.PageSize, Total: page.Total}
}

func (s *Server) requestContext(r *http.Request) service.RequestContext {
	return service.RequestContext{
		RequestID:      requestIDFromContext(r.Context()),
		UserID:         strings.TrimSpace(r.Header.Get("X-User-Id")),
		CallerService:  strings.TrimSpace(r.Header.Get("X-Caller-Service")),
		ServiceToken:   strings.TrimSpace(r.Header.Get("X-Service-Token")),
		Roles:          splitCSV(r.Header.Get("X-User-Roles")),
		Permissions:    splitCSV(r.Header.Get("X-User-Permissions")),
		ForwardedFor:   strings.TrimSpace(r.Header.Get("X-Forwarded-For")),
		ForwardedProto: strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")),
	}
}

func (s *Server) requireDocumentService(w http.ResponseWriter, r *http.Request) bool {
	if s.documents != nil {
		return true
	}
	writeError(w, r, service.NewError(service.CodeDependency, "document service is not configured", nil))
	return false
}

func (s *Server) parseMultipartFile(w http.ResponseWriter, r *http.Request) (multipart.File, *multipart.FileHeader, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxUploadBytes)
	if err := r.ParseMultipartForm(s.maxUploadBytes); err != nil {
		fieldMessage := "multipart form is invalid"
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			fieldMessage = "exceeds maximum upload size"
		}
		writeError(w, r, service.ValidationError(map[string]string{"file": fieldMessage}))
		return nil, nil, false
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, r, service.ValidationError(map[string]string{"file": "is required"}))
		return nil, nil, false
	}
	return file, header, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, r, service.ValidationError(map[string]string{"body": "must be a valid JSON object"}))
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, r, service.ValidationError(map[string]string{"body": "must contain only one JSON object"}))
		return false
	}
	return true
}

func parsePage(r *http.Request) (int, int, error) {
	page, err := parsePositiveIntQuery(r, "page")
	if err != nil {
		return 0, 0, err
	}
	pageSize, err := parsePositiveIntQuery(r, "pageSize")
	if err != nil {
		return 0, 0, err
	}
	return page, pageSize, nil
}

func parsePositiveIntQuery(r *http.Request, key string) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, service.ValidationError(map[string]string{key: "must be a positive integer"})
	}
	return value, nil
}

func parseOptionalBoolQuery(r *http.Request, key string) (*bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, service.ValidationError(map[string]string{key: "must be a boolean"})
	}
	return &value, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func formValue(r *http.Request, key string) string {
	if r.MultipartForm == nil {
		return ""
	}
	return strings.TrimSpace(firstValue(r.MultipartForm.Value[key]))
}

func formValues(r *http.Request, key string) []string {
	if r.MultipartForm == nil {
		return nil
	}
	values := r.MultipartForm.Value[key]
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	return result
}

func firstValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
