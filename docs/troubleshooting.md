# Troubleshooting

If `/ready` fails, inspect sanitized backend logs and check PostgreSQL/MinIO health with `docker compose ps`. Confirm the configured database URL, bucket, migration path, and that development ports are free. Do not paste secrets into issues.

If a sandbox cannot start, check Docker Desktop Linux mode, allowlisted image availability, socket access, memory limits, and existing labeled resources. Run `docker ps -a --filter label=com.restoreguard.managed=true`, then `./scripts/cleanup.ps1`; never use global system/volume prune.

On Windows, antivirus can briefly lock a completed Go test executable and make cleanup report `unlinkat ... file in use` even after tests print `PASS`. Re-run serially with `-p 1`. If npm/Docker DNS is unavailable, fix Docker Desktop proxy/DNS rather than changing tests to skip.
