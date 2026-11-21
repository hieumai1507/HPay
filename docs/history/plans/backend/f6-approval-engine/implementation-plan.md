[← F5](../f5-p2p-fee/implementation-plan.md) · f6-approval-engine · [solution-architecture →](solution-architecture.md)

# F6 Approval Engine — Implementation Plan

## 1. Scope & non-goals

> **Advantage:** the approval schema has been generalized since F1 (migration 000005: `approval_requests` has `subject_type` covering all 4 kinds, `approval_actions` has UNIQUE(request_id, checker_id), `approval_policies` is versioned, plus `otp_challenges`). F6 does NOT need to change the schema — only seed a definition and write code. The F2 OTP service (`auth.RequestOTP/VerifyOTP`, purpose-scoped) is reused as-is.

| Ships | Not done in F6 |
| --- | --- |
| Org API: create org (with the org wallet created atomically, creator = org admin), add member, create/change policy (new version + deactivate the old one); org topup via mock bank (so it can be demoed); maker creates an `org_disbursement` order through the engine; `org_disbursement` definition v1 (VALIDATE → FRAUD_CHECK → WAIT_APPROVAL → MOVE_FUNDS → CONFIRM); engine pause-hook: on reaching `human_approval` → create `approval_requests` (policy matched by org+amount, tiers snapshotted) + txn → `pending_approval` in the SAME tx; 3-step checker API (inbox "My Tasks" / OTP challenge / decide with FOR UPDATE + pure `Evaluate`); resume via outbox ResultEvent for the WAIT_APPROVAL step (with a step_execution so the timeline is complete); expiry sweeper (claim-lease UPDATE…RETURNING, E10); validator/wallet handlers support org wallets (generic owner-status); audit_logs for decide + policy change; executor registry keyed by `subject_type` (F6 registers `org_transaction`); E08/E09/E10 tests; openapi.yaml in the same commit | Executors for `reversal` / `account_lock` / `policy_change` (the framework accepts these subjects but the APIs that create such requests come in F9 together with fraud/recon); admin_users auth + `/v1/admin/*` My Tasks admin screen (A-phases — F6 serves checkers who are org USERS via mobile business, per doc 03/M5); notifications for maker/checker (F7 notify); org daily limit |

## 2. Task breakdown

