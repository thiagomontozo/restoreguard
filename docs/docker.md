# Docker

`compose.yml` provides development PostgreSQL, MinIO, backend, and frontend on loopback-only published ports. `compose.test.yml` uses tmpfs-backed PostgreSQL/MinIO with RestoreGuard labels. Dockerfiles use multi-stage builds and non-root runtime users where practical.

Every programmatic test/sandbox resource uses `com.restoreguard.managed=true` and a purpose label. Cleanup targets only that label or the RestoreGuard Compose project; global prune commands are forbidden. Health checks cover PostgreSQL, MinIO, backend readiness, and the frontend container.

Sandboxes never use privileged or host network mode. Do not mount `/`, arbitrary host paths, or the socket into the frontend. Development socket access is documented risk; production should move execution behind a runner boundary.
