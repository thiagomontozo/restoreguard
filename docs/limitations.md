# Limitations

RestoreGuard v0.1 is experimental. Docker is the only executor and PostgreSQL logical dumps are the primary fully validated recovery workflow. It has no Veeam, Restic, Borg, pgBackRest, cloud snapshot, VM, Kubernetes, or remote runner adapter. Roadmap names are not present capabilities.

TOTP has a data-model foundation but no full enrollment/recovery workflow. Secrets use an external-key local AES-GCM implementation, not Vault/HSM. The scheduler/worker model is not proven for large distributed deployments. HTTP/webhook defenses are foundations, not a general-purpose outbound integration platform. No production-scale load, archive extraction, physical PostgreSQL/base-backup recovery, point-in-time recovery, or disaster-site networking is validated.

A passed drill is evidence for one artifact, plan, image, sandbox, validation set, and time. It does not guarantee recovery for every outage, corrupted dependency, infrastructure loss, credential failure, workload scale, or production topology. When observations are insufficient, RestoreGuard reports `INCONCLUSIVE`.
