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

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

const defaultTimeout = 30 * time.Second

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string, httpClient *http.Client) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("file service URL must be an absolute http(s) URL")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		baseURL:    strings.TrimRight(parsed.String(), "/"),
		httpClient: httpClient,
	}, nil
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
	setContextHeaders(req, reqCtx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return service.FileObject{}, service.NewError(service.CodeDependency, "file service unavailable", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		if resp.StatusCode == http.StatusBadRequest {
			return service.FileObject{}, service.NewError(service.CodeValidation, "file upload is invalid", nil)
		}
		return service.FileObject{}, service.NewError(service.CodeDependency, "file service failed", nil)
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
	if strings.TrimSpace(fileID) == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/internal/v1/files/"+url.PathEscape(fileID), nil)
	if err != nil {
		return service.NewError(service.CodeDependency, "file service request failed", err)
	}
	setContextHeaders(req, reqCtx)
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

func (c *Client) ReadFileContent(ctx context.Context, reqCtx service.RequestContext, fileID string) (service.FileContent, error) {
	if strings.TrimSpace(fileID) == "" {
		return service.FileContent{}, service.NewError(service.CodeNotFound, "file not found", nil)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/files/"+url.PathEscape(fileID)+"/content", nil)
	if err != nil {
		return service.FileContent{}, service.NewError(service.CodeDependency, "file service request failed", err)
	}
	setContextHeaders(req, reqCtx)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return service.FileContent{}, service.NewError(service.CodeDependency, "file service unavailable", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		if resp.StatusCode == http.StatusNotFound {
			return service.FileContent{}, service.NewError(service.CodeNotFound, "file content not found", nil)
		}
		return service.FileContent{}, service.NewError(service.CodeDependency, "file service failed", nil)
	}

	filename := filenameFromContentDisposition(resp.Header.Get("Content-Disposition"))
	return service.FileContent{
		Filename:    filename,
		ContentType: resp.Header.Get("Content-Type"),
		SizeBytes:   resp.ContentLength,
		Content:     resp.Body,
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

func filenameFromContentDisposition(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(value)
	if err != nil {
		return ""
	}
	return params["filename"]
}

func setContextHeaders(req *http.Request, reqCtx service.RequestContext) {
	if strings.TrimSpace(reqCtx.RequestID) != "" {
		req.Header.Set("X-Request-Id", strings.TrimSpace(reqCtx.RequestID))
	}
	if strings.TrimSpace(reqCtx.UserID) != "" {
		req.Header.Set("X-User-Id", strings.TrimSpace(reqCtx.UserID))
	}
	if strings.TrimSpace(reqCtx.CallerService) != "" {
		req.Header.Set("X-Caller-Service", strings.TrimSpace(reqCtx.CallerService))
	} else {
		req.Header.Set("X-Caller-Service", "document")
	}
	if strings.TrimSpace(reqCtx.ServiceToken) != "" {
		req.Header.Set("X-Service-Token", strings.TrimSpace(reqCtx.ServiceToken))
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
