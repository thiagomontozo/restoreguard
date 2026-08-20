package drill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/thiagomontozo/restoreguard/backend/internal/domain"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.dump")
	data := []byte("synthetic")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	engine := NewValidationEngine(100)
	result := engine.Run(context.Background(), ValidationCheck{Name: "checksum", Type: SHA256, Path: path, ExpectedSHA256: hex.EncodeToString(sum[:]), Timeout: time.Second, Required: true})
	if result.Status != domain.ValidationPass {
		t.Fatalf("unexpected %s: %s", result.Status, result.Summary)
	}
}
func TestValidationFailure(t *testing.T) {
	engine := NewValidationEngine(100)
	result := engine.Run(context.Background(), ValidationCheck{Name: "missing", Type: FileExists, Path: "missing", Timeout: time.Second, Required: true})
	if result.Status != domain.ValidationFail {
		t.Fatalf("expected fail, got %s", result.Status)
	}
}
