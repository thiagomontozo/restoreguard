package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestLocalStreamingRoundTrip(t *testing.T) {
	store, err := NewLocal(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("recovery evidence")
	info, err := store.Put(context.Background(), "org/drill/evidence.txt", bytes.NewReader(payload), int64(len(payload)), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	if len(info.SHA256) != 64 {
		t.Fatal("missing checksum")
	}
	reader, got, err := store.Get(context.Background(), info.Key)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(reader)
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) || got.Size != int64(len(payload)) {
		t.Fatal("object mismatch")
	}
	if err := store.Delete(context.Background(), info.Key); err != nil {
		t.Fatal(err)
	}
}
func TestLocalRejectsTraversalAndOversize(t *testing.T) {
	store, _ := NewLocal(t.TempDir(), 4)
	if _, err := store.Put(context.Background(), "../escape", bytes.NewReader([]byte("x")), 1, "text/plain"); err == nil {
		t.Fatal("expected traversal rejection")
	}
	if _, err := store.Put(context.Background(), "safe/key", bytes.NewReader([]byte("12345")), 5, "text/plain"); err == nil {
		t.Fatal("expected size rejection")
	}
}
