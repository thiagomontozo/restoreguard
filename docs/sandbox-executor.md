# Sandbox executor

`SandboxExecutor` separates orchestration from Docker. The Docker implementation accepts a server-controlled `SandboxSpec`; browser input cannot supply image, Docker arguments, mounts, capabilities, privilege, or network mode. Allowed PostgreSQL images come from configuration and should be pinned by digest in production.

Each drill receives unique `restoreguard-*` names, `com.restoreguard.managed=true` labels, an internal network, bounded memory/CPU/PIDs, read-only root filesystem, bounded tmpfs, and a dedicated data volume. It does not use privileged or host networking and does not mount arbitrary host paths. Readiness, total drill time, cancellation, and cleanup are bounded.

Mounting `/var/run/docker.sock` gives the process powerful host control. The Compose mount is development-only. The future design places Docker behind a separate runner agent and authenticated typed protocol.
