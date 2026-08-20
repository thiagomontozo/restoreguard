# Secrets

`SecretStore` uses AES-256-GCM with a random nonce and organization ID as additional authenticated data. The database can retain encrypted bytes and key-version metadata or an external `credentialReference`; plaintext is never stored. The 32-byte master key is supplied externally in base64 through `RESTOREGUARD_MASTER_KEY` and is never generated into repository files.

The master key must come from a deployment secret mechanism, be backed up separately, rotated deliberately, and never logged. Rotation decrypts with the previous version and re-encrypts transactionally with the new version while audit metadata contains identifiers only.

Future adapters may implement HashiCorp Vault, AWS Secrets Manager, Azure Key Vault, or GCP Secret Manager behind the same interface. They are not implemented in v0.1.
