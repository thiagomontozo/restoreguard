# ADR 014: Compensating object-storage cleanup

Status: Accepted (2026-08-20)

PostgreSQL and object storage cannot commit atomically. RestoreGuard writes a generated-key object, records metadata transactionally, and deletes the object if metadata commit fails. Idempotent keys/statuses and periodic reconciliation handle crash windows without pretending distributed atomicity.
