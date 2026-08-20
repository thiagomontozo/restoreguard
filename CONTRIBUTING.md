# Contributing to RestoreGuard

RestoreGuard welcomes focused changes that preserve safety, isolation, repeatability, evidence, recoverability, traceability, and honest results.

## Setup

Use Go 1.26+, Docker Desktop/Engine with Compose, and Node 22+ (or the containerized scripts). Copy `.env.example` only for local development; never commit `.env`.

Run `go test -C backend -p 1 ./...` for domain changes, `./scripts/test.ps1` for the integrated suite, and `./scripts/e2e.ps1` for restore-executor changes. Always run `./scripts/cleanup.ps1` after interrupted Docker tests.

## Code style and commits

Use `gofmt`, `go vet`, TypeScript strict mode, ESLint, accessible semantic HTML, and tests around invariants. Keep commits coherent and use conventional subjects such as `feat:`, `fix:`, `test:`, `docs:`, and `ci:`. Never weaken an assertion to make CI green.

## Security expectations

Do not add arbitrary shell execution, user-controlled Docker images/arguments/mounts/network modes, host archive extraction, public report URLs, plaintext secrets, JWTs in localStorage, cross-tenant queries, or logs containing credentials. New restore operations must be typed, allowlisted, bounded, cancellable, auditable, and cleanup-safe. Use only synthetic fixtures.

Published migrations are immutable; add a new versioned migration instead of editing an applied one. Update OpenAPI and documentation when behavior changes.
