# Authentication

Local authentication stores Argon2id password hashes and random hashed sessions. Login returns an HttpOnly session cookie and a separate CSRF token; no JWT or secret is stored in localStorage. Sessions expire after twelve hours by default, can be revoked at logout, and are all revoked on password change.

Cookies are SameSite=Strict. `Secure` is configuration-driven for local HTTP and must be true behind HTTPS. Reverse proxies must preserve scheme correctly and never log cookie/header values. Login failures use a generic message.

Users include an encrypted TOTP-secret slot and `mfa_enabled` foundation. v0.1 deliberately does not claim complete MFA enrollment, recovery codes, or administrator enforcement UX.
