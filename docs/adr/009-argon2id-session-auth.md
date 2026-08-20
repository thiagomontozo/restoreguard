# ADR 009: Argon2id and session authentication

Status: Accepted (2026-08-20)

Local passwords use Argon2id. Random server-side sessions are stored as hashes and delivered through HttpOnly SameSite cookies. Mutations use a separate CSRF token and origin validation. JWT in localStorage is rejected because revocation and browser secret exposure are worse for this control-plane application.
