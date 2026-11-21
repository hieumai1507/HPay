# hpay-backend

Go backend for HPay — Low-Code Service & Workflow Engine on a double-entry ledger.
Design docs: [hpay-docs](https://github.com/hieumai1507/hpay-docs).

## Quick Start

```bash
cp .env.example .env
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest   # one-time
make sqlc            # generate type-safe store code (required before build)
go mod tidy          # first run: generates go.sum
make up              # postgres 16 + redis 7 (Docker)
make migrate-up      # runs 5 clean consolidated migrations (000001 - 000005)
make dev             # runs --role=all, then: curl localhost:8080/healthz
```

## Consolidated Database Migrations (000001 - 000005)

HPay uses 5 clean consolidated migration files:
1. `000001_users_admin_auth.up.sql`: Users, Admin Users, Refresh Tokens, OTP Challenges, Audit Logs.
2. `000002_wallets_double_entry_ledger.up.sql`: Wallets, Ledger Accounts, Transactions, Ledger Entries, System Wallets.
3. `000003_services_transaction_design.up.sql`: Service Catalog, Transaction Design, Workflow Definitions, Outbox Events.
4. `000004_offer_factory_policies.up.sql`: Offers, Offer Services, Auth Matrix, Fee Policies, Money Requests.
5. `000005_orgs_approval_fraud.up.sql`: Organizations, Org Members, Approval Policies, KYC, Fraud, Recon Runs, AI Settings.

## Unified 3-Step Service Engine API

All financial transactions are handled dynamically through the Service Engine:
- `POST /v1/transactions/request`: Initiates transaction based on `service_code` (e.g. `P2P_TRANSFER`, `MONEY_REQUEST`, `TOPUP_BANK`, `ORG_DISBURSEMENT`).
- `POST /v1/transactions/confirm`: Verifies PIN/OTP and executes Action Pipeline (GL Ledger Double-Entry Posting + Bank Gateway HTTP calls).
- `POST /v1/transactions/verify`: Verifies 2FA biometric/OTP.

## Roles

One binary, four roles: `--role=api|worker|admin|all`.
