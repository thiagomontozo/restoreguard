# ADR 010: Organization isolation

Status: Accepted (2026-08-20)

Every important entity carries `organization_id`. Repository queries constrain it from authenticated context, never browser input. Roles are organization-scoped and integration tests prove cross-organization asset access and drill reuse are denied. PostgreSQL RLS remains a future defense-in-depth option.
