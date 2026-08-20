package drill

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thiagomontozo/restoreguard/backend/internal/domain"
	"github.com/thiagomontozo/restoreguard/backend/internal/platform"
	"github.com/thiagomontozo/restoreguard/backend/internal/storage"
)

type WorkerDependencies struct {
	Executor         *DockerSandboxExecutor
	Objects          storage.ObjectStorage
	AllowedImage     string
	MaxArtifactBytes int64
	DrillTimeout     time.Duration
}
type WorkerPool struct {
	db      *pgxpool.Pool
	clock   platform.Clock
	hub     *EventHub
	jobs    chan string
	root    context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	running map[string]context.CancelFunc
	deps    WorkerDependencies
}

func NewWorkerPool(db *pgxpool.Pool, clock platform.Clock, hub *EventHub, concurrency int, dependencies ...WorkerDependencies) *WorkerPool {
	root, cancel := context.WithCancel(context.Background())
	pool := &WorkerPool{db: db, clock: clock, hub: hub, jobs: make(chan string, 100), root: root, cancel: cancel, running: map[string]context.CancelFunc{}}
	if len(dependencies) > 0 {
		pool.deps = dependencies[0]
	}
	for range concurrency {
		pool.wg.Add(1)
		go pool.worker()
	}
	return pool
}
func (p *WorkerPool) Submit(id string) error {
	select {
	case p.jobs <- id:
		return nil
	default:
		return errors.New("drill queue is full")
	}
}
func (p *WorkerPool) Cancel(id string) {
	p.mu.Lock()
	cancel := p.running[id]
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
func (p *WorkerPool) Close(ctx context.Context) error {
	p.cancel()
	done := make(chan struct{})
	go func() { p.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (p *WorkerPool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.root.Done():
			return
		case id := <-p.jobs:
			p.run(id)
		}
	}
}

func (p *WorkerPool) run(id string) {
	baseCtx, cancel := context.WithCancel(p.root)
	if p.deps.DrillTimeout > 0 {
		baseCtx, cancel = context.WithTimeout(p.root, p.deps.DrillTimeout)
	}
	p.mu.Lock()
	p.running[id] = cancel
	p.mu.Unlock()
	defer func() { cancel(); p.mu.Lock(); delete(p.running, id); p.mu.Unlock() }()
	if p.deps.Executor == nil || p.deps.Objects == nil {
		_ = p.transition(baseCtx, id, domain.DrillPreparing, "PREPARE_DRILL", 1)
		_ = p.setTerminal(context.Background(), id, domain.DrillInconclusive, "No sandbox/object-storage executor is configured; recovery was not attempted")
		return
	}
	if err := p.transition(baseCtx, id, domain.DrillPreparing, "FETCH_BACKUP", 1); err != nil {
		return
	}
	var orgID, storageKey string
	var drillStarted, snapshotAt time.Time
	var checksum *string
	var rpoTarget, rtoTarget int64
	var requiredJSON []byte
	err := p.db.QueryRow(baseCtx, `SELECT d.organization_id,d.started_at,s.completed_at,s.checksum,coalesce(s.metadata->>'storageKey',''),p.rpo_target_seconds,p.rto_target_seconds,p.required_validations FROM recovery_drills d JOIN backup_snapshots s ON s.id=d.backup_snapshot_id AND s.organization_id=d.organization_id JOIN recovery_policies p ON p.id=d.recovery_policy_id AND p.organization_id=d.organization_id WHERE d.id=$1`, id).Scan(&orgID, &drillStarted, &snapshotAt, &checksum, &storageKey, &rpoTarget, &rtoTarget, &requiredJSON)
	if err != nil || storageKey == "" {
		_ = p.setTerminal(baseCtx, id, domain.DrillInconclusive, "A selected snapshot with a trusted storage key and policy is required")
		return
	}
	temp, err := os.CreateTemp("", "restoreguard-backup-*.sql")
	if err != nil {
		p.fail(baseCtx, id, "Backup staging failed")
		return
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	reader, info, err := p.deps.Objects.Get(baseCtx, storageKey)
	if err != nil {
		temp.Close()
		p.fail(baseCtx, id, "Backup artifact could not be opened")
		return
	}
	hash := sha256.New()
	limit := p.deps.MaxArtifactBytes
	if limit <= 0 {
		limit = 1 << 30
	}
	written, copyErr := io.Copy(io.MultiWriter(temp, hash), io.LimitReader(reader, limit+1))
	reader.Close()
	closeErr := temp.Close()
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if copyErr != nil || closeErr != nil || written > limit || info.Size != written || (checksum != nil && *checksum != "" && !equalHash(*checksum, actualHash)) {
		p.fail(baseCtx, id, "Backup artifact integrity validation failed")
		return
	}
	sandbox, err := p.deps.Executor.Create(baseCtx, SandboxSpec{DrillID: compactID(id), OrganizationID: orgID, Image: p.deps.AllowedImage, CPUs: 1, MemoryBytes: 512 * 1024 * 1024, Timeout: 2 * time.Minute})
	if err != nil {
		p.fail(baseCtx, id, "Sandbox creation failed")
		return
	}
	destroyed := false
	defer func() {
		if !destroyed {
			cleanupCtx, c := context.WithTimeout(context.Background(), 45*time.Second)
			defer c()
			_ = p.deps.Executor.Destroy(cleanupCtx, sandbox)
			_, _ = p.db.Exec(cleanupCtx, "UPDATE recovery_sandboxes SET status='DESTROYED',destroyed_at=now() WHERE drill_id=$1", id)
		}
	}()
	_, err = p.db.Exec(baseCtx, `INSERT INTO recovery_sandboxes(organization_id,drill_id,executor_type,status,created_at,ready_at,metadata) VALUES($1,$2,'DOCKER','READY',$3,$4,jsonb_build_object('containerName',$5,'networkName',$6)) ON CONFLICT(drill_id) DO UPDATE SET status='READY',ready_at=EXCLUDED.ready_at,metadata=EXCLUDED.metadata`, orgID, id, sandbox.CreatedAt, sandbox.ReadyAt, sandbox.ContainerName, sandbox.NetworkName)
	if err != nil {
		p.fail(baseCtx, id, "Sandbox metadata could not be persisted")
		return
	}
	if err = p.recordStep(baseCtx, id, "CREATE_SANDBOX", 2, "Isolated Docker sandbox became ready"); err != nil {
		return
	}
	if err = p.transition(baseCtx, id, domain.DrillRestoring, "RESTORE_POSTGRES", 3); err != nil {
		return
	}
	if err = p.deps.Executor.CopyPostgresBackup(baseCtx, sandbox, tempPath); err == nil {
		err = p.deps.Executor.RestorePostgresPlain(baseCtx, sandbox)
	}
	if err != nil {
		p.fail(baseCtx, id, "PostgreSQL restore failed")
		return
	}
	if err = p.transition(baseCtx, id, domain.DrillValidating, "POSTGRES_CONNECTIVITY", 4); err != nil {
		return
	}
	if err = p.deps.Executor.ValidatePostgresConnectivity(baseCtx, sandbox); err != nil {
		p.fail(baseCtx, id, "Required PostgreSQL connectivity validation failed")
		return
	}
	recoveryReady := p.clock.Now()
	if err = p.transition(baseCtx, id, domain.DrillFinalizing, "COLLECT_EVIDENCE", 5); err != nil {
		return
	}
	var required []string
	_ = json.Unmarshal(requiredJSON, &required)
	limitations := []string{}
	for _, check := range required {
		if check != "POSTGRES_CONNECTIVITY" {
			limitations = append(limitations, check+" was not configured with a typed validation profile")
		}
	}
	rpo, state := domain.MeasureRPO(drillStarted, snapshotAt)
	rto, rtoState := domain.MeasureRTO(drillStarted, recoveryReady)
	rpoResult := domain.EvaluatePolicy(rpo, state, rpoTarget)
	rtoResult := domain.EvaluatePolicy(rto, rtoState, rtoTarget)
	assessment, confidence := domain.Verified, domain.ConfidenceHigh
	if len(limitations) > 0 {
		assessment, confidence = domain.PartiallyVerified, domain.ConfidenceMedium
	}
	evidencePayload, _ := json.Marshal(map[string]any{"what": "PostgreSQL plain dump restore and typed connectivity validation", "how": "allowlisted Docker sandbox on an internal network", "when": recoveryReady.UTC(), "backupSnapshotTime": snapshotAt.UTC(), "sandboxId": sandbox.ID, "result": "PASS", "limitations": limitations, "measuredRpoSeconds": rpo, "measuredRtoSeconds": rto, "backupSha256": actualHash})
	evidenceKey := fmt.Sprintf("%s/drills/%s/evidence/recovery.json", orgID, id)
	artifact, err := p.deps.Objects.Put(baseCtx, evidenceKey, bytes.NewReader(evidencePayload), int64(len(evidencePayload)), "application/json")
	if err != nil {
		p.fail(baseCtx, id, "Recovery evidence could not be stored")
		return
	}
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 45*time.Second)
	err = p.deps.Executor.Destroy(cleanupCtx, sandbox)
	cleanupCancel()
	if err != nil {
		_ = p.deps.Objects.Delete(context.Background(), evidenceKey)
		p.fail(baseCtx, id, "Sandbox cleanup failed")
		return
	}
	destroyed = true
	_, _ = p.db.Exec(baseCtx, "UPDATE recovery_sandboxes SET status='DESTROYED',destroyed_at=now() WHERE drill_id=$1", id)
	tx, err := p.db.Begin(baseCtx)
	if err != nil {
		_ = p.deps.Objects.Delete(context.Background(), evidenceKey)
		return
	}
	defer tx.Rollback(baseCtx)
	var artifactID string
	err = tx.QueryRow(baseCtx, `INSERT INTO evidence_artifacts(organization_id,drill_id,storage_key,content_type,size_bytes,sha256,status) VALUES($1,$2,$3,$4,$5,$6,'AVAILABLE') ON CONFLICT(storage_key) DO UPDATE SET sha256=EXCLUDED.sha256 RETURNING id`, orgID, id, artifact.Key, artifact.ContentType, artifact.Size, artifact.SHA256).Scan(&artifactID)
	if err != nil {
		_ = p.deps.Objects.Delete(context.Background(), evidenceKey)
		return
	}
	_, err = tx.Exec(baseCtx, `INSERT INTO evidence(organization_id,drill_id,type,summary,artifact_id,sha256) VALUES($1,$2,'POSTGRES_RECOVERY','Plain dump restored in an isolated Docker sandbox; typed connectivity validation passed',$3,$4) ON CONFLICT(drill_id,type) DO UPDATE SET artifact_id=EXCLUDED.artifact_id,sha256=EXCLUDED.sha256`, orgID, id, artifactID, artifact.SHA256)
	if err != nil {
		_ = p.deps.Objects.Delete(context.Background(), evidenceKey)
		return
	}
	now := p.clock.Now()
	tag, err := tx.Exec(baseCtx, `UPDATE recovery_drills SET status='SUCCEEDED',completed_at=$2,measured_rpo_seconds=$3,measured_rto_seconds=$4,rpo_result=$5,rto_result=$6,recovery_status=$7,confidence=$8,summary='Controlled PostgreSQL recovery drill completed under tested conditions',updated_at=$2 WHERE id=$1 AND status='FINALIZING'`, id, now, rpo, rto, rpoResult, rtoResult, assessment, confidence)
	if err != nil || tag.RowsAffected() != 1 {
		_ = p.deps.Objects.Delete(context.Background(), evidenceKey)
		return
	}
	_, err = tx.Exec(baseCtx, `INSERT INTO recovery_reports(organization_id,drill_id,status,generated_at) VALUES($1,$2,'AVAILABLE',$3) ON CONFLICT(drill_id) DO UPDATE SET status=EXCLUDED.status`, orgID, id, now)
	if err != nil {
		_ = p.deps.Objects.Delete(context.Background(), evidenceKey)
		return
	}
	if err = tx.Commit(baseCtx); err == nil {
		p.hub.Publish(Event{DrillID: id, Type: "drill.completed", Data: map[string]any{"status": "SUCCEEDED", "assessment": assessment}})
	} else {
		_ = p.deps.Objects.Delete(context.Background(), evidenceKey)
	}
}

func compactID(id string) string {
	result := make([]byte, 0, len(id))
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result = append(result, c)
		}
	}
	if len(result) > 18 {
		result = result[:18]
	}
	return string(result)
}
func equalHash(a, b string) bool { return len(a) == len(b) && bytes.EqualFold([]byte(a), []byte(b)) }
func (p *WorkerPool) fail(ctx context.Context, id, summary string) {
	status := domain.DrillFailed
	if ctx.Err() != nil {
		status = domain.DrillCancelled
		summary = "Drill was cancelled; partial evidence may be retained"
	}
	_ = p.setTerminal(context.Background(), id, status, summary)
}
func (p *WorkerPool) recordStep(ctx context.Context, id, step string, sequence int, summary string) error {
	now := p.clock.Now()
	_, err := p.db.Exec(ctx, "INSERT INTO drill_steps(drill_id,type,status,started_at,completed_at,summary,sequence) VALUES($1,$2,'PASSED',$3,$3,$4,$5) ON CONFLICT(drill_id,sequence) DO NOTHING", id, step, now, summary, sequence)
	if err == nil {
		p.hub.Publish(Event{DrillID: id, Type: "drill.progress", Data: map[string]any{"step": step}})
	}
	return err
}
func (p *WorkerPool) transition(ctx context.Context, id string, to domain.DrillStatus, step string, sequence int) error {
	tx, err := p.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var from domain.DrillStatus
	if err = tx.QueryRow(ctx, "SELECT status FROM recovery_drills WHERE id=$1 FOR UPDATE", id).Scan(&from); err != nil {
		return err
	}
	if err = domain.Transition(from, to); err != nil {
		return err
	}
	now := p.clock.Now()
	var started *time.Time
	if to == domain.DrillPreparing {
		started = &now
	}
	_, err = tx.Exec(ctx, "UPDATE recovery_drills SET status=$2,started_at=coalesce(started_at,$3),updated_at=$4 WHERE id=$1", id, to, started, now)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO drill_steps(drill_id,type,status,started_at,completed_at,summary,sequence) VALUES($1,$2,'PASSED',$3,$3,$4,$5) ON CONFLICT(drill_id,sequence) DO NOTHING", id, step, now, fmt.Sprintf("%s completed", step), sequence)
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	p.hub.Publish(Event{DrillID: id, Type: "drill.progress", Data: map[string]any{"status": to, "step": step}})
	return nil
}
func (p *WorkerPool) setTerminal(ctx context.Context, id string, status domain.DrillStatus, summary string) error {
	_, err := p.db.Exec(ctx, "UPDATE recovery_drills SET status=$2,completed_at=now(),updated_at=now(),recovery_status=CASE WHEN $2='FAILED' THEN 'FAILED' ELSE 'INCONCLUSIVE' END,confidence='LOW',summary=$3 WHERE id=$1 AND status NOT IN ('SUCCEEDED','FAILED','CANCELLED','INCONCLUSIVE')", id, status, summary)
	if err == nil {
		p.hub.Publish(Event{DrillID: id, Type: "drill.completed", Data: map[string]any{"status": status}})
	}
	return err
}
