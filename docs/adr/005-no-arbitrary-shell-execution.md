# ADR 005: No arbitrary shell execution

Status: Accepted (2026-08-20)

Restore plans contain typed operations, never user-provided Bash, PowerShell, command text, script upload, Docker arguments, mounts, images, capabilities, or network modes. This intentionally limits flexibility to preserve authorization, auditability, validation, and sandbox safety.
