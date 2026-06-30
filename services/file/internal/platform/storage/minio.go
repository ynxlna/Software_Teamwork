package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/file/internal/service"
)

const defaultMinIOTimeout = 10 * time.Second

var errMinIODependency = errors.New("minio dependency failed")

type MinIOClientConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Region    string
	Timeout   time.Duration
}

type MinIOObjectClient interface {
	PutObject(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName string, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, minio.ObjectInfo, error)
	RemoveObject(ctx context.Context, bucketName string, objectName string, opts minio.RemoveObjectOptions) error
}

type SDKMinIOClient struct {
	client *minio.Client
}

type MinIOStore struct {
	client MinIOObjectClient
	bucket string
}

func NewMinIOClient(cfg MinIOClientConfig) (*SDKMinIOClient, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	accessKey := strings.TrimSpace(cfg.AccessKey)
	secretKey := strings.TrimSpace(cfg.SecretKey)
	if endpoint == "" || accessKey == "" || secretKey == "" {
		return nil, errors.New("minio client configuration is incomplete")
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultMinIOTimeout
	}
	transport := http.DefaultTransport
	if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = defaultTransport.Clone()
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:    cfg.UseSSL,
		Region:    strings.TrimSpace(cfg.Region),
		Transport: timeoutRoundTripper{base: transport, timeout: timeout},
	})
	if err != nil {
		return nil, errors.New("minio client initialization failed")
	}
	return &SDKMinIOClient{client: client}, nil
}

func NewMinIOStore(client MinIOObjectClient, bucket string) (*MinIOStore, error) {
	if client == nil {
		return nil, errors.New("minio client is required")
	}
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, errors.New("minio bucket is required")
	}
	return &MinIOStore{client: client, bucket: bucket}, nil
}

func (c *SDKMinIOClient) PutObject(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	return c.client.PutObject(ctx, bucketName, objectName, reader, objectSize, opts)
}

func (c *SDKMinIOClient) GetObject(ctx context.Context, bucketName string, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, minio.ObjectInfo, error) {
	object, err := c.client.GetObject(ctx, bucketName, objectName, opts)
	if err != nil {
		return nil, minio.ObjectInfo{}, err
	}
	info, err := object.Stat()
	if err != nil {
		_ = object.Close()
		return nil, minio.ObjectInfo{}, err
	}
	return object, info, nil
}

func (c *SDKMinIOClient) RemoveObject(ctx context.Context, bucketName string, objectName string, opts minio.RemoveObjectOptions) error {
	return c.client.RemoveObject(ctx, bucketName, objectName, opts)
}

func (s *MinIOStore) Put(ctx context.Context, key string, body io.Reader, contentType string, sizeBytes int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if body == nil {
		return service.ErrNotFound
	}
	counter := &countingReader{reader: body}
	_, err := s.client.PutObject(ctx, s.bucket, key, counter, sizeBytes, minio.PutObjectOptions{
		ContentType:    contentType,
		SendContentMd5: true,
	})
	if err != nil {
		return mapMinIOError(err)
	}
	if sizeBytes >= 0 && counter.n != sizeBytes {
		return service.ErrConflict
	}
	return nil
}

func (s *MinIOStore) Get(ctx context.Context, key string) (service.StoredObject, error) {
	if err := ctx.Err(); err != nil {
		return service.StoredObject{}, err
	}
	body, info, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return service.StoredObject{}, mapMinIOError(err)
	}
	contentType := strings.TrimSpace(info.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return service.StoredObject{Body: body, ContentType: contentType, SizeBytes: info.Size}, nil
}

func (s *MinIOStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return mapMinIOError(err)
	}
	return nil
}

type countingReader struct {
	reader io.Reader
	n      int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.n += int64(n)
	return n, err
}

type timeoutRoundTripper struct {
	base    http.RoundTripper
	timeout time.Duration
}

func (t timeoutRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.timeout <= 0 {
		return t.base.RoundTrip(req)
	}
	if _, ok := req.Context().Deadline(); ok {
		return t.base.RoundTrip(req)
	}
	ctx, cancel := context.WithTimeout(req.Context(), t.timeout)
	resp, err := t.base.RoundTrip(req.WithContext(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = cancelOnCloseReadCloser{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

func mapMinIOError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	response := minio.ToErrorResponse(err)
	switch response.Code {
	case "NoSuchKey", "NoSuchObject":
		return service.ErrNotFound
	default:
		return errMinIODependency
	}
}

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r cancelOnCloseReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.cancel()
	return err
}
