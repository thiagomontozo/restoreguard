# Deployment

v0.1 is experimental and best suited to isolated evaluation. Supply PostgreSQL, private object storage, HTTPS ingress, external session/master/provider secrets, pinned image digests, restrictive egress, backups for RestoreGuard metadata, and centralized logs/audit.

Run one scheduler-active instance or preserve PostgreSQL advisory-lock semantics. Do not expose the Docker socket to an Internet-facing process in a production trust zone; use a dedicated runner host when that architecture becomes available. Frontend and API should share an origin where possible.

Readiness requires database, migrations, and object storage. Shutdown stops HTTP acceptance, scheduler and workers, cancels contexts, waits within a deadline, closes SSE, and performs safe local sandbox cleanup. There is no automated cloud deployment in this repository.
