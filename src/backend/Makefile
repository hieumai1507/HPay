DB_URL ?= postgres://HPay:HPay@localhost:5432/HPay?sslmode=disable
MIGRATE = migrate -path db/migrations -database "$(DB_URL)"

.PHONY: dev build test lint sqlc migrate-up migrate-down migrate-verify up down seed loadtest

dev:            ## run everything locally (needs docker compose up first)
	APP_ENV=dev go run ./cmd/server --role=all

build:
	go build -o bin/hpay ./cmd/server

test:
	go test ./... -count=1

test-integration: ## needs docker compose up + migrate-up; uses the dev database; -p 1 because packages share it
	TEST_DATABASE_URL="$(DB_URL)" APP_ENV=ci go test -p 1 ./... -count=1

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
	APP_ENV=dev DATABASE_URL="$(DB_URL)" ./bin/seed

# Load test of the real transfer path (request, confirm with PIN, workflow, ledger). Needs Postgres and Redis\n# running and a FRESH migrated database; leftover rows from other tests distort the numbers.\n# Tune with LOAD_USERS, LOAD_WORKERS and LOAD_SECONDS. Results and history: docs/quality/performance/.\nLOAD_DB_URL ?= postgres://HPay:HPay@localhost:5432/load?sslmode=disable\nloadtest: ## Go load test of the P2P path (create the database 'load' and migrate it first)
	TEST_DATABASE_URL="$(LOAD_DB_URL)" REDIS_ADDR=localhost:6379 APP_ENV=ci \
	  go test -tags loadtest ./test/loadtest -run TestP2PLoad -v -count=1 -timeout 15m

# HTTP load test with k6. The old script is stale (see scripts/loadtest/README.md); not wired up.

up:
	docker compose up -d

down:
	docker compose down -v
