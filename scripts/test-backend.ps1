# Chay test backend trong Docker (Postgres + Redis + Go 1.25). Yeu cau: Docker Desktop dang chay.
# Dung: .\scripts\test-backend.ps1 [-GoArgs "./internal/ledger -race -count=1"]
param([string]$GoArgs = "./... -count=1")
$ErrorActionPreference = "Continue"   # docker ghi trang thai ra stderr
$root = Split-Path $PSScriptRoot -Parent
Push-Location "$root\src\backend"
try {
  docker compose -p hpay up -d postgres redis 2>&1 | Out-Null
  $mig = "postgres://HPay:HPay@postgres:5432/HPay?sslmode=disable"
  docker run --rm --network hpay_default -v "${PWD}\db\migrations:/migrations" migrate/migrate:v4.17.1 -path=/migrations -database $mig up 2>&1 | Select-Object -Last 2
  docker run --rm --network hpay_default `
    -v "${PWD}:/src" -v "$root\scripts\test-backend.sh:/run-tests.sh" `
    -v hpay-gomod:/go/pkg/mod -v hpay-gobuild:/root/.cache/go-build `
    -e TEST_DATABASE_URL=$mig -e REDIS_ADDR=redis:6379 -e GOFLAGS=-buildvcs=false `
    golang:1.25 bash /run-tests.sh @($GoArgs -split ' ')
} finally { Pop-Location }