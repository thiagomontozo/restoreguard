# ADR 011: Backup provider abstraction

Status: Accepted (2026-08-20)

Discover/GetSnapshot/OpenBackup/ValidateMetadata isolate provider details from the recovery domain. v0.1 implements honest local/S3/PostgreSQL-dump capabilities and does not fake proprietary integrations. Source/external-ID uniqueness makes discovery idempotent.
