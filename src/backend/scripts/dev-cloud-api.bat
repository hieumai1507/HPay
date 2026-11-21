@echo off
REM Admin-API run against whatever DATABASE_URL .env points at (the Neon dev DB).
REM role=admin mounts /v1/admin/* and serves HTTP but RunsWorkers() is false, so
REM no sweeper/outbox relay mutates the seeded demo state while inspecting it.
cd /d "%~dp0.."
go run ./cmd/server -role admin