| # | Task | Files | Size | Depends |
| --- | --- | --- | --- | --- |
| T1 | Migration 000013: seed definition `org_disbursement` v1 (WAIT_APPROVAL: `expires_h=24`, on_failure=fail; MOVE_FUNDS compensatable) + fee policy `org_disbursement` 0 VND (explicit, to avoid an implicit fallback) | db/migrations/000013_* | S | — |
| T2 | Queries: org.sql (org CRUD, members, RequireRole lookup), approval.sql (insert request, GetForUpdate, insert action, guarded finalize, inbox by checker+level, ExpireDue claim-lease, policy match by org+amount+active), wallets.sql adds `GetOwnerStatusByWalletID` (user OR org — fixes the org-wallet gap) + CreateOrgWallet; transactions.sql adds `PendTransactionGuarded` (processing → pending_approval) | internal/store/queries/* | M | T1 |
| T3 | `internal/approval`: pure `Evaluate(snapshot, actions)` + unit tests (reject-first, ascending tiers, current_level); types PolicySnapshot/Decision | internal/approval/evaluate*.go | S | — |
| T4 | Engine: `PauseHook` interface + `SetPauseHook` (advance calls the hook WITHIN the same tx q when it reaches human_approval); OnResult guard accepts `waiting_approval` when the step is human_approval; stuck sweeper skips waiting_approval (already correct — it only scans running) | internal/workflow/engine.go, types.go | M | T2 |
| T5 | `internal/approval` service: CreateForInstance (pause-hook impl: match policy, snapshot, txn→pending_approval); Challenge (eligibility: correct org, checker role, level = current_level, has not acted, request pending → OTP purpose `approve:{requestID}`); Decide (verify OTP → tx: FOR UPDATE request, guard pending + not expired, insert action (23505 → ERR_ALREADY_ACTED), Evaluate → finalize + WAIT_APPROVAL step_execution + outbox ResultEvent + audit_log); Expiry sweeper loop (worker role) | internal/approval/service.go, sweeper.go | L | T3,T4 |
| T6 | `internal/org` service: CreateOrg (+org wallet, creator admin, 1 tx), AddMember, CreatePolicy/NewVersion (audit), OrgTopup (mock bank, idempotency like user topup), CreateDisbursement (maker: policy exists → fail fast ERR_NO_POLICY; txn type org_disbursement sender=org wallet + engine.Start in the same tx — reuses the transfer pattern) | internal/org/service.go | L | T2 |
| T7 | Org-wallet handlers: validator + wallet switch to `GetOwnerStatusByWalletID` (an org suspended midway also fails like E12); API layer: `RequireOrgRole` middleware, org_handler + approval_handler, routes `/v1/org/*` (mounted per MountsAdminAPI — role admin|all), new errmap, openapi.yaml | internal/workflow/handlers/, internal/api/* | L | T5,T6 |
| T8 | Tests: E08/E09/E10 + 2-tier E2E + reject path + eligibility/authz; wire main.go (pause hook, sweeper); CI REQUIRED_EDGE_IDS += E08 E09 E10 | *_test.go, cmd/server, ci | L | T7 |

## 3. Schema changes

No new tables/columns. 000013 is only a **seed data migration** (definition + fee policy) — its down removes exactly 2 rows by name/version. All approval tables have existed since 000005; if a missing column is discovered while coding → stop, go back and propose a docs fix first (docs-first).

## 4. API changes (into openapi.yaml in the same commit)

| Endpoint | Role | Description / Errors |
| --- | --- | --- |
| `POST /v1/org` | user | {name} → org + org wallet; creator becomes org admin. ERR_VALIDATION |
| `POST /v1/org/{id}/members` | org admin | {phone, role, checker_level} → ERR_NOT_ORG_MEMBER, ERR_FORBIDDEN_ROLE, ERR_USER_NOT_FOUND, ERR_MEMBER_EXISTS |
| `POST /v1/org/{id}/policies` | org admin | {name, min_amount, max_amount, tiers[]} → new version, deactivates the old one in the same range. ERR_VALIDATION |
| `POST /v1/org/{id}/topup` | org admin | Idempotency-Key; mock bank like user topup. ERR_LIMIT_EXCEEDED |
| `POST /v1/org/{id}/transactions` | org maker | {receiver_phone, amount, note} + Idempotency-Key → 202 {ref_code, status=pending after reaching WAIT_APPROVAL}; ERR_NO_POLICY, ERR_RECEIVER_NOT_FOUND, ERR_INSUFFICIENT_BALANCE (soft) |
| `GET /v1/org/approvals?status=pending` | checker | "My Tasks": pending requests in my org, whose current tier matches my level, and that I have not acted on |
| `POST /v1/org/approvals/{id}/challenge` | checker | Eligibility is checked first, then OTP is issued (dev: returns the code). ERR_APPROVAL_NOT_FOUND, ERR_ALREADY_FINALIZED, ERR_WRONG_LEVEL, ERR_ALREADY_ACTED, ERR_OTP_COOLDOWN |
| `POST /v1/org/approvals/{id}/decide` | checker | {action: approve\|reject, otp} → updates level or finalizes; enough signatures → workflow resumes. ERR_OTP_INVALID/EXPIRED/MAX_ATTEMPTS, ERR_ALREADY_FINALIZED, ERR_ALREADY_ACTED, ERR_WRONG_LEVEL, ERR_APPROVAL_EXPIRED |

## 5. Test plan

| ID | Test |
| --- | --- |
| E08 | `TestEdge_E08_CheckerRaceSingleFinalize`: the tier needs 1 approval, 2 checkers decide concurrently (2 goroutines, separate OTPs) → exactly 1 action finalizes + the workflow resumes exactly once (1 ResultEvent), the later one gets ERR_ALREADY_FINALIZED; approval_actions has exactly 1 row |
| E09 | `TestEdge_E09_PolicySnapshotSurvivesPolicyChange`: create an order under policy v1 (1 signature) → org admin changes the policy to 2 signatures → checker approves → still finalizes per the v1 snapshot; a NEW order created afterwards needs 2 signatures |
| E10 | `TestEdge_E10_ExpiredApprovalFailsClean`: request past `expires_at` (set in the past) → sweeper runs → request expired, instance failed, txn failed, ledger has NO entries; 2 concurrent sweepers do not double-claim (lease) |
| — | 2-tier E2E: maker disburses 5M (policy: L1×1 + L2×1) → txn pending_approval + L1 inbox sees it, L2 does not yet; L1 approves → current_level=2; L2 approves → resume → money leaves the org wallet, txn completed, timeline has WAIT_APPROVAL; reject path: L1 rejects → txn failed, no ledger entries; eligibility: maker approving their own order → 403; checker of another org → 404; wrong level → ERR_WRONG_LEVEL; Evaluate unit table-test; org topup idempotent (E01 pattern) |

## 6. Rollout & risk

- **Resume = ResultEvent for WAIT_APPROVAL** (no new event type): engine OnResult must loosen its guard to accept `waiting_approval` — only when the step in the definition is human_approval, kept strict for auto steps. A duplicate finalize event is harmless thanks to the existing guard (E06).

- Pause hook failure (no policy matches when the engine runs — rare because the maker already fails fast) → the instance fails cleanly via on_failure, not left hanging.

- `GetOwnerStatusByWalletID` replaces `GetUserStatusByWalletID` in the handlers — rerun the entire F5 test suite to make sure P2P does not break.

- Routes /v1/org are mounted at role `admin|all` (per F1 config). Mobile business calls through them; dev --role=all changes nothing.

- Docs 00 §5.1 describes a `transaction_approvals` table — in reality it is already `approval_requests` (doc 04 §2.3 supersedes it). No further drift; add a note reminding to update docs 00 to match 04 after merge.

## 7. Execution checklist

- [x] gen-feature-plan · [ ] T1 (gen-migration) → T2 (sqlc) → T3→T8 (gen-workflow-step conventions for the engine hook, gen-api-test for E08–E10) · [ ] fintech-review · [ ] commit `feat: F6 approval engine - org maker-checker, policy snapshot, human gate resume, expiry sweeper`
