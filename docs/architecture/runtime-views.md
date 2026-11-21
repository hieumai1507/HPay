---
title: Runtime views
type: architecture
status: verified-against-code
last_verified: 2026-11-21
code: [src/backend/internal/api/router.go, src/backend/internal/transfer/service.go, src/backend/internal/workflow/engine.go, src/backend/internal/workflow/consumer.go, src/backend/internal/workflow/handlers/handlers.go, src/backend/internal/outbox/relay.go, src/backend/internal/ledger/ledger.go, src/backend/internal/org/service.go, src/backend/internal/approval/service.go, src/backend/internal/kyc, src/backend/internal/wallet/callback.go, src/backend/internal/recon]
---

# Runtime views

Common machinery used by scenarios 1, 2 and 3: a request handler inserts a `transactions` row and calls `engine.Start(tx, ...)`, which inserts a `workflow_instances` row and an `outbox` event `step.start` in the SAME database transaction. The outbox relay (200 ms poll, `FOR UPDATE SKIP LOCKED`) pushes events to Redis streams: `step.start` goes to `wf:steps:<handler>`, everything else to `wf:events` (`outbox/relay.go:88-98`). Workers execute the handler and, in one tx, write `step_executions` plus a `step.result` outbox event (`consumer.go:164`); the engine consumes results from `wf:events` and advances (`engine.go:82`).

## 1. P2P transfer via the engine

Mobile calls `POST /v1/transactions/request` (requires `Idempotency-Key`, `router.go:118`), then `POST /v1/transactions/confirm`.

```mermaid
sequenceDiagram
  participant M as Mobile
  participant A as API (transfer.ExecuteEngineRequest)
  participant DB as PostgreSQL
  participant R as Outbox relay
  participant S as Redis streams
  participant W as Worker
  participant E as Engine
  M->>A: POST /v1/transactions/request (Idempotency-Key)
  A->>DB: service by code, OfferGuard limits + auth method, fee, balance pre-check
  A->>DB: BEGIN; INSERT transactions(status created, idempotency_key UNIQUE)
  A->>DB: engine.Start -> INSERT workflow_instance + outbox step.start(VALIDATE); COMMIT
  A-->>M: 200 {transaction_id, ref_code, fee, total, auth_required}
  M->>A: POST /v1/transactions/confirm {transaction_id, pin}
  A-->>M: current status only (pin is NOT verified; ledger untouched)
  R->>S: XADD wf:steps:validator
  W->>DB: run VALIDATE, record step_execution + outbox step.result
  R->>S: XADD wf:events
  E->>DB: OnResult: advance to FRAUD_CHECK, ACTIONS_BEFORE_GL ...
  W->>DB: MOVE_FUNDS: tx { lock wallets ORDER BY id FOR UPDATE; ledger entries; balances }
  W->>DB: CONFIRM, ROUND_UP; transaction status completed
  M->>A: GET /v1/transactions/:refCode (poll)
  A-->>M: status completed
```

Verified details:
- `ExecuteEngineRequest` at `transfer/service.go:277-414`: unknown service code falls back to `P2P_TRANSFER`; insert conflict on the idempotency constraint returns the old transaction (`:368-386`).
- `ExecuteEngineConfirm` (`:515-528`) never posts to the ledger and does not check `PIN`/`OTP`; it only reads state. Money moves as soon as `request` commits (audit BE-C2). The mobile app hard-codes pin `123456` in the confirm call (`mobile/app/(main)/transfer/confirm.tsx:53`).
- Ledger posting: `ledger.PostWithQueries` locks wallets in id order, rejects overdraft on non-system wallets, inserts entries with running `balance_after`, then applies one balance delta per wallet (`ledger/ledger.go:77-141`). The `wallet` handler runs it in its own tx, separate from the `step_executions` write, which is the BE-C1 double-post window (`workflow/handlers/handlers.go:82-122`).
- Retries: `engine.OnResult` retries with backoff while `ev.Retryable && attempt < retry`; terminal failure on a compensatable step starts the compensation queue (`engine.go:150-160, 275`).

## 2. Org disbursement with maker-checker

```mermaid
sequenceDiagram
  participant Mk as Maker (mobile)
  participant A as API
  participant DB as PostgreSQL
  participant W as Worker + Engine
  participant Ck as Checker (mobile)
  Mk->>A: POST /v1/org/:id/transactions (Idempotency-Key)
  A->>DB: requireRole(maker); amount <= 1,000,000,000; org active; receiver; active approval policy for amount; balance soft check
  A->>DB: INSERT transaction(org_disbursement) + engine.Start(org_disbursement v2); COMMIT
  W->>DB: VALIDATE, FRAUD_CHECK, ACTIONS_BEFORE_GL
  W->>DB: step WAIT_APPROVAL (human_approval): PauseHook.OnPause creates approval_request (snapshot of policy tiers), instance -> waiting_approval
  Ck->>A: GET /v1/org/approvals (inbox)
  Ck->>A: POST /v1/org/approvals/:id/challenge (rate limited 10/min)
  A-->>Ck: OTP issued (code returned as otp_dev only when APP_ENV=dev)
  Ck->>A: POST /v1/org/approvals/:id/decide {approve, otp}
  A->>DB: VerifyOTP; BEGIN; lock approval row; eligibility (member, checker role, level == current tier, not expired); INSERT approval_action (unique per checker/level); Evaluate(snapshot, actions)
  alt all tiers approved
    A->>DB: FinalizeApprovalRequest + resumeWorkflow: step_execution WAIT_APPROVAL, transaction -> processing, outbox step.result; audit; COMMIT
    W->>DB: MOVE_FUNDS, CONFIRM -> completed
  else rejected
    A->>DB: finalize rejected, step result OK=false -> instance failed
  end
```

