# Backup providers

`BackupProvider` exposes `Discover`, `GetSnapshot`, `OpenBackup`, and `ValidateMetadata`. Provider-specific configuration and external IDs remain in the adapter layer. Discovery is explicit, asynchronous-ready, and idempotent through `(organization_id, backup_source_id, external_id)` uniqueness.

v0.1 models local filesystem, S3-compatible, and PostgreSQL dump sources. The local adapter accepts only safe leaf identifiers and bounded `.sql`, `.dump`, or `.backup` files. The S3 adapter uses generated object keys and streaming IO. Veeam, Restic, Borg, pgBackRest, and cloud systems are roadmap entries, not fictitious integrations.
