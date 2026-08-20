BEGIN;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE organizations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name varchar(160) NOT NULL,
  slug varchar(80) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  email varchar(254) NOT NULL, password_hash text NOT NULL, display_name varchar(160) NOT NULL,
  mfa_secret_encrypted bytea, mfa_enabled boolean NOT NULL DEFAULT false, active boolean NOT NULL DEFAULT true,
  password_changed_at timestamptz NOT NULL DEFAULT now(), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, email)
);
CREATE TABLE sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, token_hash bytea NOT NULL UNIQUE, csrf_hash bytea NOT NULL,
  expires_at timestamptz NOT NULL, revoked_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), last_seen_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE roles (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name varchar(40) NOT NULL UNIQUE);
CREATE TABLE permissions (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name varchar(80) NOT NULL UNIQUE);
CREATE TABLE role_permissions (role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE, permission_id uuid NOT NULL REFERENCES permissions(id) ON DELETE CASCADE, PRIMARY KEY(role_id, permission_id));
CREATE TABLE user_roles (organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE, PRIMARY KEY(organization_id,user_id,role_id));

CREATE TABLE protected_assets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name varchar(160) NOT NULL, type varchar(40) NOT NULL CHECK(type IN ('POSTGRESQL_DATABASE','FILESYSTEM','APPLICATION_DATASET')),
  environment varchar(80) NOT NULL, criticality varchar(16) NOT NULL CHECK(criticality IN ('LOW','MEDIUM','HIGH','CRITICAL')),
  owner_name varchar(160), owner_email varchar(254), team varchar(120), description text NOT NULL DEFAULT '', enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(organization_id,name)
);
CREATE TABLE secret_metadata (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name varchar(160) NOT NULL, encrypted_value bytea NOT NULL, key_version int NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now(), rotated_at timestamptz
);
CREATE TABLE backup_sources (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name varchar(160) NOT NULL, type varchar(40) NOT NULL CHECK(type IN ('LOCAL_FILESYSTEM','S3_COMPATIBLE','POSTGRES_DUMP')),
  description text NOT NULL DEFAULT '', environment varchar(80) NOT NULL, credential_reference uuid REFERENCES secret_metadata(id) ON DELETE SET NULL,
  configuration jsonb NOT NULL DEFAULT '{}', enabled boolean NOT NULL DEFAULT true, last_discovery_at timestamptz, last_discovery_status varchar(30),
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(organization_id,name)
);
CREATE TABLE asset_backup_sources (organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, protected_asset_id uuid NOT NULL REFERENCES protected_assets(id) ON DELETE CASCADE, backup_source_id uuid NOT NULL REFERENCES backup_sources(id) ON DELETE CASCADE, PRIMARY KEY(protected_asset_id,backup_source_id));
CREATE TABLE backup_snapshots (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  backup_source_id uuid NOT NULL REFERENCES backup_sources(id) ON DELETE CASCADE, external_id varchar(255) NOT NULL, name varchar(255) NOT NULL,
  type varchar(40) NOT NULL, started_at timestamptz, completed_at timestamptz, size_bytes bigint NOT NULL DEFAULT 0 CHECK(size_bytes>=0), checksum varchar(64),
  status varchar(20) NOT NULL CHECK(status IN ('AVAILABLE','INCOMPLETE','CORRUPT','MISSING','UNKNOWN')), metadata jsonb NOT NULL DEFAULT '{}',
  discovered_at timestamptz NOT NULL DEFAULT now(), last_verified_at timestamptz, UNIQUE(organization_id,backup_source_id,external_id)
);
CREATE TABLE recovery_policies (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name varchar(160) NOT NULL, protected_asset_id uuid NOT NULL REFERENCES protected_assets(id) ON DELETE CASCADE,
  rpo_target_seconds bigint NOT NULL CHECK(rpo_target_seconds>0), rto_target_seconds bigint NOT NULL CHECK(rto_target_seconds>0),
  schedule varchar(100) NOT NULL, required_validations jsonb NOT NULL DEFAULT '[]', retention jsonb NOT NULL DEFAULT '{}', enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE restore_plans (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, protected_asset_id uuid NOT NULL REFERENCES protected_assets(id) ON DELETE CASCADE, name varchar(160) NOT NULL, steps jsonb NOT NULL, enabled boolean NOT NULL DEFAULT true, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE validation_checks (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, name varchar(160) NOT NULL, type varchar(40) NOT NULL CHECK(type IN ('FILE_EXISTS','FILE_SIZE','SHA256','POSTGRES_CONNECTIVITY','POSTGRES_QUERY','POSTGRES_TABLE_EXISTS','POSTGRES_ROW_COUNT','HTTP_HEALTH')), configuration jsonb NOT NULL DEFAULT '{}', required boolean NOT NULL DEFAULT true, timeout_seconds int NOT NULL CHECK(timeout_seconds BETWEEN 1 AND 3600), enabled boolean NOT NULL DEFAULT true);
CREATE TABLE recovery_drills (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  protected_asset_id uuid NOT NULL REFERENCES protected_assets(id), backup_snapshot_id uuid REFERENCES backup_snapshots(id), recovery_policy_id uuid REFERENCES recovery_policies(id),
  requested_by uuid NOT NULL REFERENCES users(id), idempotency_key varchar(100), trigger_type varchar(20) NOT NULL CHECK(trigger_type IN ('MANUAL','SCHEDULED','API')),
  status varchar(20) NOT NULL CHECK(status IN ('QUEUED','PREPARING','RESTORING','VALIDATING','FINALIZING','SUCCEEDED','FAILED','CANCELLED','INCONCLUSIVE')),
  started_at timestamptz, completed_at timestamptz, measured_rpo_seconds bigint, measured_rto_seconds bigint,
  rpo_result varchar(20), rto_result varchar(20), recovery_status varchar(30), confidence varchar(10), summary text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(organization_id,idempotency_key)
);
CREATE TABLE drill_steps (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), drill_id uuid NOT NULL REFERENCES recovery_drills(id) ON DELETE CASCADE, type varchar(50) NOT NULL, status varchar(20) NOT NULL, started_at timestamptz, completed_at timestamptz, summary text NOT NULL DEFAULT '', error_code varchar(50), sequence int NOT NULL, UNIQUE(drill_id,sequence));
CREATE TABLE recovery_sandboxes (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, drill_id uuid NOT NULL UNIQUE REFERENCES recovery_drills(id) ON DELETE CASCADE, executor_type varchar(30) NOT NULL, status varchar(30) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), ready_at timestamptz, destroyed_at timestamptz, metadata jsonb NOT NULL DEFAULT '{}');
CREATE TABLE evidence_artifacts (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, drill_id uuid NOT NULL REFERENCES recovery_drills(id) ON DELETE CASCADE, storage_key varchar(500) NOT NULL UNIQUE, content_type varchar(120) NOT NULL, size_bytes bigint NOT NULL, sha256 varchar(64) NOT NULL, status varchar(20) NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE evidence (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, drill_id uuid NOT NULL REFERENCES recovery_drills(id) ON DELETE CASCADE, type varchar(50) NOT NULL, summary text NOT NULL, artifact_id uuid REFERENCES evidence_artifacts(id) ON DELETE SET NULL, sha256 varchar(64), created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(drill_id,type));
CREATE TABLE validation_results (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), drill_id uuid NOT NULL REFERENCES recovery_drills(id) ON DELETE CASCADE, validation_check_id uuid NOT NULL REFERENCES validation_checks(id), status varchar(20) NOT NULL CHECK(status IN ('PASS','FAIL','WARNING','INCONCLUSIVE')), started_at timestamptz NOT NULL, completed_at timestamptz NOT NULL, summary text NOT NULL, metrics jsonb NOT NULL DEFAULT '{}', evidence_id uuid REFERENCES evidence(id), UNIQUE(drill_id,validation_check_id));
CREATE TABLE recovery_reports (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, drill_id uuid NOT NULL UNIQUE REFERENCES recovery_drills(id) ON DELETE CASCADE, artifact_id uuid REFERENCES evidence_artifacts(id), status varchar(20) NOT NULL, generated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE notifications (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, type varchar(30) NOT NULL, event_type varchar(60) NOT NULL, status varchar(20) NOT NULL, configuration jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE audit_events (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, actor_user_id uuid REFERENCES users(id), event_type varchar(80) NOT NULL, resource_type varchar(60), resource_id uuid, metadata jsonb NOT NULL DEFAULT '{}', timestamp timestamptz NOT NULL DEFAULT now(), ip_address inet);
CREATE TABLE scheduler_jobs (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, policy_id uuid NOT NULL UNIQUE REFERENCES recovery_policies(id) ON DELETE CASCADE, next_run_at timestamptz NOT NULL, lease_owner varchar(120), lease_until timestamptz, updated_at timestamptz NOT NULL DEFAULT now());

CREATE INDEX idx_sessions_user ON sessions(organization_id,user_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_assets_org_created ON protected_assets(organization_id,created_at DESC);
CREATE INDEX idx_sources_org_created ON backup_sources(organization_id,created_at DESC);
CREATE INDEX idx_snapshots_source_date ON backup_snapshots(organization_id,backup_source_id,completed_at DESC);
CREATE INDEX idx_policies_asset ON recovery_policies(organization_id,protected_asset_id);
CREATE INDEX idx_drills_org_status_created ON recovery_drills(organization_id,status,created_at DESC);
CREATE INDEX idx_drills_asset_created ON recovery_drills(protected_asset_id,created_at DESC);
CREATE INDEX idx_steps_drill ON drill_steps(drill_id,sequence);
CREATE INDEX idx_evidence_drill ON evidence(organization_id,drill_id,created_at);
CREATE INDEX idx_audit_org_time ON audit_events(organization_id,timestamp DESC);

INSERT INTO roles(name) VALUES ('OWNER'),('ADMIN'),('RECOVERY_ENGINEER'),('OPERATOR'),('AUDITOR'),('VIEWER') ON CONFLICT DO NOTHING;
INSERT INTO permissions(name) VALUES ('backup_source.read'),('backup_source.manage'),('recovery_policy.read'),('recovery_policy.manage'),('drill.read'),('drill.run'),('evidence.read'),('report.export'),('user.manage'),('audit.read'),('settings.manage') ON CONFLICT DO NOTHING;
INSERT INTO role_permissions(role_id,permission_id) SELECT r.id,p.id FROM roles r CROSS JOIN permissions p WHERE r.name IN ('OWNER','ADMIN') ON CONFLICT DO NOTHING;
INSERT INTO role_permissions(role_id,permission_id) SELECT r.id,p.id FROM roles r JOIN permissions p ON p.name IN ('backup_source.read','backup_source.manage','recovery_policy.read','recovery_policy.manage','drill.read','drill.run','evidence.read','report.export') WHERE r.name='RECOVERY_ENGINEER' ON CONFLICT DO NOTHING;
INSERT INTO role_permissions(role_id,permission_id) SELECT r.id,p.id FROM roles r JOIN permissions p ON p.name IN ('backup_source.read','recovery_policy.read','drill.read','drill.run','evidence.read') WHERE r.name='OPERATOR' ON CONFLICT DO NOTHING;
INSERT INTO role_permissions(role_id,permission_id) SELECT r.id,p.id FROM roles r JOIN permissions p ON p.name IN ('backup_source.read','recovery_policy.read','drill.read','evidence.read','report.export','audit.read') WHERE r.name='AUDITOR' ON CONFLICT DO NOTHING;
INSERT INTO role_permissions(role_id,permission_id) SELECT r.id,p.id FROM roles r JOIN permissions p ON p.name IN ('backup_source.read','recovery_policy.read','drill.read','evidence.read') WHERE r.name='VIEWER' ON CONFLICT DO NOTHING;
INSERT INTO schema_migrations(version) VALUES ('001_initial') ON CONFLICT DO NOTHING;
COMMIT;
