# Threat model

| Threat | Mitigation in v0.1 | Remaining limitation |
|---|---|---|
| Malicious backup / SQL dump | Treat as untrusted; checksum, byte/time/resource limits; execute only in a disposable DB | A database engine vulnerability could escape; runner isolation is future work |
| Path traversal / archive escape | Generated restrictive storage keys; root-relative checks; no host archive extraction | Additional formats need separate parsers |
| Decompression bomb / oversized artifact | No ZIP extraction; declared and observed size limits; bounded tmpfs/disk/time | PostgreSQL logical expansion can still consume the configured sandbox limit |
| Docker escape / socket compromise | No privileged/host network/arbitrary mounts; allowlisted image; internal network | Docker socket on control plane remains high risk |
| Credential theft | AES-256-GCM, external master key, redacted logs, scoped references | No HSM/Vault adapter and local process can decrypt required secrets |
| Object-storage access | Private credentials, tenant-prefixed generated keys, API authorization, SHA-256 | Bucket policy hardening is deployment responsibility |
| Cross-organization access | Session-derived tenant, tenant predicate on every repository query, integration tests | PostgreSQL RLS is not enabled in v0.1 |
| SSRF | No arbitrary webhook target; HTTP health private/loopback allowlist, no redirects, timeout | DNS rebinding defenses need strengthening before general webhooks |
| Sandbox network pivot | Per-drill Docker `--internal` network; no control/corporate network | Docker host remains shared infrastructure |
| Secret leakage | Structured logging fields, no secret metadata, private reports | Operators must sanitize external tool output |
| Evidence tampering | SHA-256, size/time/drill/source metadata, private authorization | Not append-only or externally notarized; no tamper-proof claim |
| Report disclosure | Cookie auth + `report.export`, private/no-store response, no permanent URL | Downloaded files are controlled by the recipient endpoint |
| Compromised source | Provider boundary, metadata validation, sandbox, no restore to source | A source can serve plausible but malicious content |

Trust boundaries are the browser/API, API/control database, API/object storage, control plane/Docker socket, and sandbox/artifact. Production should place the runner on separate infrastructure, block egress, pin digests, rotate credentials, and monitor Docker/audit events.
