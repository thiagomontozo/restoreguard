# ADR 001: Modular monolith

Status: Accepted (2026-08-20)

RestoreGuard begins as one Go control-plane deployment with internal domain, application, and adapter boundaries. This minimizes operational failure modes and transactional complexity while the recovery model evolves. A remote runner may be extracted only when isolation or scale justifies the network boundary; Kafka/RabbitMQ/NATS/Kubernetes are not v0.1 requirements.
