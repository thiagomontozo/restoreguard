package storage

import (
	"context"
	"io"
)

type ObjectInfo struct {
	Key         string
	Size        int64
	SHA256      string
	ContentType string
}
type ObjectStorage interface {
	Put(context.Context, string, io.Reader, int64, string) (ObjectInfo, error)
	Get(context.Context, string) (io.ReadCloser, ObjectInfo, error)
	Delete(context.Context, string) error
	Health(context.Context) error
}
