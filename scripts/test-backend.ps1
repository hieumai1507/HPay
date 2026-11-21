# Run backend tests in Docker (Postgres + Redis + Go 1.25). Needs Docker Desktop running.
# Usage: .\scripts\test-backend.ps1 [-GoArgs "./internal/ledger -race -count=1"] [-Db agent1]
#
# Why Docker: Windows Application Control blocks Go-built test executables on the dev machine.
# Why -Db: several people or agents can run tests at the same time. They share one Postgres
# container but each -Db name gets its own database (created and migrated on demand), so
# parallel runs cannot deadlock each other or steal each other's outbox events. The default
# database "HPay" is the shared one used by a single developer.
param(
  [string]$GoArgs = "./... -count=1",
  [string]$Db = "HPay"
)
$ErrorActionPreference = "Continue"   # docker writes status lines to stderr
if ($Db -notmatch '^[A-Za-z][A-Za-z0-9_]{0,40}$') { throw "invalid -Db name '$Db' (letters, digits, underscore)" }
$root = Split-Path $PSScriptRoot -Parent
Push-Location "$root\src\backend"
try {
  docker compose -p hpay up -d postgres redis 2>&1 | Out-Null
  if ($Db -ne "HPay") {
    $exists = docker exec hpay-postgres-1 psql -U HPay -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$Db'" 2>$null
    if (-not $exists) { docker exec hpay-postgres-1 psql -U HPay -d postgres -c "CREATE DATABASE $Db" 2>&1 | Out-Null }
  }
  $mig = "postgres://HPay:HPay@postgres:5432/${Db}?sslmode=disable"
  docker run --rm --network hpay_default -v "${PWD}\db\migrations:/migrations" migrate/migrate:v4.17.1 -path=/migrations -database $mig up 2>&1 | Select-Object -Last 2
  docker run --rm --network hpay_default `
    -v "${PWD}:/src" -v "$root\src\admin\src\stores:/admin-stores:ro" -v "$root\scripts\test-backend.sh:/run-tests.sh" `
    -v hpay-gomod:/go/pkg/mod -v hpay-gobuild:/root/.cache/go-build `
    -e TEST_DATABASE_URL=$mig -e REDIS_ADDR=redis:6379 -e APP_ENV=ci -e ADMIN_STORE_FILE=/admin-stores/admin.ts -e GOFLAGS=-buildvcs=false -e TEST_DB_MAX_CONNS=$env:TEST_DB_MAX_CONNS `
    golang:1.26 bash /run-tests.sh @($GoArgs -split ' ')
} finally { Pop-Location }