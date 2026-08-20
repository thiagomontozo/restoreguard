# Domain model

The central aggregate is `RecoveryDrill`. A protected asset can use several backup sources; discovery upserts source-scoped external snapshot IDs. A policy supplies RPO/RTO targets, schedule, required validations, and retention. A restore plan contains typed steps only.

```mermaid
erDiagram
  ORGANIZATION ||--o{ USER : contains
  ORGANIZATION ||--o{ PROTECTED_ASSET : owns
  ORGANIZATION ||--o{ BACKUP_SOURCE : owns
  PROTECTED_ASSET }o--o{ BACKUP_SOURCE : uses
  BACKUP_SOURCE ||--o{ BACKUP_SNAPSHOT : discovers
  PROTECTED_ASSET ||--o{ RECOVERY_POLICY : governed_by
  PROTECTED_ASSET ||--o{ RESTORE_PLAN : restored_by
  PROTECTED_ASSET ||--o{ RECOVERY_DRILL : tested_by
  BACKUP_SNAPSHOT ||--o{ RECOVERY_DRILL : selected_for
  RECOVERY_POLICY ||--o{ RECOVERY_DRILL : evaluates
  RECOVERY_DRILL ||--o{ DRILL_STEP : records
  RECOVERY_DRILL ||--o| RECOVERY_SANDBOX : creates
  RECOVERY_DRILL ||--o{ VALIDATION_RESULT : produces
  VALIDATION_CHECK ||--o{ VALIDATION_RESULT : evaluates
  RECOVERY_DRILL ||--o{ EVIDENCE : proves
  EVIDENCE_ARTIFACT ||--o{ EVIDENCE : supports
  RECOVERY_DRILL ||--o| RECOVERY_REPORT : summarizes
  ORGANIZATION ||--o{ AUDIT_EVENT : records
```

Every important row is organization-scoped. The organization comes from the authenticated session, never from a browser-provided organization ID. `DrillStatus` describes orchestration; `RecoveryAssessment` describes evidence quality. Policy results are independent so a restore can pass while RPO or RTO fails.
