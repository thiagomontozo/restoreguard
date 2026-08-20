# PostgreSQL recovery

Plain SQL dumps are restored with `psql -v ON_ERROR_STOP=1` into a newly created PostgreSQL database container. Custom-format support uses `pg_restore` only when the typed plan identifies that format. PostgreSQL is not required on the Windows host; tools run inside the pinned sandbox image.

SQL dumps are hostile input. They execute only inside the drill's disposable PostgreSQL instance, never against the RestoreGuard control database, source database, shared development database, or production. The sandbox uses test-only credentials and is removed whether restore succeeds, fails, times out, or is cancelled.

The E2E fixture creates synthetic `customers`, `products`, and `orders`, takes a dump, removes the source, restores to a separate container, validates three tables and three order rows, records SHA-256 evidence plus RPO/RTO, then destroys container/network/volume. A corrupt input must produce `FAILED` and cleanup.
