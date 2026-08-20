# Security model

RestoreGuard assumes authenticated users, backup artifacts, backup sources, and network targets may be malicious or compromised. Controls are layered: organization-scoped authorization, typed operations, bounded inputs, isolated restore destinations, cryptographic integrity metadata, least disclosure, auditable events, and cleanup.

Passwords use Argon2id (64 MiB, three iterations, two lanes, 32-byte output, random 16-byte salt). Sessions use random 256-bit tokens stored only as SHA-256 hashes in PostgreSQL, HttpOnly SameSite=Strict cookies, expiry, logout revocation, and revocation on password change. Secure cookies are mandatory under HTTPS. Mutations require a separate random CSRF token plus origin validation. CORS allows one configured origin with credentials, never `*`.

Security headers include content-type sniffing prevention, frame denial, no-referrer, and CSP. Secrets use AES-256-GCM with organization ID as authenticated data and an external base64 master key. Secrets never belong in Git, URLs, logs, audit metadata, or evidence. See authentication, authorization, secrets, threat model, and sandbox documents.
