# CI/CD

CI runs on pull requests and main pushes with read-only default permissions. It checks formatting/vet/unit/race where practical, real PostgreSQL/MinIO integration, synthetic Docker restore E2E, frontend lint/typecheck/tests/build, Docker image builds, and OpenAPI presence. Failure artifacts may include sanitized test/coverage output only.

Release workflow runs only for `v*` tags or manual dispatch. It validates, builds backend/frontend images, publishes GHCR with `GITHUB_TOKEN`, and generates SBOM/provenance metadata when supported. It does not create infrastructure or deploy to a server.

Docker cleanup steps use `if: always()`. CI does not use `pull_request_target`, self-hosted runners, production secrets, real backups, or test weakening.
