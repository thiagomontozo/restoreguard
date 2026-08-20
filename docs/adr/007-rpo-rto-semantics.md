# ADR 007: RPO and RTO semantics

Status: Accepted (2026-08-20)

RPO is selected snapshot age at drill time; RTO spans drill start through completion of required validations. Each compares independently with policy and returns PASS/FAIL/INCONCLUSIVE. Backup frequency is not RPO, restore duration alone is not RTO, and objective failure is distinct from technical failure.
