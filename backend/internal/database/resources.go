package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/thiagomontozo/restoreguard/backend/internal/domain"
)

type Asset struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Environment string    `json:"environment"`
	Criticality string    `json:"criticality"`
	OwnerName   string    `json:"ownerName"`
	Team        string    `json:"team"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
}
type Source struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Type                string    `json:"type"`
	Environment         string    `json:"environment"`
	Description         string    `json:"description"`
	Enabled             bool      `json:"enabled"`
	LastDiscoveryStatus *string   `json:"lastDiscoveryStatus"`
	CreatedAt           time.Time `json:"createdAt"`
}
type Policy struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ProtectedAssetID string `json:"protectedAssetId"`
	RPOTargetSeconds int64  `json:"rpoTargetSeconds"`
	RTOTargetSeconds int64  `json:"rtoTargetSeconds"`
	Schedule         string `json:"schedule"`
	Enabled          bool   `json:"enabled"`
}

func (s *Store) Overview(ctx context.Context, org string) (map[string]int, error) {
	row := s.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM protected_assets WHERE organization_id=$1 AND enabled),(SELECT count(DISTINCT protected_asset_id) FROM recovery_drills WHERE organization_id=$1 AND recovery_status='VERIFIED'),(SELECT count(*) FROM protected_assets a WHERE organization_id=$1 AND NOT EXISTS(SELECT 1 FROM recovery_drills d WHERE d.protected_asset_id=a.id)),(SELECT count(*) FROM recovery_drills WHERE organization_id=$1 AND rpo_result='FAIL'),(SELECT count(*) FROM recovery_drills WHERE organization_id=$1 AND rto_result='FAIL'),(SELECT count(*) FROM recovery_drills WHERE organization_id=$1 AND status='FAILED'),(SELECT count(*) FROM recovery_drills WHERE organization_id=$1 AND status='INCONCLUSIVE'),(SELECT count(*) FROM recovery_drills WHERE organization_id=$1 AND status IN ('QUEUED','PREPARING','RESTORING','VALIDATING','FINALIZING'))`, org)
	keys := []string{"protectedAssets", "verifiedAssets", "neverTested", "rpoFailures", "rtoFailures", "failedDrills", "inconclusiveDrills", "runningDrills"}
	values := make([]int, 8)
	if err := row.Scan(&values[0], &values[1], &values[2], &values[3], &values[4], &values[5], &values[6], &values[7]); err != nil {
		return nil, err
	}
	result := map[string]int{}
	for i, key := range keys {
		result[key] = values[i]
	}
	return result, nil
}
func pageValues(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}
func (s *Store) ListAssets(ctx context.Context, org, search string, page, limit int) ([]Asset, error) {
	page, limit = pageValues(page, limit)
	rows, err := s.Pool.Query(ctx, "SELECT id,name,type,environment,criticality,coalesce(owner_name,''),coalesce(team,''),description,enabled,created_at FROM protected_assets WHERE organization_id=$1 AND ($2='' OR name ILIKE '%'||$2||'%') ORDER BY created_at DESC LIMIT $3 OFFSET $4", org, search, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Asset{}
	for rows.Next() {
		var item Asset
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.Environment, &item.Criticality, &item.OwnerName, &item.Team, &item.Description, &item.Enabled, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) CreateAsset(ctx context.Context, org string, item Asset) (Asset, error) {
	err := s.Pool.QueryRow(ctx, "INSERT INTO protected_assets(organization_id,name,type,environment,criticality,owner_name,team,description,enabled) VALUES($1,$2,$3,$4,$5,$6,$7,$8,true) RETURNING id,created_at", org, item.Name, item.Type, item.Environment, item.Criticality, item.OwnerName, item.Team, item.Description).Scan(&item.ID, &item.CreatedAt)
	item.Enabled = true
	return item, err
}
func (s *Store) GetAsset(ctx context.Context, org, id string) (Asset, error) {
	var item Asset
	err := s.Pool.QueryRow(ctx, "SELECT id,name,type,environment,criticality,coalesce(owner_name,''),coalesce(team,''),description,enabled,created_at FROM protected_assets WHERE organization_id=$1 AND id=$2", org, id).Scan(&item.ID, &item.Name, &item.Type, &item.Environment, &item.Criticality, &item.OwnerName, &item.Team, &item.Description, &item.Enabled, &item.CreatedAt)
	return item, err
}
func (s *Store) ListSources(ctx context.Context, org string, page, limit int) ([]Source, error) {
	page, limit = pageValues(page, limit)
	rows, err := s.Pool.Query(ctx, "SELECT id,name,type,environment,description,enabled,last_discovery_status,created_at FROM backup_sources WHERE organization_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3", org, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Source{}
	for rows.Next() {
		var item Source
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.Environment, &item.Description, &item.Enabled, &item.LastDiscoveryStatus, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) CreateSource(ctx context.Context, org string, item Source) (Source, error) {
	err := s.Pool.QueryRow(ctx, "INSERT INTO backup_sources(organization_id,name,type,environment,description,configuration) VALUES($1,$2,$3,$4,$5,'{}') RETURNING id,created_at", org, item.Name, item.Type, item.Environment, item.Description).Scan(&item.ID, &item.CreatedAt)
	item.Enabled = true
	return item, err
}
func (s *Store) DiscoverSource(ctx context.Context, org, id string) error {
	tag, err := s.Pool.Exec(ctx, "UPDATE backup_sources SET last_discovery_at=now(),last_discovery_status='SUCCEEDED',updated_at=now() WHERE organization_id=$1 AND id=$2", org, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}
func (s *Store) ListPolicies(ctx context.Context, org string, page, limit int) ([]Policy, error) {
	page, limit = pageValues(page, limit)
	rows, err := s.Pool.Query(ctx, "SELECT id,name,protected_asset_id,rpo_target_seconds,rto_target_seconds,schedule,enabled FROM recovery_policies WHERE organization_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3", org, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Policy{}
	for rows.Next() {
		var item Policy
		if err := rows.Scan(&item.ID, &item.Name, &item.ProtectedAssetID, &item.RPOTargetSeconds, &item.RTOTargetSeconds, &item.Schedule, &item.Enabled); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) CreatePolicy(ctx context.Context, org string, item Policy) (Policy, error) {
	if item.RPOTargetSeconds <= 0 || item.RTOTargetSeconds <= 0 || item.RPOTargetSeconds > 31536000 || item.RTOTargetSeconds > 604800 {
		return item, errors.New("invalid RPO/RTO target")
	}
	err := s.Pool.QueryRow(ctx, "INSERT INTO recovery_policies(organization_id,name,protected_asset_id,rpo_target_seconds,rto_target_seconds,schedule,required_validations,retention) SELECT $1,$2,id,$4,$5,$6,'[]','{}' FROM protected_assets WHERE organization_id=$1 AND id=$3 RETURNING id", org, item.Name, item.ProtectedAssetID, item.RPOTargetSeconds, item.RTOTargetSeconds, item.Schedule).Scan(&item.ID)
	item.Enabled = true
	return item, err
}
func (s *Store) ListDrills(ctx context.Context, org, status, asset string, page, limit int) ([]domain.RecoveryDrill, error) {
	page, limit = pageValues(page, limit)
	rows, err := s.Pool.Query(ctx, `SELECT id,organization_id,protected_asset_id,coalesce(backup_snapshot_id::text,''),coalesce(recovery_policy_id::text,''),requested_by,trigger_type,status,started_at,completed_at,measured_rpo_seconds,measured_rto_seconds,coalesce(rpo_result,''),coalesce(rto_result,''),coalesce(recovery_status,''),coalesce(confidence,''),summary,created_at,updated_at FROM recovery_drills WHERE organization_id=$1 AND ($2='' OR status=$2) AND ($3='' OR protected_asset_id::text=$3) ORDER BY created_at DESC LIMIT $4 OFFSET $5`, org, status, asset, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.RecoveryDrill{}
	for rows.Next() {
		var item domain.RecoveryDrill
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.ProtectedAssetID, &item.BackupSnapshotID, &item.RecoveryPolicyID, &item.RequestedBy, &item.TriggerType, &item.Status, &item.StartedAt, &item.CompletedAt, &item.MeasuredRPOSeconds, &item.MeasuredRTOSeconds, &item.RPOResult, &item.RTOResult, &item.RecoveryStatus, &item.Confidence, &item.Summary, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) CreateDrill(ctx context.Context, p Principal, assetID, snapshotID, policyID, idempotency string) (domain.RecoveryDrill, bool, error) {
	var item domain.RecoveryDrill
	if idempotency == "" {
		idempotency = uuid.NewString()
	}
	err := s.Pool.QueryRow(ctx, `INSERT INTO recovery_drills(organization_id,protected_asset_id,backup_snapshot_id,recovery_policy_id,requested_by,idempotency_key,trigger_type,status) SELECT $1,a.id,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,$5,$6,'MANUAL','QUEUED' FROM protected_assets a WHERE a.organization_id=$1 AND a.id=$2 ON CONFLICT(organization_id,idempotency_key) DO NOTHING RETURNING id,organization_id,protected_asset_id,coalesce(backup_snapshot_id::text,''),coalesce(recovery_policy_id::text,''),requested_by,trigger_type,status,summary,created_at,updated_at`, p.OrganizationID, assetID, snapshotID, policyID, p.UserID, idempotency).Scan(&item.ID, &item.OrganizationID, &item.ProtectedAssetID, &item.BackupSnapshotID, &item.RecoveryPolicyID, &item.RequestedBy, &item.TriggerType, &item.Status, &item.Summary, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.Pool.QueryRow(ctx, `SELECT id,organization_id,protected_asset_id,coalesce(backup_snapshot_id::text,''),coalesce(recovery_policy_id::text,''),requested_by,trigger_type,status,summary,created_at,updated_at FROM recovery_drills WHERE organization_id=$1 AND idempotency_key=$2`, p.OrganizationID, idempotency).Scan(&item.ID, &item.OrganizationID, &item.ProtectedAssetID, &item.BackupSnapshotID, &item.RecoveryPolicyID, &item.RequestedBy, &item.TriggerType, &item.Status, &item.Summary, &item.CreatedAt, &item.UpdatedAt)
		return item, false, err
	}
	return item, true, err
}
func (s *Store) GetDrill(ctx context.Context, org, id string) (domain.RecoveryDrill, error) {
	items, err := s.ListDrills(ctx, org, "", "", 1, 100)
	if err != nil {
		return domain.RecoveryDrill{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return domain.RecoveryDrill{}, pgx.ErrNoRows
}
func (s *Store) CancelDrill(ctx context.Context, org, id string) error {
	tag, err := s.Pool.Exec(ctx, "UPDATE recovery_drills SET status='CANCELLED',completed_at=now(),updated_at=now(),summary='Cancelled by an authorized user' WHERE organization_id=$1 AND id=$2 AND status IN ('QUEUED','PREPARING','RESTORING','VALIDATING','FINALIZING')", org, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}
