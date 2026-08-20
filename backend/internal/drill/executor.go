package drill

import (
	"context"
	"time"
)

type SandboxSpec struct {
	DrillID, OrganizationID string
	Image                   string
	CPUs                    float64
	MemoryBytes             int64
	Timeout                 time.Duration
}
type Sandbox struct {
	ID, ContainerName, NetworkName, VolumeName, Host string
	Port                                             int
	CreatedAt, ReadyAt                               time.Time
}
type SandboxExecutor interface {
	Create(context.Context, SandboxSpec) (Sandbox, error)
	Destroy(context.Context, Sandbox) error
}
