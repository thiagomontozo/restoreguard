//go:build integration

package storage

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"io"
	"os"
	"testing"
	"time"
)

func TestS3CompatibleStreaming(t *testing.T) {
	endpoint := os.Getenv("RESTOREGUARD_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Fatal("RESTOREGUARD_TEST_S3_ENDPOINT is required")
	}
	bucket := "restoreguard-test-" + uuid.NewString()[:8]
	store, err := NewS3(S3Config{Endpoint: endpoint, AccessKey: "restoreguardtest", SecretKey: "restoreguard-test-minio-only", Bucket: bucket, MaxBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = store.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}
	payload := []byte("minio recovery evidence")
	info, err := store.Put(ctx, "org/drill/evidence.json", bytes.NewReader(payload), int64(len(payload)), "application/json")
	if err != nil {
		t.Fatal(err)
	}
	if len(info.SHA256) != 64 {
		t.Fatal("checksum missing")
	}
	reader, _, err := store.Get(ctx, info.Key)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(data, payload) {
		t.Fatal("S3 object mismatch")
	}
	if err = store.Delete(ctx, info.Key); err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.Get(ctx, info.Key); err == nil {
		t.Fatal("missing object should return an error")
	}
}
