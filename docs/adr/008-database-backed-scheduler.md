# ADR 008: Database-backed scheduler

Status: Accepted (2026-08-20)

PostgreSQL job rows plus an advisory lock prevent duplicate scheduling without a message broker. Leases are inspectable and transactional. This is conservative for v0.1; a distributed worker system may replace it only with equivalent idempotency and audit semantics.
