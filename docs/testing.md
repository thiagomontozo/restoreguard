# Testing

Unit tests cover state transitions, RPO/RTO/policy evaluation, confidence semantics, password hashing, secret organization binding, RBAC, path safety, streaming storage, validation failure/cancellation, Docker argument safety, reporting, and scheduler foundations.

Integration tests use real PostgreSQL for migrations, entities, organization isolation, transactions, and drill idempotency. MinIO tests put/get/stream/delete/checksum/missing-object behavior; local storage adds traversal and oversize cases. Docker executor tests inspect labels, internal network, memory/read-only settings, readiness, destroy, and cancellation.

The mandatory E2E creates only synthetic data, generates a plain dump, removes the source, restores into a different PostgreSQL container, validates tables/rows, produces SHA-256 evidence, measures objectives, and destroys resources. Separate tests prove corrupt restore failure, required validation failure, fake-clock RPO/RTO failure, cancellation, and idempotency. `scripts/test.ps1` and `scripts/e2e.ps1` always clean up.
