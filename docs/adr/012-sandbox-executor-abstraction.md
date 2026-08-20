# ADR 012: Sandbox executor abstraction

Status: Accepted (2026-08-20)

Orchestration depends on Create/Destroy and a typed spec, not Docker commands. Docker is the sole v0.1 adapter. Kubernetes, VM, cloud, and remote runner are extension points only and must preserve cancellation, resource, network, evidence, and cleanup guarantees.
