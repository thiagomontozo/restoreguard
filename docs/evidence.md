# Recovery evidence

Evidence records what was verified, how, when, with which snapshot, in which sandbox, and what result was observed. Artifacts live in object storage; PostgreSQL stores organization/drill references, generated storage key, content type, byte size, SHA-256, status, and timestamps.

All hashes stream through `io.Reader`/`io.Writer` and `io.Copy`; large files are never intentionally loaded in full by the storage adapters. User filenames are display metadata only and never become object paths. Unique drill/type and drill/report constraints make retries idempotent.

Integrity metadata can help detect unintended changes. RestoreGuard does not claim evidence is tamper-proof, forensically certified, court-admissible, or compliance-certified.
