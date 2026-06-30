package storage_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/file/internal/platform/storage"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/file/internal/service"
)

func TestMinIOStorePutSendsContentTypeChecksumAndSize(t *testing.T) {
	client := &fakeMinIOClient{}
	client.put = func(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
		body, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if bucketName != "file-objects" || objectName != "files/file_1" {
			t.Fatalf("bucket/key = %q/%q", bucketName, objectName)
		}
		if string(body) != "content" || objectSize != int64(len("content")) {
			t.Fatalf("body=%q size=%d", string(body), objectSize)
		}
		if opts.ContentType != "text/plain" || !opts.SendContentMd5 {
			t.Fatalf("PutObjectOptions = %+v", opts)
		}
		return minio.UploadInfo{Size: objectSize}, nil
	}
	store, err := storage.NewMinIOStore(client, "file-objects")
	if err != nil {
		t.Fatalf("NewMinIOStore() error = %v", err)
	}

	if err := store.Put(context.Background(), "files/file_1", strings.NewReader("content"), "text/plain", int64(len("content"))); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
}

func TestMinIOStorePutDetectsSizeMismatch(t *testing.T) {
	client := &fakeMinIOClient{}
	client.put = func(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
		if _, err := io.Copy(io.Discard, reader); err != nil {
			t.Fatalf("Copy() error = %v", err)
		}
		return minio.UploadInfo{Size: objectSize}, nil
	}
	store, err := storage.NewMinIOStore(client, "file-objects")
	if err != nil {
		t.Fatalf("NewMinIOStore() error = %v", err)
	}

	err = store.Put(context.Background(), "files/file_1", strings.NewReader("content"), "text/plain", int64(len("content")+1))
	if !errors.Is(err, service.ErrConflict) {
		t.Fatalf("Put() error = %v, want ErrConflict", err)
	}
}

func TestMinIOStoreGetReturnsObjectMetadata(t *testing.T) {
	client := &fakeMinIOClient{}
	client.get = func(ctx context.Context, bucketName string, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, minio.ObjectInfo, error) {
		if bucketName != "file-objects" || objectName != "files/file_1" {
			t.Fatalf("bucket/key = %q/%q", bucketName, objectName)
		}
		return io.NopCloser(strings.NewReader("content")), minio.ObjectInfo{
			Size:        int64(len("content")),
			ContentType: "text/plain",
		}, nil
	}
	store, err := storage.NewMinIOStore(client, "file-objects")
	if err != nil {
		t.Fatalf("NewMinIOStore() error = %v", err)
	}

	object, err := store.Get(context.Background(), "files/file_1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer object.Body.Close()
	body, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != "content" || object.ContentType != "text/plain" || object.SizeBytes != int64(len("content")) {
		t.Fatalf("object = %+v, body = %q", object, string(body))
	}
}

func TestMinIOStoreMapsNotFoundWithoutLeakingStorageDetails(t *testing.T) {
	client := &fakeMinIOClient{}
	client.get = func(ctx context.Context, bucketName string, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, minio.ObjectInfo, error) {
		return nil, minio.ObjectInfo{}, minio.ErrorResponse{
			Code:       "NoSuchKey",
			Message:    "object does not exist",
			StatusCode: http.StatusNotFound,
			BucketName: "secret-bucket",
			Key:        "files/file_1",
		}
	}
	store, err := storage.NewMinIOStore(client, "secret-bucket")
	if err != nil {
		t.Fatalf("NewMinIOStore() error = %v", err)
	}

	_, err = store.Get(context.Background(), "files/file_1")
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
	assertNoStorageLeak(t, err.Error())
}

func TestMinIOStoreMapsDependencyErrorWithoutLeakingStorageDetails(t *testing.T) {
	client := &fakeMinIOClient{}
	client.remove = func(ctx context.Context, bucketName string, objectName string, opts minio.RemoveObjectOptions) error {
		return minio.ErrorResponse{
			Code:       "AccessDenied",
			Message:    "access denied for secret-bucket/files/file_1",
			StatusCode: http.StatusForbidden,
			BucketName: "secret-bucket",
			Key:        "files/file_1",
		}
	}
	store, err := storage.NewMinIOStore(client, "secret-bucket")
	if err != nil {
		t.Fatalf("NewMinIOStore() error = %v", err)
	}

	err = store.Delete(context.Background(), "files/file_1")
	if err == nil || errors.Is(err, service.ErrNotFound) {
		t.Fatalf("Delete() error = %v, want sanitized dependency error", err)
	}
	assertNoStorageLeak(t, err.Error())
}

func TestMinIOStoreHonorsCancelledContext(t *testing.T) {
	client := &fakeMinIOClient{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store, err := storage.NewMinIOStore(client, "file-objects")
	if err != nil {
		t.Fatalf("NewMinIOStore() error = %v", err)
	}

	err = store.Put(ctx, "files/file_1", strings.NewReader("content"), "text/plain", int64(len("content")))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Put() error = %v, want context.Canceled", err)
	}
}

func TestMinIOStorePreservesTimeoutErrors(t *testing.T) {
	client := &fakeMinIOClient{}
	client.put = func(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
		return minio.UploadInfo{}, context.DeadlineExceeded
	}
	store, err := storage.NewMinIOStore(client, "file-objects")
	if err != nil {
		t.Fatalf("NewMinIOStore() error = %v", err)
	}

	err = store.Put(context.Background(), "files/file_1", strings.NewReader("content"), "text/plain", int64(len("content")))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Put() error = %v, want context.DeadlineExceeded", err)
	}
}

type fakeMinIOClient struct {
	put    func(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	get    func(ctx context.Context, bucketName string, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, minio.ObjectInfo, error)
	remove func(ctx context.Context, bucketName string, objectName string, opts minio.RemoveObjectOptions) error
}

func (c *fakeMinIOClient) PutObject(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	if c.put == nil {
		return minio.UploadInfo{}, errors.New("unexpected PutObject call")
	}
	return c.put(ctx, bucketName, objectName, reader, objectSize, opts)
}

func (c *fakeMinIOClient) GetObject(ctx context.Context, bucketName string, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, minio.ObjectInfo, error) {
	if c.get == nil {
		return nil, minio.ObjectInfo{}, errors.New("unexpected GetObject call")
	}
	return c.get(ctx, bucketName, objectName, opts)
}

func (c *fakeMinIOClient) RemoveObject(ctx context.Context, bucketName string, objectName string, opts minio.RemoveObjectOptions) error {
	if c.remove == nil {
		return errors.New("unexpected RemoveObject call")
	}
	return c.remove(ctx, bucketName, objectName, opts)
}

func assertNoStorageLeak(t *testing.T, value string) {
	t.Helper()
	for _, forbidden := range []string{"secret-bucket", "files/file_1", "access-key", "secret-key", "minio:9000"} {
		if strings.Contains(value, forbidden) {
			t.Fatalf("error leaked %q: %s", forbidden, value)
		}
	}
}
