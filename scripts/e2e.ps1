$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root
try { go test -C backend -p 1 -tags=e2e -timeout 10m ./internal/drill -run 'TestPostgresRecoveryE2E|TestCorruptBackupFailsSafely' -v } finally { & "$PSScriptRoot/cleanup.ps1" }
