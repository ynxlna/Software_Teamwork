package fileclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

const defaultTimeout = 30 * time.Second

type Client struct {
	baseURL      string
	serviceToken string
	httpClient   *http.Client
}

func New(baseURL string, serviceToken string, httpClient *http.Client) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("file service URL must be an absolute http(s) URL")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("file service URL must not contain credentials")
	}
	return &Client{
		baseURL:      strings.TrimRight(parsed.String(), "/"),
		serviceToken: strings.TrimSpace(serviceToken),
		httpClient:   noRedirectHTTPClient(httpClient),
	}, nil
}

func noRedirectHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	} else {
		copied := *client
		client = &copied
	}
	// File Service requests carry service credentials and user context. Treat
	// redirects as dependency responses so those headers are never replayed to
	// an object-store URL or any unexpected host.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client
}

func (c *Client) CreateFile(ctx context.Context, reqCtx service.RequestContext, file service.UploadedFile) (service.FileObject, error) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	go func() {
		err := writeMultipartFile(multipartWriter, file)
		if closeErr := multipartWriter.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/files", reader)
	if err != nil {
		return service.FileObject{}, service.NewError(service.CodeDependency, "file service request failed", err)
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	c.setContextHeaders(req, reqCtx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return service.FileObject{}, service.NewError(service.CodeDependency, "file service unavailable", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		switch resp.StatusCode {
		case http.StatusBadRequest, http.StatusRequestEntityTooLarge:
			return service.FileObject{}, service.NewError(service.CodeValidation, "file upload is invalid", nil)
		case http.StatusUnauthorized:
			return service.FileObject{}, service.NewError(service.CodeUnauthorized, "file service rejected knowledge request", nil)
		case http.StatusForbidden:
			return service.FileObject{}, service.NewError(service.CodeForbidden, "file service rejected knowledge request", nil)
		default:
			return service.FileObject{}, service.NewError(service.CodeDependency, "file service failed", nil)
		}
	}

	var envelope struct {
		Data struct {
			ID             string  `json:"id"`
			Filename       string  `json:"filename"`
			ContentType    string  `json:"contentType"`
			SizeBytes      int64   `json:"sizeBytes"`
			ChecksumSHA256 *string `json:"checksumSha256"`
			CreatedAt      string  `json:"createdAt"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return service.FileObject{}, service.NewError(service.CodeDependency, "file service returned invalid response", err)
	}
	createdAt, err := time.Parse(time.RFC3339, envelope.Data.CreatedAt)
	if err != nil {
		createdAt = time.Time{}
	}
	checksum := ""
	if envelope.Data.ChecksumSHA256 != nil {
		checksum = *envelope.Data.ChecksumSHA256
	}
	return service.FileObject{
		ID:             envelope.Data.ID,
		Filename:       envelope.Data.Filename,
		ContentType:    envelope.Data.ContentType,
		SizeBytes:      envelope.Data.SizeBytes,
		ChecksumSHA256: checksum,
		CreatedAt:      createdAt,
	}, nil
}

func (c *Client) DeleteFile(ctx context.Context, reqCtx service.RequestContext, fileID string) error {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/internal/v1/files/"+url.PathEscape(fileID), nil)
	if err != nil {
		return service.NewError(service.CodeDependency, "file service request failed", err)
	}
	c.setContextHeaders(req, reqCtx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return service.NewError(service.CodeDependency, "file service unavailable", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return service.NewError(service.CodeDependency, "file service failed", nil)
	}
	return nil
}

func (c *Client) ReadSource(ctx context.Context, reqCtx service.RequestContext, fileID string) (service.SourceDocument, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return service.SourceDocument{}, service.NewError(service.CodeDependency, "file source is not configured", nil)
	}
	content, err := c.readContent(ctx, reqCtx, fileID, false)
	if err != nil {
		return service.SourceDocument{}, err
	}
	return service.SourceDocument{
		Body:        content.Content,
		ContentType: content.ContentType,
		SizeBytes:   content.SizeBytes,
	}, nil
}

func (c *Client) GetFileContent(ctx context.Context, reqCtx service.RequestContext, fileID string) (service.FileContent, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return service.FileContent{}, service.NewError(service.CodeNotFound, "file content not found", nil)
	}
	return c.readContent(ctx, reqCtx, fileID, true)
}

func (c *Client) readContent(ctx context.Context, reqCtx service.RequestContext, fileID string, exposeResourceErrors bool) (service.FileContent, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/files/"+url.PathEscape(fileID)+"/content", nil)
	if err != nil {
		return service.FileContent{}, service.NewError(service.CodeDependency, "file service request failed", err)
	}
	c.setContextHeaders(req, reqCtx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return service.FileContent{}, service.NewError(service.CodeDependency, "file service unavailable", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		if !exposeResourceErrors {
			return service.FileContent{}, service.NewError(service.CodeDependency, "file service content read failed", nil)
		}
		switch resp.StatusCode {
		case http.StatusNotFound:
			return service.FileContent{}, service.NewError(service.CodeNotFound, "file content not found", nil)
		case http.StatusUnauthorized:
			return service.FileContent{}, service.NewError(service.CodeUnauthorized, "file service rejected knowledge request", nil)
		case http.StatusForbidden:
			return service.FileContent{}, service.NewError(service.CodeForbidden, "file service rejected knowledge request", nil)
		default:
			return service.FileContent{}, service.NewError(service.CodeDependency, "file service failed", nil)
		}
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return service.FileContent{
		Content:     resp.Body,
		ContentType: contentType,
		SizeBytes:   resp.ContentLength,
	}, nil
}

func writeMultipartFile(writer *multipart.Writer, file service.UploadedFile) error {
	if strings.TrimSpace(file.ChecksumSHA256) != "" {
		if err := writer.WriteField("checksumSha256", strings.TrimSpace(file.ChecksumSHA256)); err != nil {
			return err
		}
	}
	contentType := strings.TrimSpace(file.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name":     "file",
		"filename": file.Filename,
	}))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file.Content)
	return err
}

func (c *Client) setContextHeaders(req *http.Request, reqCtx service.RequestContext) {
	if strings.TrimSpace(reqCtx.RequestID) != "" {
		req.Header.Set("X-Request-Id", strings.TrimSpace(reqCtx.RequestID))
	}
	if strings.TrimSpace(reqCtx.UserID) != "" {
		req.Header.Set("X-User-Id", strings.TrimSpace(reqCtx.UserID))
	}
	callerService := strings.TrimSpace(reqCtx.CallerService)
	if callerService == "" {
		callerService = "knowledge"
	}
	req.Header.Set("X-Caller-Service", callerService)
	serviceToken := strings.TrimSpace(reqCtx.ServiceToken)
	if serviceToken == "" {
		serviceToken = c.serviceToken
	}
	if serviceToken != "" {
		req.Header.Set("X-Service-Token", serviceToken)
	}
	if len(reqCtx.Roles) > 0 {
		req.Header.Set("X-User-Roles", strings.Join(reqCtx.Roles, ","))
	}
	if len(reqCtx.Permissions) > 0 {
		req.Header.Set("X-User-Permissions", strings.Join(reqCtx.Permissions, ","))
	}
	if strings.TrimSpace(reqCtx.ForwardedFor) != "" {
		req.Header.Set("X-Forwarded-For", strings.TrimSpace(reqCtx.ForwardedFor))
	}
	if strings.TrimSpace(reqCtx.ForwardedProto) != "" {
		req.Header.Set("X-Forwarded-Proto", strings.TrimSpace(reqCtx.ForwardedProto))
	}
}