Verified: `org/service.go:362-445`, `approval/service.go:405-571`, `router.go:151-162`. Expiry: `approval.Sweeper` every 30 s. Known gaps: `eligibility` (`approval/service.go:221-250`) has no explicit `checkerID != createdBy` check (audit verification appendix "Org self-approval: CONFIRMED"); `org_members` is UNIQUE `(org_id,user_id)` so a user has a single role per org, which limits but does not remove the risk; the org transaction limit and balance are not re-checked after approval except by the ledger overdraft guard.

## 3. KYC submission and review

```mermaid
sequenceDiagram
  participant M as Mobile
  participant A as API (kyc.Submit)
  participant ST as Storage (disk/Cloudinary)
  participant DB as PostgreSQL
  participant W as Worker
  participant LLM as LLM or mock scorer
  participant Ad as Admin web
  M->>A: POST /v1/kyc/submit (multipart: id card + selfie)
  A->>A: ValidateImage (magic bytes, size), reject if pending/verified
  A->>ST: Put <uuid>/id_card.jpg, selfie.jpg
  A->>DB: tx: users.kyc_status none/rejected -> pending; INSERT kyc_requests; engine.Start(kyc_verification)
  W->>DB: step SCORE: load active ai_settings("kyc_scoring"), provider anthropic/openai/mock
  W->>LLM: score images
  W->>DB: step CLASSIFY: score >= 85 auto_approved; 50-84 manual_review; < 50 rejected (kyc/classify.go)
  alt manual_review
    Ad->>A: GET /v1/admin/kyc (queue, signed image URLs)
    Ad->>A: POST /v1/admin/kyc/:id/review {approve, reason}
    A->>DB: guarded UPDATE (second reviewer gets ERR_ALREADY_REVIEWED 409); users.kyc_status -> verified/rejected; audit_logs kyc.approve/kyc.reject
  end
  M->>A: GET /v1/kyc/status
```

Verified: `kyc/service.go:65-124`, `kyc/handlers.go:49-170`, `kyc/classify.go:7-40`, `kyc/admin.go:107-156`. Both `auto_approved` and `verified` select offer `OFFER_CUST_L1` (`kyc/offer.go`). Note the KYC workflow instance has `transaction_id = uuid.Nil` (`kyc/service.go:115`).

## 4. Bank top-up callback

```mermaid
sequenceDiagram
  participant B as Mock bank
  participant A as API (MockBankHandler)
  participant WS as wallet.BankCallback
  participant DB as PostgreSQL
  B->>A: POST /internal/mock-bank/callback {ref_code, amount, bank_ref} + X-Mock-Bank-Secret
  A->>A: constant-time secret compare else 401 ERR_INVALID_CALLBACK_SECRET
  A->>WS: BankCallback
  WS->>DB: BEGIN; SELECT topup by ref_code FOR UPDATE
  alt status created AND amount equals txn.amount
    WS->>DB: ledger.Post(system cash debit, user credit); status -> completed
  else duplicate or mismatch
    WS->>DB: no money change
  end
  WS->>DB: INSERT bank_statements (every callback, incl. duplicates); COMMIT
  WS-->>A: ok / ERR_CALLBACK_AMOUNT_MISMATCH 422
```

Verified: `wallet/callback.go:61-113`, `api/mockbank_handler.go`. The pending top-up row is created by `CreatePendingTopup` (`callback.go:24`) which is only called from tests and the seeder (comment `:22`). The interactive mobile top-up (`POST /v1/wallets/topup`) is NOT routed in `router.go` (the handler method exists at `wallet_handler.go:33` but is unmounted), so the app's top-up screen currently has no working backend endpoint (audit appendix lists `/v1/wallets/topup` as called by mobile and missing). The engine service `TOPUP_BANK` (wallet strategy `syscash_to_user`) is the supported path through `/v1/transactions/request`. The callback route is mounted in every environment (audit BE-H7).

## 5. Nightly reconciliation

```mermaid
sequenceDiagram
  participant C as recon.Cron (goroutine)
  participant R as recon.Service.RunForDate
  participant DB as PostgreSQL
  participant AI as AI advisor
  participant ST as Storage
  participant Ad as Admin web
  C->>C: sleep until next 00:15 (server local time)
  C->>R: RunForDate(yesterday)
  R->>DB: completed top-ups of the day; statements by ref_code (any date); statements booked that day
  R->>R: classify: matched, cutoff_shift, duplicate, amount_mismatch, missing_internal ...
  R->>DB: ListWalletLedgerMismatches: wallets with balance != SUM(ledger) added as internal_drift
  R->>AI: summarize (advisor, may be mock)
  R->>ST: write xlsx report (excelize), signed URL
  R->>DB: INSERT recon_runs (unique run_date -> ErrAlreadyRan), INSERT recon_items
  Ad->>DB: POST /v1/admin/recon/run (manual) / GET /v1/admin/recon/runs/:date
```

Verified: `recon/cron.go:22-50`, `recon/recon.go:53-127`, routes `router.go:196-197`. Timezone: `time.Now()` local to the server process; with the distroless image the default is UTC, so "00:15" is 07:15 in Vietnam unless TZ is set (see crosscutting, timezone).
