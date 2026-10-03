// Package storage stores bill documents in object storage. Keys are opaque and
// never exposed as public URLs; clients only ever get short-lived signed links.
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound is returned when an object does not exist.
var ErrNotFound = errors.New("storage: object not found")

// Store is the minimal object-storage contract the app needs.
type Store interface {
	Put(ctx context.Context, key, contentType string, body io.ReadSeeker, size int64) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete removes an object; deleting a missing object is not an error.
	Delete(ctx context.Context, key string) error
}

// Presigner is implemented by stores that can hand the browser direct, signed URLs,
// so file bytes don't pass through the API.
type Presigner interface {
	PresignPut(ctx context.Context, key, contentType string, size int64, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, key, contentType, disposition string, ttl time.Duration) (string, error)
}
