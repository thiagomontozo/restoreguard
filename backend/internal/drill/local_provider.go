package drill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type LocalProvider struct {
	root     string
	maxBytes int64
}

func NewLocalProvider(root string, maxBytes int64) (*LocalProvider, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if maxBytes <= 0 {
		return nil, errors.New("max bytes must be positive")
	}
	return &LocalProvider{root: abs, maxBytes: maxBytes}, nil
}
func (p *LocalProvider) resolve(externalID string) (string, error) {
	if externalID == "" || strings.Contains(externalID, "..") || strings.ContainsAny(externalID, "/\\") {
		return "", errors.New("unsafe snapshot identifier")
	}
	path := filepath.Join(p.root, externalID)
	rel, err := filepath.Rel(p.root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("snapshot escapes source root")
	}
	return path, nil
}
func (p *LocalProvider) Discover(ctx context.Context) ([]DiscoveredSnapshot, error) {
	entries, err := os.ReadDir(p.root)
	if err != nil {
		return nil, err
	}
	result := []DiscoveredSnapshot{}
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.Size() > p.maxBytes {
			continue
		}
		name := entry.Name()
		if !(strings.HasSuffix(name, ".sql") || strings.HasSuffix(name, ".dump") || strings.HasSuffix(name, ".backup")) {
			continue
		}
		result = append(result, DiscoveredSnapshot{ExternalID: name, Name: name, Type: "POSTGRES_DUMP", CompletedAt: info.ModTime().UTC(), SizeBytes: info.Size()})
	}
	return result, nil
}
func (p *LocalProvider) GetSnapshot(ctx context.Context, id string) (DiscoveredSnapshot, error) {
	path, err := p.resolve(id)
	if err != nil {
		return DiscoveredSnapshot{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return DiscoveredSnapshot{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return DiscoveredSnapshot{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, io.LimitReader(file, p.maxBytes+1)); err != nil {
		return DiscoveredSnapshot{}, err
	}
	return DiscoveredSnapshot{ExternalID: id, Name: id, Type: "POSTGRES_DUMP", CompletedAt: info.ModTime().UTC(), SizeBytes: info.Size(), Checksum: hex.EncodeToString(hash.Sum(nil))}, ctx.Err()
}
func (p *LocalProvider) OpenBackup(_ context.Context, id string) (io.ReadCloser, error) {
	path, err := p.resolve(id)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}
func (p *LocalProvider) ValidateMetadata(_ context.Context, m BackupMetadata) error {
	if m.SizeBytes <= 0 || m.SizeBytes > p.maxBytes {
		return errors.New("invalid backup size")
	}
	if m.Format != "plain" && m.Format != "custom" {
		return errors.New("unsupported PostgreSQL dump format")
	}
	return nil
}
