[← F8](../f8-kyc/implementation-plan.md) · f9-fraud-recon-reversal · [solution-architecture →](solution-architecture.md)

# F9 Fraud + Recon + Reversal — Implementation Plan

## 1. Scope & non-goals

> **3 independent areas, 1 shared principle** (index.html §9): hard decisions about money are always **deterministic rules + humans** — the LLM is advisory only (explaining fraud, summarizing recon), with a mock fallback when the key is missing. Reversal follows TMDS (doc 04 §1): **passive reversing entry, does NOT go through the workflow engine**, maker-checker between 2 admins is mandatory — the executor registers into the F6 approval framework via the `RegisterExecutor` hook that was left ready for it.

| Ships | Not done in F9 |
| --- | --- |
| **Fraud:** deterministic rule engine replacing FraudStub in the FRAUD_CHECK step (VELOCITY_5MIN · AMOUNT_SPIKE vs the 30-day average · ODD_HOURS 0–5h · NEW_RECEIVER); int score accumulated, capped at 100; **>70 → block** (terminal, txn fails via the existing on_failure) + flag; **40–70 → flag open + LLM advisory in Vietnamese** (best-effort, an LLM error does not block the transaction) + txn proceeds; <40 pass. Admin API: list flags + resolve (clear\|confirm_fraud, guarded open→…, audit). **Recon:** RunForDate — matches completed topups against bank_statements **by global ref_code, NOT filtered by date** (E16: a statement falling on the adjacent day is still matched, detail marked cutoff_shift; a statement with a duplicate ref → detail duplicate, half of E15); classifies matched/amount_mismatch/missing_internal/missing_external; internal recon SUM(ledger)=balance per wallet; xlsx export (excelize) stored via storage + signed URL; AI summary attached to the run; cron 00:15 (ticker, run_date UNIQUE guard); admin API run/get/export. **Reversal:** admin A creates a request (subject=reversal, snapshot of 1 signature, policy NULL) → admin B ≠ A decides (admin My Tasks) → executor INSIDE the finalize tx: new txn type=reversal + reversal_of, mirror entries from the txn row (exactly how F5 compensate does it, tested in E07) via `ledger.Post`, original completed→reversed guarded; double reversal blocked on 2 layers (UNIQUE subject + one_reversal_per_txn) | Account lock via approval (the subject is already in the enum, executor left to A-phases/F10 — resolving fraud does NOT automatically lock the account); OTP for admin decide (OTP infra has FK→users, admin is already logged in with aud=admin — see Risks); real bank callback endpoint (full E15 test in F10); NEW_DEVICE rule (transactions do not store device_id — replaced by NEW_RECEIVER, which is also in the docs 00 §8.2 list); notification |

## 2. Task breakdown

