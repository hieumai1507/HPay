---
title: Glossary
type: reference
status: generated-from-source
last_verified: 2026-11-21
code: [src/backend/internal, src/backend/db/migrations]
---

# Glossary

Domain docs live in `docs/domain/<name>.md` (some may still be in progress).

## Money and ledger

| Term | Definition | Doc |
|---|---|---|
| Minor unit | All amounts are `BIGINT` VND with no decimals; floats are never used. | [ledger](../domain/ledger.md) |
| Ledger entry | One debit or credit row in `ledger_entries` (append-only, enforced by trigger). A transaction posts balanced legs. | [ledger](../domain/ledger.md) |
| Double entry / Post | `ledger.Post` writes all legs of a transaction atomically and updates wallet balances; errors `ErrUnbalanced`, `ErrInsufficientBalance`. | [ledger](../domain/ledger.md) |
| Ledger account | Row in `ledger_accounts` (SYS_CASH, SYS_FEE, SYS_SUSPENSE) that maps a system wallet to a chart-of-accounts type. | [ledger](../domain/ledger.md) |
| Wallet | Balance holder for a user, organization or the system, with `purpose` (main, vault, system pools) and optimistic `version`. | [wallet](../domain/wallet.md) |
| Main wallet | The single `purpose='main'` wallet per owner and currency; self-healed on login (`EnsureMainWallet`). | [wallet](../domain/wallet.md) |
| System wallet | Wallet owned by the system user with a `system_code`: SYSTEM_CASH_IN (funding pool), SYSTEM_FEE (fee income), SYSTEM_SUSPENSE. May run negative. | [wallet](../domain/wallet.md) |
| Suspense | System wallet reserved for money in transit or unresolved items; pairs with the reversal and recon flows. | [ledger](../domain/ledger.md) |
| Vault | A user wallet with `purpose='vault'` and a unique label, used for savings; money moves only through the engine (service VAULT_SAVINGS). | [wallet](../domain/wallet.md) |
| Round-up | Optional workflow step (ROUND_UP) that sweeps a spend's rounding difference into the single vault flagged `round_up_target`. | [wallet](../domain/wallet.md) |
| Top-up / cash-in | Money entering from the (mock) bank into a user wallet from SYSTEM_CASH_IN; confirmed by the mock bank callback. | [wallet](../domain/wallet.md) |
| Fee policy | Row in `fee_policies` (fixed fee, basis points, min, max) chosen by txn type and KYC status. | [transfer-and-fees](../domain/transfer-and-fees.md) |
| Ref code | Public transaction identifier (`transactions.ref_code`, up to 24 chars), used in URLs and recon. | [transfer-and-fees](../domain/transfer-and-fees.md) |
| Idempotency key | Client header `Idempotency-Key` stored in `transactions.idempotency_key` (globally unique) so retries return the same transaction. | [transfer-and-fees](../domain/transfer-and-fees.md) |
| Reversal | A new transaction of type `reversal` (`reversal_of` points to the original) created after approval; ledger history is never edited. | [reversal](../domain/reversal.md) |
| Compensation | Workflow rollback of a completed MOVE_FUNDS step when a later step fails (`on_failure: compensate`). | [workflow-engine](../domain/workflow-engine.md) |
| Money request | A user asking another user for money (`money_requests`); split bill creates several sharing a `split_group_id`. | [money-requests](../domain/money-requests.md) |

## Service catalog and engine

| Term | Definition | Doc |
|---|---|---|
| Service | A catalog entry (`services`): code (P2P_TRANSFER, TOPUP_BANK, ...), status, action pipeline, and dispatch metadata (workflow_name, ledger_txn_type, wallet_strategy). | [service-catalog](../domain/service-catalog.md) |
| Wallet strategy | `services.wallet_strategy`: user_to_user, syscash_to_user, org_to_user, user_to_vault; decides sender and receiver wallets. | [service-catalog](../domain/service-catalog.md) |
| Offer | Package assigned to a user (`user_offers`) listing which services are enabled with limits; e.g. OFFER_CUST_L2. | [service-catalog](../domain/service-catalog.md) |
| Tier (KYC) | Customer level 1 or 2 expressed as an offer (OFFER_CUST_L1/L2) and `kyc_status`; drives limits and fees. | [kyc](../domain/kyc.md) |
| Tier (approval) | One amount band of an approval policy stating how many level-1/2/3 checkers are required. | [approval-maker-checker](../domain/approval-maker-checker.md) |
| OfferGuard | Fail-closed check of service availability, min/max, daily/monthly volume and interval before a transaction starts. | [service-catalog](../domain/service-catalog.md) |
| Auth matrix | `offer_auth_matrix`: amount ranges mapped to none/pin/otp/2fa. Currently returned as a label; not enforced server-side. | [service-catalog](../domain/service-catalog.md) |
| Action pipeline | Per-service list of third-party actions (`services.actions`) run by the `action_pipeline` step before or after the GL posting (`action_priority`). | [service-catalog](../domain/service-catalog.md) |
| Transaction engine | The 3-step API `request`, `confirm`, `verify` used by all services. | [transfer-and-fees](../domain/transfer-and-fees.md) |

