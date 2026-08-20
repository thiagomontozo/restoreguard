package drill

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var safeIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type CommandRunner interface {
	Run(context.Context, string, ...string) (string, error)
}
type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	return strings.TrimSpace(output.String()), err
}

type DockerSandboxExecutor struct {
	runner       CommandRunner
	allowedImage string
}

func NewDockerSandboxExecutor(allowedImage string) *DockerSandboxExecutor {
	return &DockerSandboxExecutor{runner: execRunner{}, allowedImage: allowedImage}
}
func NewDockerSandboxExecutorWithRunner(allowedImage string, runner CommandRunner) *DockerSandboxExecutor {
	return &DockerSandboxExecutor{runner: runner, allowedImage: allowedImage}
}
func (e *DockerSandboxExecutor) Create(ctx context.Context, spec SandboxSpec) (Sandbox, error) {
	if spec.Image != e.allowedImage {
		return Sandbox{}, errors.New("image is not allowlisted")
	}
	if !safeIdentifier.MatchString(spec.DrillID) || spec.CPUs <= 0 || spec.CPUs > 4 || spec.MemoryBytes < 128*1024*1024 || spec.MemoryBytes > 8*1024*1024*1024 {
		return Sandbox{}, errors.New("invalid sandbox specification")
	}
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	base := "restoreguard-" + spec.DrillID + "-" + suffix
	network := base + "-net"
	volume := base + "-data"
	container := base + "-pg"
	cleanup := func() {
		cleanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = e.runner.Run(cleanCtx, "docker", "rm", "-f", container)
		_, _ = e.runner.Run(cleanCtx, "docker", "network", "rm", network)
		_, _ = e.runner.Run(cleanCtx, "docker", "volume", "rm", volume)
	}
	if _, err := e.runner.Run(ctx, "docker", "network", "create", "--internal", "--label", "com.restoreguard.managed=true", "--label", "com.restoreguard.purpose=restore-sandbox", network); err != nil {
		return Sandbox{}, fmt.Errorf("create network: %w", err)
	}
	if _, err := e.runner.Run(ctx, "docker", "volume", "create", "--label", "com.restoreguard.managed=true", "--label", "com.restoreguard.purpose=restore-sandbox", volume); err != nil {
		cleanup()
		return Sandbox{}, fmt.Errorf("create volume: %w", err)
	}
	cpu := strconv.FormatFloat(spec.CPUs, 'f', 2, 64)
	memory := strconv.FormatInt(spec.MemoryBytes, 10)
	args := []string{"run", "-d", "--name", container, "--network", network, "--cpus", cpu, "--memory", memory, "--pids-limit", "256", "--read-only", "--tmpfs", "/tmp:rw,noexec,nosuid,size=64m", "--tmpfs", "/var/run/postgresql:rw,nosuid,nodev,size=16m", "--mount", "type=volume,source=" + volume + ",target=/var/lib/postgresql/data", "--label", "com.restoreguard.managed=true", "--label", "com.restoreguard.purpose=restore-sandbox", "--label", "com.restoreguard.drill-id=" + spec.DrillID, "-e", "POSTGRES_PASSWORD=restoreguard-sandbox-only", "-e", "POSTGRES_DB=recovery", spec.Image}
	if _, err := e.runner.Run(ctx, "docker", args...); err != nil {
		cleanup()
		return Sandbox{}, fmt.Errorf("create container: %w", err)
	}
	created := time.Now().UTC()
	readyTimeout := spec.Timeout
	if readyTimeout <= 0 || readyTimeout > 5*time.Minute {
		readyTimeout = 90 * time.Second
	}
	readyCtx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()
	for {
		// pg_isready can briefly succeed against the entrypoint's temporary
		// initialization server. Wait for the explicit init-complete marker before
		// accepting readiness from the final PostgreSQL process.
		logs, logErr := e.runner.Run(readyCtx, "docker", "logs", container)
		if logErr == nil && strings.Contains(logs, "PostgreSQL init process complete; ready for start up.") {
			if _, err := e.runner.Run(readyCtx, "docker", "exec", container, "pg_isready", "-U", "postgres", "-d", "recovery"); err == nil {
				ready := time.Now().UTC()
				return Sandbox{ID: uuid.NewString(), ContainerName: container, NetworkName: network, VolumeName: volume, Host: container, Port: 5432, CreatedAt: created, ReadyAt: ready}, nil
			}
		}
		select {
		case <-readyCtx.Done():
			cleanup()
			return Sandbox{}, fmt.Errorf("sandbox readiness: %w", readyCtx.Err())
		case <-time.After(time.Second):
		}
	}
}
func (e *DockerSandboxExecutor) Destroy(ctx context.Context, s Sandbox) error {
	for _, value := range []string{s.ContainerName, s.NetworkName, s.VolumeName} {
		if value != "" && !strings.HasPrefix(value, "restoreguard-") {
			return errors.New("refusing to remove unmanaged Docker resource")
		}
	}
	var errs []string
	if s.ContainerName != "" {
		if out, err := e.runner.Run(ctx, "docker", "rm", "-f", s.ContainerName); err != nil && !strings.Contains(out, "No such container") {
			errs = append(errs, "container: "+err.Error())
		}
	}
	if s.NetworkName != "" {
		if out, err := e.runner.Run(ctx, "docker", "network", "rm", s.NetworkName); err != nil && !strings.Contains(out, "not found") {
			errs = append(errs, "network: "+err.Error())
		}
	}
	if s.VolumeName != "" {
		if out, err := e.runner.Run(ctx, "docker", "volume", "rm", s.VolumeName); err != nil && !strings.Contains(out, "no such volume") {
			errs = append(errs, "volume: "+err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// CopyPostgresBackup and RestorePostgresPlain are deliberately typed operations.
// Callers cannot supply command arguments, mounts, images, or a destination database.
func (e *DockerSandboxExecutor) CopyPostgresBackup(ctx context.Context, sandbox Sandbox, localPath string) error {
	if !strings.HasPrefix(sandbox.ContainerName, "restoreguard-") {
		return errors.New("unmanaged sandbox")
	}
	if _, err := e.runner.Run(ctx, "docker", "cp", localPath, sandbox.ContainerName+":/tmp/restoreguard-backup.sql"); err != nil {
		return fmt.Errorf("copy PostgreSQL backup: %w", err)
	}
	return nil
}
func (e *DockerSandboxExecutor) RestorePostgresPlain(ctx context.Context, sandbox Sandbox) error {
	if !strings.HasPrefix(sandbox.ContainerName, "restoreguard-") {
		return errors.New("unmanaged sandbox")
	}
	if _, err := e.runner.Run(ctx, "docker", "exec", sandbox.ContainerName, "psql", "-U", "postgres", "-d", "recovery", "-v", "ON_ERROR_STOP=1", "-f", "/tmp/restoreguard-backup.sql"); err != nil {
		return fmt.Errorf("restore PostgreSQL plain dump: %w", err)
	}
	return nil
}
func (e *DockerSandboxExecutor) ValidatePostgresConnectivity(ctx context.Context, sandbox Sandbox) error {
	if !strings.HasPrefix(sandbox.ContainerName, "restoreguard-") {
		return errors.New("unmanaged sandbox")
	}
	if _, err := e.runner.Run(ctx, "docker", "exec", sandbox.ContainerName, "psql", "-U", "postgres", "-d", "recovery", "-v", "ON_ERROR_STOP=1", "-Atc", "SELECT 1"); err != nil {
		return fmt.Errorf("validate PostgreSQL connectivity: %w", err)
	}
	return nil
}
