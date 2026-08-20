$ErrorActionPreference = 'Continue'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root
docker compose down -v --remove-orphans
docker compose -f compose.test.yml down -v --remove-orphans
$containers = docker ps -aq --filter 'label=com.restoreguard.managed=true'
if ($containers) { docker rm -f $containers }
$networks = docker network ls -q --filter 'label=com.restoreguard.managed=true'
if ($networks) { docker network rm $networks }
$volumes = docker volume ls -q --filter 'label=com.restoreguard.managed=true'
if ($volumes) { docker volume rm $volumes }
Write-Output 'RestoreGuard Docker cleanup completed.'
