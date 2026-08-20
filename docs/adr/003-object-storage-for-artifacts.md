# ADR 003: Object storage for artifacts

Status: Accepted (2026-08-20)

Large backup, evidence, validation, and report artifacts belong in local/S3-compatible object storage rather than PostgreSQL. PostgreSQL retains references, size, type, hash, and state. Streaming and compensating cleanup bound memory and handle the lack of a cross-store transaction.
