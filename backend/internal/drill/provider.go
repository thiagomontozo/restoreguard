package drill

import (
	"context"
	"io"
	"time"
)

type DiscoveredSnapshot struct {
	ExternalID, Name, Type string
	CompletedAt            time.Time
	SizeBytes              int64
	Checksum               string
	Metadata               map[string]string
}
type BackupMetadata struct {
	SizeBytes        int64
	Checksum, Format string
}
type BackupProvider interface {
	Discover(context.Context) ([]DiscoveredSnapshot, error)
	GetSnapshot(context.Context, string) (DiscoveredSnapshot, error)
	OpenBackup(context.Context, string) (io.ReadCloser, error)
	ValidateMetadata(context.Context, BackupMetadata) error
}
