# API

The versioned API lives under `/api/v1`; liveness is `/health` and readiness is `/ready`. The canonical OpenAPI 3.1 contract is [`contracts/openapi.yaml`](../contracts/openapi.yaml). It documents login/session, protected assets, sources/discovery, snapshots, policies, drills/cancellation/SSE/report, evidence, and audit.

Errors use `{ "error": { "code", "message", "requestId" } }` and never expose stack traces. Collection endpoints accept bounded `page`/`limit`; drills accept status/asset filters and snapshots accept source/status filters. Mutations use cookie authentication, origin validation, and `X-CSRF-Token`.

`POST /drills` accepts `Idempotency-Key`; repeated organization/key requests return the existing drill. `GET /drills/{id}/events` is `text/event-stream` with progress events and heartbeats. Reports are private PDF responses with `no-store`.
