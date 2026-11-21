---
title: ADR 0006 Docker-based backend tests
type: decision
status: verified-against-code
last_verified: 2026-11-21
---

# ADR 0006: Docker-based backend tests

Status: Accepted (2026-11-21)

## Context
On the primary dev machine Windows Application Control blocks every Go-built executable, including `go test` binaries, `go run` and `bin/seed`. Backend integration tests also need Postgres and Redis. The baseline of 160 passing tests was produced this way.

## Decision
Backend tests on this machine run through `scripts/test-backend.ps1` (and `scripts/test-backend.sh` inside the container): start Postgres 16 and Redis 7 with docker compose (project `hpay`), apply all migrations with the `migrate/migrate:v4.17.1` image, then run `go test` in a `golang:1.25` container with `TEST_DATABASE_URL` and `REDIS_ADDR` set. CI runs raw `go test` against service containers. Docker Desktop must be running.

## Consequences
- Tests run despite the policy and match CI's database versions.
- Slower startup; module and build caches are kept in named volumes (`hpay-gomod`, `hpay-gobuild`).
- Tests share the development database (not isolated), the script does not set `TEST_REDIS_ADDR` or pass `-p 1`; those differences from CI should be closed.
- Only the container may execute Go; local binaries (`go run ./cmd/seed`) may need the same wrapper.