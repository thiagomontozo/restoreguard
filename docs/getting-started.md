# Getting started

Install Docker Desktop/Engine with Linux containers and Compose. Clone the repository, copy `.env.example` to `.env` for reference, then run `docker compose up -d --build`. Compose uses ports 55173 (UI), 58080 (API), 55432 (PostgreSQL), and 59000/59001 (MinIO API/console).

The development seed creates the fictitious organization `RestoreGuard Demo`, asset `Demo PostgreSQL ERP`, and a weekly RPO 24h/RTO 30m policy. The Compose bootstrap password is development-only. Change it after login. Production bootstrap must be supplied through a secret mechanism, then removed.

Check `http://localhost:58080/health` and `/ready`, sign in at `http://localhost:55173`, and run `./scripts/cleanup.ps1` when finished. Cleanup removes only resources carrying RestoreGuard project names or labels.
