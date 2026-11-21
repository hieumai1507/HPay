DB_URL ?= postgres://HPay:HPay@localhost:5432/HPay?sslmode=disable
MIGRATE = migrate -path db/migrations -database "$(DB_URL)"

.PHONY: dev build test lint sqlc migrate-up migrate-down migrate-verify up down seed loadtest

dev:            ## run everything locally (needs docker compose up first)
	go run ./cmd/server --role=all

build:
	go build -o bin/hpay ./cmd/server

test:
	go test ./... -count=1

test-integration: ## needs docker compose up + migrate-up; uses the dev database
	TEST_DATABASE_URL="$(DB_URL)" go test ./... -count=1

lint:
	go vet ./...
	golangci-lint run

sqlc:
	sqlc generate

migrate-up:
	$(MIGRATE) up

migrate-down:
	$(MIGRATE) down -all

migrate-verify: ## E18: up -> down -> up must all succeed
	$(MIGRATE) up && $(MIGRATE) down -all && $(MIGRATE) up

seed: ## F10: idempotent demo data (needs migrate-up); asserts balance == SUM(ledger)
	go build -o bin/seed ./cmd/seed
	DATABASE_URL="$(DB_URL)" ./bin/seed

loadtest: ## F10: 100 TPS transfer for 60s against a running server (make dev + seed first)
	k6 run scripts/loadtest/k6-transfer.js

up:
	docker compose up -d

down:
	docker compose down -v