## Workflow

| Term | Definition | Doc |
|---|---|---|
| Workflow definition | Versioned JSON list of steps per name in `workflow_definitions`; only one active version per name is used for new runs. | [workflow-engine](../domain/workflow-engine.md) |
| Instance | A running execution (`workflow_instances`) bound to a transaction, with status and JSON context. | [workflow-engine](../domain/workflow-engine.md) |
| Step | One unit in a definition (VALIDATE, FRAUD_CHECK, MOVE_FUNDS, CONFIRM, ...) with handler, retry, timeout, `on_failure`. | [workflow-engine](../domain/workflow-engine.md) |
| Step execution | Attempt record in `step_executions`; unique per (instance, step, attempt) and used as idempotency arbiter. | [workflow-engine](../domain/workflow-engine.md) |
| Handler | Go function registered under a name (validator, fraud, wallet, confirmer, action_pipeline, round_up, kyc_scorer, kyc_classifier). | [workflow-engine](../domain/workflow-engine.md) |
| human_approval | Step type that parks the instance in `waiting_approval` and creates an approval request. | [approval-maker-checker](../domain/approval-maker-checker.md) |
| Worker / Recovery | Stream consumer executing steps; recovery re-drives stuck instances. | [workflow-engine](../domain/workflow-engine.md) |

## Events

| Term | Definition | Doc |
|---|---|---|
| Outbox | Table `outbox` written in the business transaction; the relay publishes rows to Redis Streams (at-least-once). | [outbox-and-events](../domain/outbox-and-events.md) |
| Relay | Only writer to Redis Streams; reads unpublished outbox rows. | [outbox-and-events](../domain/outbox-and-events.md) |
| Role (process) | `api`, `worker`, `admin`, `all`: which surfaces a process runs. See configuration. | [outbox-and-events](../domain/outbox-and-events.md) |

## Organization and approval

| Term | Definition | Doc |
|---|---|---|
| Organization | Business account with its own wallet and members. | [organization](../domain/organization.md) |
| Maker | Org member who creates a disbursement or cancels their own request. | [approval-maker-checker](../domain/approval-maker-checker.md) |
| Checker | Org member (checker_level 1 to 5) who approves or rejects; level must match the current tier. | [approval-maker-checker](../domain/approval-maker-checker.md) |
| Approval policy | Versioned, per-org rule set mapping amount ranges to required checker levels; snapshotted on each request. | [approval-maker-checker](../domain/approval-maker-checker.md) |
| Approval request | Pending decision for an org transaction or a reversal; statuses pending, approved, rejected, expired, cancelled. | [approval-maker-checker](../domain/approval-maker-checker.md) |
| Disbursement | Org-to-user payout (`org_disbursement`) that requires approval. | [organization](../domain/organization.md) |

## Identity, KYC, risk

| Term | Definition | Doc |
|---|---|---|
| Access / refresh token | Short-lived JWT plus rotating opaque refresh token with family ids; reuse revokes the family. | [identity-and-auth](../domain/identity-and-auth.md) |
| Admin | Back-office user (`admin_users`) with token aud `admin`; roles viewer, kyc_reviewer, risk_officer, ops, superadmin (not enforced). | [identity-and-auth](../domain/identity-and-auth.md) |
| OTP challenge | One-time code (`otp_challenges`) used for approval decisions; max 5 attempts. | [identity-and-auth](../domain/identity-and-auth.md) |
| KYC | Identity verification by ID card and selfie, AI scored, then auto-approved or manually reviewed. | [kyc](../domain/kyc.md) |
| Fraud flag | Row raised by the fraud step with rule code and score; reviewed by an admin (open, cleared, confirmed_fraud). | [fraud-and-recon](../domain/fraud-and-recon.md) |
| Reconciliation (recon) | Nightly comparison of top-ups against bank statements, producing a run, items and an xlsx report; also checks balance = SUM(ledger). | [fraud-and-recon](../domain/fraud-and-recon.md) |
| Mock bank | Simulated bank gateway and callback endpoint under `/internal/`. | [fraud-and-recon](../domain/fraud-and-recon.md) |
| AI advisor | LLM (or mock) used for KYC scoring and fraud advice; configured per function in `ai_function_settings`. | [ai-integration](../domain/ai-integration.md) |
| Signed URL | HMAC link with expiry used to serve stored KYC images and reports. | [kyc](../domain/kyc.md) |

## Technical

| Term | Definition |
|---|---|
| sqlc | Generates typed Go queries into `internal/store/gen` from SQL files. |
| golang-migrate | Applies `db/migrations/*.up.sql` and `.down.sql`. |
| Edge case IDs (E, M, A) | Test-case identifiers (E01 to E20 edge, M mobile, A admin) used in comments and `scripts/check-edge-tests.sh`. |
| F1 to F10 | Backend delivery phases named in code comments. |
| Audit log | `audit_logs` rows with before/after JSON for admin actions. |
