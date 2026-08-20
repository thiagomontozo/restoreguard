# ADR 002: PostgreSQL as primary store

Status: Accepted (2026-08-20)

PostgreSQL stores tenant metadata, sessions, policies, durable drill timelines, assessments, audit, and scheduler leases. UUIDs, UTC timestamps, constraints, indexes, JSONB metadata, transactions, and advisory locks fit the invariants. SQLite substitutes are not used for integration tests.
