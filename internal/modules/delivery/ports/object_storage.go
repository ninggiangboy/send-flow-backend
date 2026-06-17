package ports

import (
	"context"
	"io"
)

// ObjectStorage is a subset interface for storing and retrieving email attachment blobs.
type ObjectStorage interface {
	PutObject(ctx context.Context, key string, body io.Reader, contentType string) error
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	DeleteObject(ctx context.Context, key string) error
}
