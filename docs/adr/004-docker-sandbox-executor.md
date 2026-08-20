# ADR 004: Docker sandbox executor

Status: Accepted with risk (2026-08-20)

Docker supplies reproducible disposable PostgreSQL environments on developer and CI machines. Unique labeled internal networks, bounded resources, read-only root filesystems, allowlisted images, and guaranteed cleanup are required. Socket access is a high-risk v0.1 compromise and motivates a future remote runner.
