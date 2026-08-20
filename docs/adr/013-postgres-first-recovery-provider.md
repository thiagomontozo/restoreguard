# ADR 013: PostgreSQL-first recovery

Status: Accepted (2026-08-20)

PostgreSQL logical dumps provide a meaningful end-to-end recovery target that can be generated and restored synthetically in CI. The project validates this path deeply before claiming more database coverage. Dumps execute only in a new isolated instance with typed checks.
