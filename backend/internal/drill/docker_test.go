package drill

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	calls  [][]string
	failAt int
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if len(args) > 0 && args[0] == "logs" {
		return "PostgreSQL init process complete; ready for start up.", nil
	}
	return "", nil
}
func TestDockerExecutorUsesIsolationAndLabels(t *testing.T) {
	runner := &fakeRunner{}
	exec := NewDockerSandboxExecutorWithRunner("postgres:17.6-alpine", runner)
	sandbox, err := exec.Create(context.Background(), SandboxSpec{DrillID: "drill123", OrganizationID: "org", Image: "postgres:17.6-alpine", CPUs: 1, MemoryBytes: 256 * 1024 * 1024, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, call := range runner.calls {
		joined += strings.Join(call, " ") + "\n"
	}
	for _, required := range []string{"--internal", "--read-only", "--pids-limit 256", "com.restoreguard.managed=true", "--cpus 1.00", "--memory 268435456"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("missing %q in %s", required, joined)
		}
	}
	if strings.Contains(joined, "--privileged") || strings.Contains(joined, "--network host") {
		t.Fatal("unsafe Docker arguments")
	}
	if err := exec.Destroy(context.Background(), sandbox); err != nil {
		t.Fatal(err)
	}
}
func TestDockerExecutorRejectsImageFromCaller(t *testing.T) {
	exec := NewDockerSandboxExecutorWithRunner("postgres:17.6-alpine", &fakeRunner{})
	_, err := exec.Create(context.Background(), SandboxSpec{DrillID: "safe", Image: "evil:latest", CPUs: 1, MemoryBytes: 256 * 1024 * 1024})
	if err == nil {
		t.Fatal("expected image rejection")
	}
}

type failingRunner struct{}

func (failingRunner) Run(ctx context.Context, _ string, _ ...string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		return "", context.DeadlineExceeded
	}
}
func TestDockerExecutorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	executor := NewDockerSandboxExecutorWithRunner("postgres:17.6-alpine", failingRunner{})
	_, err := executor.Create(ctx, SandboxSpec{DrillID: "cancel123", Image: "postgres:17.6-alpine", CPUs: 1, MemoryBytes: 256 * 1024 * 1024, Timeout: time.Second})
	if err == nil {
		t.Fatal("expected cancellation")
	}
}
