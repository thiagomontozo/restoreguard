package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thiagomontozo/restoreguard/backend/internal/security"
)

type Store struct{ Pool *pgxpool.Pool }
type Principal struct {
	UserID, OrganizationID, Email, DisplayName string
	Roles                                      []string
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{Pool: pool} }

func (s *Store) BootstrapDemo(ctx context.Context, email, password string) error {
	if email == "" || password == "" {
		return nil
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return fmt.Errorf("bootstrap password: %w", err)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var orgID, userID, assetID, sourceID string
	if err = tx.QueryRow(ctx, "INSERT INTO organizations(name,slug) VALUES('RestoreGuard Demo','restoreguard-demo') ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name RETURNING id").Scan(&orgID); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, "INSERT INTO users(organization_id,email,password_hash,display_name) VALUES($1,$2,$3,'Demo Owner') ON CONFLICT(organization_id,email) DO UPDATE SET email=EXCLUDED.email RETURNING id", orgID, email, hash).Scan(&userID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO user_roles(organization_id,user_id,role_id) SELECT $1,$2,id FROM roles WHERE name='OWNER' ON CONFLICT DO NOTHING", orgID, userID); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, "INSERT INTO protected_assets(organization_id,name,type,environment,criticality,owner_name,team,description) VALUES($1,'Demo PostgreSQL ERP','POSTGRESQL_DATABASE','DEMO','CRITICAL','Demo Team','Platform','Synthetic asset used only for safe recovery drills') ON CONFLICT(organization_id,name) DO UPDATE SET updated_at=now() RETURNING id", orgID).Scan(&assetID); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, "INSERT INTO backup_sources(organization_id,name,type,description,environment,configuration) VALUES($1,'Demo PostgreSQL Dumps','POSTGRES_DUMP','Synthetic local dump source','DEMO','{\"storagePrefix\":\"demo/postgres\"}') ON CONFLICT(organization_id,name) DO UPDATE SET updated_at=now() RETURNING id", orgID).Scan(&sourceID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO asset_backup_sources(organization_id,protected_asset_id,backup_source_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", orgID, assetID, sourceID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO recovery_policies(organization_id,name,protected_asset_id,rpo_target_seconds,rto_target_seconds,schedule,required_validations,retention) SELECT $1,'Demo ERP Weekly Recovery',$2,86400,1800,'WEEKLY','[\"POSTGRES_CONNECTIVITY\",\"POSTGRES_TABLE_EXISTS\",\"POSTGRES_ROW_COUNT\"]','{\"evidenceDays\":90}' WHERE NOT EXISTS(SELECT 1 FROM recovery_policies WHERE organization_id=$1 AND name='Demo ERP Weekly Recovery')", orgID, assetID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func randomToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:], nil
}

func (s *Store) Login(ctx context.Context, email, password string, ttl time.Duration) (Principal, string, string, error) {
	var p Principal
	var encoded string
	err := s.Pool.QueryRow(ctx, "SELECT id,organization_id,email,display_name,password_hash FROM users WHERE lower(email)=lower($1) AND active=true", email).Scan(&p.UserID, &p.OrganizationID, &p.Email, &p.DisplayName, &encoded)
	if err != nil || !security.VerifyPassword(encoded, password) {
		return p, "", "", errors.New("invalid credentials")
	}
	p.Roles, err = s.roles(ctx, p.UserID, p.OrganizationID)
	if err != nil {
		return p, "", "", err
	}
	token, tokenHash, err := randomToken()
	if err != nil {
		return p, "", "", err
	}
	csrf, csrfHash, err := randomToken()
	if err != nil {
		return p, "", "", err
	}
	_, err = s.Pool.Exec(ctx, "INSERT INTO sessions(organization_id,user_id,token_hash,csrf_hash,expires_at) VALUES($1,$2,$3,$4,$5)", p.OrganizationID, p.UserID, tokenHash, csrfHash, time.Now().UTC().Add(ttl))
	return p, token, csrf, err
}

func (s *Store) roles(ctx context.Context, userID, orgID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, "SELECT r.name FROM user_roles ur JOIN roles r ON r.id=ur.role_id WHERE ur.user_id=$1 AND ur.organization_id=$2", userID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (s *Store) Authenticate(ctx context.Context, token string) (Principal, []byte, error) {
	var p Principal
	sum := sha256.Sum256([]byte(token))
	var csrfHash []byte
	err := s.Pool.QueryRow(ctx, "SELECT u.id,u.organization_id,u.email,u.display_name,se.csrf_hash FROM sessions se JOIN users u ON u.id=se.user_id AND u.organization_id=se.organization_id WHERE se.token_hash=$1 AND se.revoked_at IS NULL AND se.expires_at>now() AND u.active=true", sum[:]).Scan(&p.UserID, &p.OrganizationID, &p.Email, &p.DisplayName, &csrfHash)
	if err != nil {
		return p, nil, err
	}
	p.Roles, err = s.roles(ctx, p.UserID, p.OrganizationID)
	return p, csrfHash, err
}
func (s *Store) Revoke(ctx context.Context, token string) error {
	sum := sha256.Sum256([]byte(token))
	_, err := s.Pool.Exec(ctx, "UPDATE sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL", sum[:])
	return err
}
func (s *Store) ChangePassword(ctx context.Context, p Principal, current, next string) error {
	var encoded string
	if err := s.Pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1 AND organization_id=$2", p.UserID, p.OrganizationID).Scan(&encoded); err != nil {
		return err
	}
	if !security.VerifyPassword(encoded, current) {
		return errors.New("current password is invalid")
	}
	hash, err := security.HashPassword(next)
	if err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "UPDATE users SET password_hash=$1,password_changed_at=now(),updated_at=now() WHERE id=$2 AND organization_id=$3", hash, p.UserID, p.OrganizationID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE sessions SET revoked_at=now() WHERE user_id=$1 AND organization_id=$2", p.UserID, p.OrganizationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) Audit(ctx context.Context, p Principal, event, resourceType, resourceID, metadata string, ip net.IP) error {
	var id any
	if resourceID != "" {
		if parsed, err := uuid.Parse(resourceID); err == nil {
			id = parsed
		}
	}
	_, err := s.Pool.Exec(ctx, "INSERT INTO audit_events(organization_id,actor_user_id,event_type,resource_type,resource_id,metadata,ip_address) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7)", p.OrganizationID, p.UserID, event, resourceType, id, metadata, ip)
	return err
}
func (s *Store) Health(ctx context.Context) error { return s.Pool.Ping(ctx) }
