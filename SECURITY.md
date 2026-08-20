# Security Policy

RestoreGuard is experimental. Do not connect this version to production backup repositories or production databases without an independent security review.

## Reporting a vulnerability

Please use GitHub's private **Security Advisories** feature for this repository. Do not open a public issue with exploit details, credentials, backup metadata, tenant data, or evidence artifacts. Include the affected revision, a minimal synthetic reproduction, impact, and suggested mitigation when possible. No response SLA is promised at the experimental stage.

## Scope and safe research

Use only synthetic data and disposable Docker resources that you own. Do not test against third-party systems, production backup sources, public object storage, or other organizations. Never include real secrets in reports.

The highest-risk surfaces are Docker socket access, malicious backup/SQL artifacts, cross-organization authorization, secret handling, SSRF-capable checks/webhooks, evidence access, and cleanup after cancellation or failure. See `docs/threat-model.md`.
