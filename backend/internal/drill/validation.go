package drill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/thiagomontozo/restoreguard/backend/internal/domain"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type ValidationType string

const (
	FileExists           ValidationType = "FILE_EXISTS"
	FileSize             ValidationType = "FILE_SIZE"
	SHA256               ValidationType = "SHA256"
	PostgresConnectivity ValidationType = "POSTGRES_CONNECTIVITY"
	PostgresTableExists  ValidationType = "POSTGRES_TABLE_EXISTS"
	PostgresRowCount     ValidationType = "POSTGRES_ROW_COUNT"
	HTTPHealth           ValidationType = "HTTP_HEALTH"
)

type ValidationCheck struct {
	Name                 string
	Type                 ValidationType
	Required             bool
	Timeout              time.Duration
	Path, ExpectedSHA256 string
	MinBytes             int64
	URL                  string
}
type ValidationResult struct {
	Name                   string
	Status                 domain.ValidationStatus
	Summary                string
	StartedAt, CompletedAt time.Time
}
type ValidationEngine struct{ maxFileBytes int64 }

func NewValidationEngine(maxFileBytes int64) *ValidationEngine {
	return &ValidationEngine{maxFileBytes: maxFileBytes}
}
func (e *ValidationEngine) Run(ctx context.Context, check ValidationCheck) ValidationResult {
	started := time.Now().UTC()
	result := ValidationResult{Name: check.Name, StartedAt: started, Status: domain.ValidationInconclusive}
	checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
	defer cancel()
	var err error
	select {
	case <-checkCtx.Done():
		err = checkCtx.Err()
		result.CompletedAt = time.Now().UTC()
		result.Status = domain.ValidationInconclusive
		result.Summary = "Validation did not complete"
		return result
	default:
	}
	switch check.Type {
	case FileExists:
		_, err = os.Stat(check.Path)
	case FileSize:
		var info os.FileInfo
		info, err = os.Stat(check.Path)
		if err == nil && info.Size() < check.MinBytes {
			err = fmt.Errorf("file size %d is below %d", info.Size(), check.MinBytes)
		}
	case SHA256:
		err = e.verifySHA(checkCtx, check.Path, check.ExpectedSHA256)
	case HTTPHealth:
		err = e.httpHealth(checkCtx, check.URL)
	default:
		err = errors.New("validation type requires a typed PostgreSQL validator")
	}
	result.CompletedAt = time.Now().UTC()
	if err == nil {
		result.Status = domain.ValidationPass
		result.Summary = "Validation passed"
	} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		result.Status = domain.ValidationInconclusive
		result.Summary = "Validation did not complete"
	} else {
		result.Status = domain.ValidationFail
		result.Summary = err.Error()
	}
	return result
}
func (e *ValidationEngine) verifySHA(ctx context.Context, path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(&contextReader{ctx: ctx, r: file}, e.maxFileBytes+1))
	if err != nil {
		return err
	}
	if n > e.maxFileBytes {
		return errors.New("file exceeds validation limit")
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expected) {
		return errors.New("checksum mismatch")
	}
	return nil
}
func (e *ValidationEngine) httpHealth(ctx context.Context, url string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	host := request.URL.Hostname()
	ip := net.ParseIP(host)
	if ip == nil || (!ip.IsLoopback() && !ip.IsPrivate()) {
		return errors.New("HTTP health target is not an authorized sandbox address")
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirects disabled") }}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("health returned %d", response.StatusCode)
	}
	return nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.r.Read(p)
	}
}
