@echo off
REM Run the API against the LOCAL docker Postgres, never the cloud DATABASE_URL
REM in .env. godotenv does not override variables already present in the
REM environment, so setting it here wins.
cd /d "%~dp0.."
set "DATABASE_URL=postgres://HPay:HPay@localhost:5432/HPay?sslmode=disable"
set "REDIS_ADDR=localhost:6379"
set "APP_ENV=dev"
go run ./cmd/server