| # | Task | Files | Size | Depends |
| --- | --- | --- | --- | --- |
| T1 | Migration 000016: (a) DROP FK `approval_requests.created_by` + `approval_actions.checker_id` → users — the actor is polymorphic by subject_type (user for org_transaction, ADMIN for reversal/account_lock/policy_change; same class of bug as the reviewer_id already fixed in 000015, docs 00 will get a note); (b) `ALTER reconciliation_runs ADD ai_summary TEXT` (the docs say "AI summary attached to the report" but the original schema lacks the column) | db/migrations/000016_* | S | — |
| T2 | Queries: fraud.sql (CountSenderTxnsSince, AvgSenderAmount30d, HasSentToReceiver, InsertFraudFlag, ListFraudFlags by status, ResolveFraudFlagGuarded WHERE status='open'); recon.sql (ListCompletedTopupsByDate, FindStatementsByRefCodes, InsertReconRun/Item, GetReconRunByDate, ListReconItems, ListWalletBalanceMismatches SUM-ledger-vs-balance); transactions.sql (InsertReversalTransaction with reversal_of + status completed directly — passive posting without the engine, MarkTransactionReversed guarded completed→reversed); approval.sql (InsertApprovalRequest: org_id/policy_id changed to NULLIF-nullable so reversal passes Nil; ListPendingAdminRequests for subject≠org_transaction with txn info) | internal/store/queries/* | L | T1 |
| T3 | `internal/ai`: Advisor interface {Complete(ctx, system, user)} — claude text (net/http, 15s timeout, F8 scorer_claude style) + mock (fixed template); selected by ANTHROPIC_API_KEY | internal/ai/* | S | — |
| T4 | `internal/fraud`: pure rules.go (each rule returns {code, points, evidence}; Score = sum capped at 100) + unit tests at the 40/70 boundaries; handler.go replacing FraudStub (reads history via queries, 3-branch decision, flag + advisory); RegisterAll drops FraudStub → fraud.RegisterHandler(reg, pool, advisor); admin list/resolve service | internal/fraud/*, internal/workflow/handlers | L | T2,T3 |
| T5 | `internal/recon`: RunForDate (E16 match + duplicate detect + internal ledger check + insert run/items in 1 tx, run_date UNIQUE = idempotent); xlsx.go (excelize → storage key `recon/{date}.xlsx`); summary via Advisor; cron.go ticker (00:15, RunForDate for yesterday); admin service | internal/recon/* | L | T2,T3 |
| T6 | Reversal: `internal/reversal` — CreateRequest (admin maker: txn completed? not yet reversed? → approval_requests subject=reversal, snapshot {L1×1}, expires 24h); Executor (registered via `approval.RegisterExecutor(reversal)`: runs INSIDE the finalize tx — InsertReversalTransaction + ledger.Post mirror legs from the txn row [sender +amount+fee reversed, SYSTEM_FEE refunded] + MarkTransactionReversed + audit); approval.AdminDecide (reuses F6 Evaluate/finalize/execs, NO OTP, guards **maker ≠ checker** + actor is an admin) | internal/reversal/*, internal/approval | L | T2 |
| T7 | Admin API + wiring: GET/POST fraud-flags[+/:id/resolve]; POST reversals; GET admin/approvals (My Tasks) + POST admin/approvals/:id/decide; POST recon/run {date} + GET recon/runs/:date + export (signed URL); refactor the route file into `GET /v1/files/*key` (Signer.SignURL changes base — serves both kyc and recon reports); errmap; openapi 0.9.0; main.go (advisor, recon cron, reversal executor, fraud handler) | internal/api/*, cmd/server, api/openapi.yaml | L | T4,T5,T6 |
| T8 | Tests: `TestEdge_E16_ReconCutoffNoFalseMismatch` (txn 23:59 on day D, statement on day D+1 → run D and D+1 both report NO false mismatch, detail cutoff_shift); fraud: unit rules + integration 3 branches (pass/flag+advisory-mock/block → txn failed, no ledger entry); recon: full run with 4 classifications + duplicate statement + xlsx file exists + run idempotent per day; reversal E2E: A creates → B decides → money refunded correctly (including fee), original reversed, ledger net 0; A deciding own request is blocked; creating a 2nd request → 409; a 2nd reversal transaction is blocked by unique; CI += E16 | test/integration/*, ci.yml | L | T7 |

## 3. Schema changes

Migration 000016 — 2 items, both consequences of already-settled design rather than new features: (1) drop 2 FKs → users on the approval tables (polymorphic actor — **docs 00 §3.4 needs a note after merge**, like the reviewer_id round); (2) add `reconciliation_runs.ai_summary TEXT` (docs 00 §8.2 describes the feature but §3.5 lacks the column — **the docs also need this column added**). Down: re-add the FKs (with a comment: fails if admin-actor data already exists — accepted, structurally reversible).

## 4. API changes (openapi 0.9.0, same commit)

| Endpoint | Description | Main errors |
| --- | --- | --- |
| `GET /v1/admin/fraud-flags?status=open\|cleared\|confirmed_fraud` | Fraud queue (default open), with reasons (rule evidence + LLM advisory) | 401 |
| `POST /v1/admin/fraud-flags/{id}/resolve` | {action: clear\|confirm_fraud, reason} — guarded open→…, audit | ERR_FLAG_NOT_FOUND, ERR_ALREADY_RESOLVED (409) |
| `POST /v1/admin/reversals` | {ref_code, reason} — admin maker creates a reversal request | ERR_TXN_NOT_FOUND, ERR_TXN_NOT_REVERSIBLE (not completed), ERR_ALREADY_REVERSED, ERR_REVERSAL_PENDING (409 — a request already exists) |
| `GET /v1/admin/approvals?status=pending` | Admin My Tasks — requests with subject ≠ org_transaction | 401 |
| `POST /v1/admin/approvals/{id}/decide` | {action: approve\|reject, reason} — admin ≠ creator; once enough signatures are collected the executor runs immediately inside the finalize tx | ERR_APPROVAL_NOT_FOUND, ERR_SELF_APPROVAL (403), ERR_ALREADY_FINALIZED, ERR_ALREADY_ACTED |
| `POST /v1/admin/recon/run` | {date: YYYY-MM-DD} — manual run (demo); the cron runs automatically at 00:15 for yesterday | ERR_RECON_ALREADY_RAN (409, run_date UNIQUE) |
| `GET /v1/admin/recon/runs/{date}` | Run + items + ai_summary + report_url (signed) | ERR_RECON_NOT_FOUND |
| `GET /v1/files/{key}` | Replaces the route /v1/files/kyc/{key} — serves both KYC images and the recon xlsx (HMAC as before) | 403/404/410 as in F8 |

## 5. Test plan

| ID | Test |
| --- | --- |
| E16 | `TestEdge_E16_ReconCutoffNoFalseMismatch`: topup completed at 23:59:30 on day D, bank statement carries stmt_date D+1 with the same ref_code → run(D) matched (detail cutoff_shift=true), run(D+1) does not double-count; NO missing item for that ref |
| — | Fraud unit: each rule gives the right points; boundary totals 39/40/70/71 → pass/flag/flag/block. Integration: (a) normal transfer → no flag; (b) a chain of 4 transfers within 5' + amount spike → flag open + advisory mock text in reasons, txn still completed; (c) score >70 (large spike + night + new receiver + velocity) → txn failed, no ledger entry, flag open |
| — | Recon: 1 matched + 1 amount_mismatch + 1 missing_internal + 1 missing_external + 1 duplicate statement → counts correct, xlsx exists in storage, ai_summary (mock) non-empty, re-run on the same day → ERR_RECON_ALREADY_RAN |
| — | Reversal: E2E refunds both amount+fee (sender returns to original state, SYSTEM_FEE is deducted back), original→reversed, new txn type=reversal completed, ledger net 0 across all 3 wallets; self-approve → ERR_SELF_APPROVAL; 2nd request for the same txn → 409; reject → original txn REMAINS completed |

## 6. Rollout & risk

> **2 deviations that need your nod when approving the plan:** (1) **Admin decide without OTP** — doc 00 §5 describes OTP for the checker, but otp_challenges has FK→users; the admin is already authenticated with aud=admin + argon2id password, and Maker≠Checker is still enforced. If real admin OTP is wanted, a migration relaxing the FK is needed — proposal: let the A-phases decide when building the UI. (2) **Rule NEW_DEVICE → NEW_RECEIVER** — transactions do not store device_id; NEW_RECEIVER is in the same docs §8.2 list so it still matches the design, it just picks 4 of 5 rules that have data.

- Fraud block (>70) uses the EXISTING on_failure=fail path of the p2p/org definitions — no definition change, no engine migration.
- LLM advisory synchronous in the handler but with a 15s timeout < step timeout of 3s? — NO: FRAUD_CHECK has timeout_ms=3000 in the definition! The advisory must run AFTER the decision, as a fire-and-forget goroutine with its own context (does not hold the step), with the flag reasons updated asynchronously. Stated explicitly in solution-architecture.
- Excelize is a new dependency (~a few MB) — imported only in internal/recon, does not affect the hot binary path.
- The /v1/files route changes shape — only the admin SPA uses it (not built yet), so it is safe to change; openapi updated in the same commit.
- The reversal executor runs inside the decide finalize tx — preserves F6's invariant "resume/effect commit together with the decision"; ledger.Post lock ordering takes care of deadlocks.

## 7. Execution checklist

- [x] gen-feature-plan · [ ] T1 (gen-migration) → T2 (sqlc) → T3→T8 · [ ] fintech-review · [ ] commit `feat: F9 fraud rules + llm advisory, reconciliation with xlsx, admin reversal via approval`
