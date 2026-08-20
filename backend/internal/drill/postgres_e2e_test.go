//go:build e2e

package drill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thiagomontozo/restoreguard/backend/internal/domain"
)

const postgresImage = "postgres:17.6-alpine"

func dockerCommand(t *testing.T, ctx context.Context, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
func waitPostgres(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	for range 45 {
		logs, _ := exec.CommandContext(ctx, "docker", "logs", name).CombinedOutput()
		if strings.Contains(string(logs), "PostgreSQL init process complete; ready for start up.") {
			cmd := exec.CommandContext(ctx, "docker", "exec", name, "pg_isready", "-U", "postgres")
			if cmd.Run() == nil {
				return
			}
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("PostgreSQL %s did not become ready", name)
}
func newPostgres(t *testing.T, ctx context.Context, name, network, volume, db string) {
	t.Helper()
	args := []string{"run", "-d", "--name", name, "--network", network, "--cpus", "1", "--memory", "512m", "--pids-limit", "256", "--label", "com.restoreguard.managed=true", "--label", "com.restoreguard.purpose=e2e", "-e", "POSTGRES_PASSWORD=synthetic-test-only", "-e", "POSTGRES_DB=" + db}
	if volume != "" {
		args = append(args, "--mount", "type=volume,source="+volume+",target=/var/lib/postgresql/data")
	} else {
		args = append(args, "--tmpfs", "/var/lib/postgresql/data:rw,nosuid,nodev,size=512m")
	}
	args = append(args, postgresImage)
	dockerCommand(t, ctx, args...)
	waitPostgres(t, ctx, name)
}
func TestPostgresRecoveryE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Docker E2E")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	network := "restoreguard-e2e-" + suffix + "-net"
	source := "restoreguard-e2e-" + suffix + "-source"
	restore := "restoreguard-e2e-" + suffix + "-restore"
	volume := "restoreguard-e2e-" + suffix + "-data"
	t.Cleanup(func() {
		cleanupCtx, c := context.WithTimeout(context.Background(), 45*time.Second)
		defer c()
		for _, args := range [][]string{{"rm", "-f", "-v", source}, {"rm", "-f", "-v", restore}, {"network", "rm", network}, {"volume", "rm", volume}} {
			_ = exec.CommandContext(cleanupCtx, "docker", args...).Run()
		}
	})
	dockerCommand(t, ctx, "network", "create", "--internal", "--label", "com.restoreguard.managed=true", "--label", "com.restoreguard.purpose=e2e", network)
	dockerCommand(t, ctx, "volume", "create", "--label", "com.restoreguard.managed=true", "--label", "com.restoreguard.purpose=e2e", volume)
	newPostgres(t, ctx, source, network, "", "restoreguard_demo")
	syntheticSQL := `CREATE TABLE customers(id bigint PRIMARY KEY,name text NOT NULL);CREATE TABLE products(id bigint PRIMARY KEY,name text NOT NULL);CREATE TABLE orders(id bigint PRIMARY KEY,customer_id bigint REFERENCES customers(id),product_id bigint REFERENCES products(id));INSERT INTO customers VALUES(1,'Synthetic Alpha'),(2,'Synthetic Beta');INSERT INTO products VALUES(1,'Synthetic Widget'),(2,'Synthetic Service');INSERT INTO orders VALUES(1,1,1),(2,1,2),(3,2,1);`
	dockerCommand(t, ctx, "exec", source, "psql", "-U", "postgres", "-d", "restoreguard_demo", "-v", "ON_ERROR_STOP=1", "-c", syntheticSQL)
	dumpPath := filepath.Join(t.TempDir(), "restoreguard_demo.sql")
	dump, err := os.OpenFile(dumpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	dumpCmd := exec.CommandContext(ctx, "docker", "exec", source, "pg_dump", "-U", "postgres", "--format=plain", "--no-owner", "--no-privileges", "restoreguard_demo")
	dumpCmd.Stdout = dump
	var dumpError strings.Builder
	dumpCmd.Stderr = &dumpError
	if err := dumpCmd.Run(); err != nil {
		dump.Close()
		t.Fatalf("pg_dump: %v %s", err, dumpError.String())
	}
	if err = dump.Close(); err != nil {
		t.Fatal(err)
	}
	dockerCommand(t, ctx, "rm", "-f", "-v", source)
	drillStarted := time.Now().UTC()
	newPostgres(t, ctx, restore, network, volume, "recovery")
	file, err := os.Open(dumpPath)
	if err != nil {
		t.Fatal(err)
	}
	restoreCmd := exec.CommandContext(ctx, "docker", "exec", "-i", restore, "psql", "-U", "postgres", "-d", "recovery", "-v", "ON_ERROR_STOP=1")
	restoreCmd.Stdin = file
	output, err := restoreCmd.CombinedOutput()
	file.Close()
	if err != nil {
		t.Fatalf("restore failed: %v %s", err, output)
	}
	tables := dockerCommand(t, ctx, "exec", restore, "psql", "-U", "postgres", "-d", "recovery", "-Atc", "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('customers','orders','products')")
	if tables != "3" {
		t.Fatalf("expected three tables, got %q", tables)
	}
	rows := dockerCommand(t, ctx, "exec", restore, "psql", "-U", "postgres", "-d", "recovery", "-Atc", "SELECT count(*) FROM orders")
	if rows != "3" {
		t.Fatalf("expected three orders, got %q", rows)
	}
	recoveryReady := time.Now().UTC()
	rto, state := domain.MeasureRTO(drillStarted, recoveryReady)
	if state == domain.PolicyInconclusive || rto < 0 {
		t.Fatal("RTO was not measured")
	}
	rpo, rpoState := domain.MeasureRPO(drillStarted, drillStarted.Add(-5*time.Hour))
	if rpoState == domain.PolicyInconclusive || rpo != 18000 {
		t.Fatal("RPO was not measured")
	}
	dumpBytes, err := os.ReadFile(dumpPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(dumpBytes)
	evidence := map[string]any{"verified": "customers, orders, products and order row count", "method": "typed PostgreSQL validation in isolated Docker network", "backup": "synthetic plain SQL dump", "sandbox": restore, "result": "PASS", "sha256": hex.EncodeToString(sum[:]), "measuredRpoSeconds": rpo, "measuredRtoSeconds": rto}
	encoded, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(t.TempDir(), "evidence.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	dockerCommand(t, ctx, "rm", "-f", "-v", restore)
	dockerCommand(t, ctx, "network", "rm", network)
	dockerCommand(t, ctx, "volume", "rm", volume)
	t.Logf("synthetic database=YES backup=YES isolated restore=YES tables=YES rows=YES evidence=YES RPO=%ds RTO=%ds sandbox destroyed=YES", rpo, rto)
}
func TestCorruptBackupFailsSafely(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	suffix := uuid.NewString()[:8]
	network := "restoreguard-corrupt-" + suffix + "-net"
	container := "restoreguard-corrupt-" + suffix + "-pg"
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", "-v", container).Run()
		_ = exec.Command("docker", "network", "rm", network).Run()
	})
	dockerCommand(t, ctx, "network", "create", "--internal", "--label", "com.restoreguard.managed=true", "--label", "com.restoreguard.purpose=e2e", network)
	newPostgres(t, ctx, container, network, "", "recovery")
	invalid := strings.NewReader("this is not a PostgreSQL dump; DROP nonsense")
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", container, "psql", "-U", "postgres", "-d", "recovery", "-v", "ON_ERROR_STOP=1")
	cmd.Stdin = invalid
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("corrupt backup unexpectedly restored: %s", output)
	}
	assessment, _ := domain.Assess(false, nil, false)
	if assessment != domain.AssessmentFailed {
		t.Fatal("corrupt restore must be FAILED")
	}
	dockerCommand(t, ctx, "rm", "-f", "-v", container)
	dockerCommand(t, ctx, "network", "rm", network)
	t.Log("corrupt backup failed in a controlled way; assessment=FAILED; sandbox destroyed=YES")
}
func TestCancellationPropagatesContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	engine := NewValidationEngine(100)
	result := engine.Run(ctx, ValidationCheck{Name: "cancelled", Type: FileExists, Path: "unused", Timeout: time.Minute, Required: true})
	if result.Status != domain.ValidationInconclusive {
		t.Fatalf("cancelled check must be inconclusive, got %s", result.Status)
	}
}
func Example() {
	fmt.Println("Evidence records what, how, when, backup, sandbox, result")
	// Output: Evidence records what, how, when, backup, sandbox, result
}
