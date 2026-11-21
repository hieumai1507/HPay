# ⚡ 06 — Plan: Restructuring & Streamlining the Backend API (Service Engine Consolidation)

A detailed plan to review and remove redundant APIs and consolidate all monetary transaction flows into the **standard 3-step Engine System (HPay Service Engine)**.

## 1. API Audit & Pruning List

All the previously scattered individual transaction APIs will be removed and moved entirely over to the Service Engine API:

| Old API (to be removed / streamlined) | Status | Standard replacement Engine API | Service Catalog code (`service_code`) |
|---|---|---|---|
| `POST /v1/transactions/transfer` | ❌ REMOVE (REDUNDANT) | `POST /v1/transactions/request` | `P2P_TRANSFER` |
| `POST /v1/wallets/topup` | ❌ REMOVE (REDUNDANT) | `POST /v1/transactions/request` | `TOPUP_BANK` |
| `POST /v1/money-requests/:id/pay` | ❌ REMOVE (REDUNDANT) | `POST /v1/transactions/request` | `MONEY_REQUEST` |
| `POST /v1/org/:id/topup` | ❌ REMOVE (REDUNDANT) | `POST /v1/transactions/request` | `TOPUP_BANK` |
| `POST /v1/org/:id/transactions` | ❌ REMOVE (REDUNDANT) | `POST /v1/transactions/request` | `ORG_DISBURSEMENT` |
| `POST /v1/transactions/request` | ⚡ ENGINE CORE | Initiate a transaction by Service Code | Every `service_code` in the Catalog |
| `POST /v1/transactions/confirm` | ⚡ ENGINE CORE | PIN / OTP authentication & Action Pipeline posting | Every `service_code` in the Catalog |
| `POST /v1/transactions/verify` | ⚡ ENGINE CORE | 2FA authentication for large-limit transactions | Every `service_code` in the Catalog |

## 2. Detailed Task Execution Breakdown

**Task 1: Clean up the Backend Router (`src/backend/internal/api/router.go`)**

Remove the redundant endpoints `/transactions/transfer`, `/wallets/topup`, `/money-requests/:id/pay`, `/org/:id/transactions` from the `router.go` file. Keep only the Engine API set of 3: `/v1/transactions/request`, `confirm`, `verify`.

**Task 2: Dynamic Service Engine Dispatcher (`src/backend/internal/transfer/service.go`)**

Add a Switch/Dispatcher that reads the `service_code` from the PostgreSQL DB. When receiving a transaction request, the Engine automatically triggers the Action Pipeline (GL Ledger Posting + Bank Gateway HTTP Call) corresponding to that Service.

**Task 3: Update the Mobile App Client (`src/mobile`)**

Convert all the Transfer, Request Money, and Payroll screens to call only `useTransactionEngine()`.

**Task 4: Sync the Admin Ops Catalog (`src/admin`)**

Make sure Admin Ops can toggle and configure the Action Pipeline for all Services in the DB.
