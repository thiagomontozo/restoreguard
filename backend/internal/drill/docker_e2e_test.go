//go:build e2e

package drill

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestDockerSandboxExecutorLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	executor := NewDockerSandboxExecutor(postgresImage)
	sandbox, err := executor.Create(ctx, SandboxSpec{DrillID: "executor123", OrganizationID: "test-org", Image: postgresImage, CPUs: 0.5, MemoryBytes: 256 * 1024 * 1024, Timeout: 2 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, c := context.WithTimeout(context.Background(), 45*time.Second)
		defer c()
		_ = executor.Destroy(cleanupCtx, sandbox)
	})
	inspect := dockerCommand(t, ctx, "inspect", "--format", "{{json .Config.Labels}}|{{.HostConfig.NetworkMode}}|{{.HostConfig.Memory}}|{{.HostConfig.ReadonlyRootfs}}", sandbox.ContainerName)
	for _, expected := range []string{"com.restoreguard.managed", "restore-sandbox", sandbox.NetworkName, "268435456", "true"} {
		if !strings.Contains(inspect, expected) {
			t.Fatalf("inspect missing %q: %s", expected, inspect)
		}
	}
	if err = executor.Destroy(ctx, sandbox); err != nil {
		t.Fatal(err)
	}
	if exec.CommandContext(ctx, "docker", "inspect", sandbox.ContainerName).Run() == nil {
		t.Fatal("container still exists")
	}
	if exec.CommandContext(ctx, "docker", "network", "inspect", sandbox.NetworkName).Run() == nil {
		t.Fatal("network still exists")
	}
	if exec.CommandContext(ctx, "docker", "volume", "inspect", sandbox.VolumeName).Run() == nil {
		t.Fatal("volume still exists")
	}
}
