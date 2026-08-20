$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)
docker compose up -d --build
docker compose ps
Write-Output 'RestoreGuard: http://localhost:55173'
