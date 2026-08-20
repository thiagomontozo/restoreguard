package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var safeKey = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9/_.-]{0,499}$`)

type Local struct {
	root     string
	maxBytes int64
}

func NewLocal(root string, maxBytes int64) (*Local, error) {
	if maxBytes <= 0 {
		return nil, errors.New("max bytes must be positive")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0700); err != nil {
		return nil, err
	}
	return &Local{root: abs, maxBytes: maxBytes}, nil
}
func (l *Local) path(key string) (string, error) {
	if !safeKey.MatchString(key) || strings.Contains(key, "..") || strings.Contains(key, "\\") {
		return "", errors.New("unsafe object key")
	}
	candidate := filepath.Join(l.root, filepath.FromSlash(key))
	rel, err := filepath.Rel(l.root, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("object escapes storage root")
	}
	return candidate, nil
}
func (l *Local) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (ObjectInfo, error) {
	if size < 0 || size > l.maxBytes {
		return ObjectInfo{}, errors.New("artifact size exceeds configured limit")
	}
	path, err := l.path(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return ObjectInfo{}, err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return ObjectInfo{}, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	hash := sha256.New()
	limited := io.LimitReader(reader, l.maxBytes+1)
	written, copyErr := io.Copy(io.MultiWriter(temp, hash), &contextReader{ctx: ctx, reader: limited})
	closeErr := temp.Close()
	if copyErr != nil {
		return ObjectInfo{}, copyErr
	}
	if closeErr != nil {
		return ObjectInfo{}, closeErr
	}
	if written > l.maxBytes || written != size {
		return ObjectInfo{}, errors.New("artifact length mismatch or exceeds limit")
	}
	if err := os.Rename(tempName, path); err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: key, Size: written, SHA256: hex.EncodeToString(hash.Sum(nil)), ContentType: contentType}, nil
}
func (l *Local) Get(_ context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	path, err := l.path(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, ObjectInfo{}, err
	}
	return file, ObjectInfo{Key: key, Size: stat.Size()}, nil
}
func (l *Local) Delete(_ context.Context, key string) error {
	path, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
func (l *Local) Health(_ context.Context) error {
	file, err := os.CreateTemp(l.root, ".health-*")
	if err != nil {
		return fmt.Errorf("local storage: %w", err)
	}
	name := file.Name()
	file.Close()
	return os.Remove(name)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(p)
	}
}
