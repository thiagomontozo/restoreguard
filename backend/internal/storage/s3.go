package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
)

type S3 struct {
	client   *minio.Client
	bucket   string
	maxBytes int64
}
type S3Config struct {
	Endpoint, AccessKey, SecretKey, Bucket string
	UseTLS                                 bool
	MaxBytes                               int64
}

func NewS3(cfg S3Config) (*S3, error) {
	if cfg.MaxBytes <= 0 {
		return nil, errors.New("max bytes must be positive")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: cfg.UseTLS})
	if err != nil {
		return nil, err
	}
	return &S3{client: client, bucket: cfg.Bucket, maxBytes: cfg.MaxBytes}, nil
}
func (s *S3) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (ObjectInfo, error) {
	if !safeKey.MatchString(key) || size < 0 || size > s.maxBytes {
		return ObjectInfo{}, errors.New("unsafe key or artifact size")
	}
	hash := sha256.New()
	info, err := s.client.PutObject(ctx, s.bucket, key, io.TeeReader(reader, hash), size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: key, Size: info.Size, SHA256: hex.EncodeToString(hash.Sum(nil)), ContentType: contentType}, nil
}
func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	if !safeKey.MatchString(key) {
		return nil, ObjectInfo{}, errors.New("unsafe object key")
	}
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, ObjectInfo{}, err
	}
	return obj, ObjectInfo{Key: key, Size: stat.Size, ContentType: stat.ContentType}, nil
}
func (s *S3) Delete(ctx context.Context, key string) error {
	if !safeKey.MatchString(key) {
		return errors.New("unsafe object key")
	}
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
func (s *S3) Health(ctx context.Context) error {
	_, err := s.client.BucketExists(ctx, s.bucket)
	return err
}
func (s *S3) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
	}
	return nil
}
