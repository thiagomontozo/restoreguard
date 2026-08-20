# Database

PostgreSQL is the primary metadata store. Timestamps are `timestamptz` and written in UTC. IDs are UUIDs. JSONB stores bounded adapter/check metadata, not large artifacts. Indexes follow tenant/status/date and common asset/source/drill foreign-key filters.

The initial versioned SQL migration creates organizations, authentication/RBAC, assets/sources/snapshots, policies/plans/checks, drills/steps/sandboxes/results, evidence/artifacts/reports, notifications, audit, scheduler jobs, and secret metadata. Published migrations are immutable; schema changes add new numbered files.

Transactions protect bootstrap, password changes, state transitions, finalization, evidence references, and audit-sensitive invariants. Object storage uses compensation because it cannot join a PostgreSQL transaction. Production operators should run migrations as a controlled release step; startup migration is convenient for this experimental single-instance v0.1.
