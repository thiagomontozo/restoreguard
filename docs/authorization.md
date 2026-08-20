# Authorization

Initial roles are `OWNER`, `ADMIN`, `RECOVERY_ENGINEER`, `OPERATOR`, `AUDITOR`, and `VIEWER`. Permissions cover backup sources, policies, drill read/run, evidence, report export, users, audit, and settings. Owner/admin receive all current permissions; other roles receive explicit subsets.

The backend enforces permissions before handlers and derives `organization_id` from the authenticated session. IDs sent by the browser identify a resource only; every query also constrains the session organization. The frontend may hide controls for usability but is not an authorization boundary.

Role changes and future MFA-required administrative roles should revoke affected sessions. Organization isolation is covered by real PostgreSQL integration tests.
