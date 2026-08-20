$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root
try {
  docker compose -f compose.test.yml up -d --wait
  $env:RESTOREGUARD_TEST_DATABASE_URL = 'postgres://restoreguard:restoreguard-test-only@127.0.0.1:55433/restoreguard_test?sslmode=disable'
  $env:RESTOREGUARD_TEST_S3_ENDPOINT = '127.0.0.1:59002'
  go test -C backend -p 1 ./...
  go test -C backend -p 1 -tags=integration ./...
  docker run --rm --label com.restoreguard.managed=true --label com.restoreguard.purpose=test -v "${root}/frontend:/app" -w /app node:22.19-alpine sh -c 'npm ci && npm run lint && npm run typecheck && npm test && npm run build'
} finally {
  docker compose -f compose.test.yml down -v --remove-orphans
}
