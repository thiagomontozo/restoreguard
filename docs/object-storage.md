# Object storage

`ObjectStorage` provides streaming `Put`, `Get`, `Delete`, and health operations. v0.1 includes a local filesystem adapter and an S3-compatible adapter tested against MinIO. Large artifacts are not stored in PostgreSQL.

Keys are generated from trusted organization/drill/artifact IDs and a restrictive character set. `..`, backslashes, absolute paths, and paths escaping the configured root are rejected. Puts enforce declared and observed byte limits, hash while copying, use a temporary file locally, and rename only after a complete write.

PostgreSQL and object storage cannot share a transaction. The service writes an object, commits its metadata, and deletes the object if the database commit fails. Retention jobs apply the same explicit, auditable compensation pattern.
