# Audit

`AuditEvent` records organization, actor, event type, resource type/ID, sanitized JSON metadata, timestamp, and optional IP. Authentication, source/policy changes, drill lifecycle, sandbox/restore/validation lifecycle, evidence, credentials, and report generation are important event families.

Audit records never contain passwords, session/CSRF tokens, master keys, provider credentials, raw dumps, full sensitive queries, or stack traces. Request IDs may link audit and structured logs without becoming authentication data.

v0.1 audit storage is ordinary PostgreSQL data, not an immutable ledger. Operators should export it to a protected external system if their threat model includes database administrators.
