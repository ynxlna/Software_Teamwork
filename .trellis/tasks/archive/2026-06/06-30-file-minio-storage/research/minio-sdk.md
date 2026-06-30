# MinIO SDK Research

## Topic

Select and wire the official Go SDK for File Service MinIO object storage.

## Findings

- Official module path is `github.com/minio/minio-go/v7`.
- Available module versions checked with `go list -m -versions github.com/minio/minio-go/v7`; latest available version in the current Go proxy result is `v7.2.1`.
- Official quickstart initializes the client with `minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(...), Secure: useSSL})`.
- `minio.PutObjectOptions` supports content-type metadata and checksum-related options such as `SendContentMd5`.
- Project docs require File Service to be the MinIO boundary and hide bucket, object key, internal URL, access key, and secret key from responses/logs.

## Decision

Use `github.com/minio/minio-go/v7@v7.2.1` in `services/file/go.mod`.

## Implementation Notes

- Keep SDK imports under `services/file/internal/platform/storage` and `cmd/server` wiring only.
- Preserve the existing `service.ObjectStore` interface.
- Convert SDK not-found responses to `service.ErrNotFound`.
- Convert non-not-found SDK failures to sanitized dependency errors without embedding bucket/object key/endpoint details in returned error strings.
- Configure HTTP timeout through the MinIO client transport and continue to honor caller `context.Context`.

## References

- https://github.com/minio/minio-go
- https://pkg.go.dev/github.com/minio/minio-go/v7
- https://docs.min.io/aistor/developers/sdk/go/api/
