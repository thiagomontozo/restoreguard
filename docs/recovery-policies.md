# Recovery policies

A recovery policy belongs to an organization and protected asset. It defines positive bounded RPO/RTO targets, a validated daily/weekly/monthly schedule, required typed checks, retention, and enabled state. Discovery frequency is not the RPO result.

The scheduler uses PostgreSQL advisory locking so only one instance leases due jobs. v0.1 targets a single control-plane deployment; distributed scheduling and recovery of abandoned running jobs require further hardening.

Snapshot freshness is evaluated against the asset policy, never a global 24-hour default. Without a suitable policy or trustworthy snapshot completion timestamp the result is `UNKNOWN`/`INCONCLUSIVE`.
