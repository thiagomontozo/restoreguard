#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
trap 'docker compose -f compose.test.yml down -v --remove-orphans' EXIT
docker compose -f compose.test.yml up -d --wait
export RESTOREGUARD_TEST_DATABASE_URL='postgres://restoreguard:restoreguard-test-only@127.0.0.1:55433/restoreguard_test?sslmode=disable'
export RESTOREGUARD_TEST_S3_ENDPOINT='127.0.0.1:59002'
go test -C backend -p 1 ./...
go test -C backend -p 1 -tags=integration ./...
