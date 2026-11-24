# HPay â€“ Bug-fix and Monorepo Consolidation Plan

Date: 2026-11-21. Basis: `docs/audit/AUDIT-REPORT.md` (including the verification appendix).
Status: **in progress**. Completed so far: monorepo merge (src/ + docs/), git history reset, skills consolidated at `.claude/skills` with workflows `hpay-feature` / `hpay-bugfix` / `hpay-review`, subagents in `.claude/agents`, Docker-based backend test runner (`scripts/test-backend.ps1`; baseline in `docs/handoff/baseline.md`: 160 Go tests pass with `-race`, mobile jest 80 pass, eslint has pre-existing errors in admin (6) and mobile (3)). Docs conversion to English Markdown is in progress. Not started: Wave 1 onward.

All documentation must be in English.

## 0. Decisions to settle before running

Each item has a default I propose. If you say nothing, the plan uses the default. User answers received so far are recorded in the "Decision / status" column.

| # | Question | Proposed default | Decision / status |
|---|---|---|---|
| D1 | Monorepo structure | `D:\hpay` is a single git repo, keeping the 3 directories `src/backend/`, `src/admin/`, `src/mobile/` and `docs/` (md). No submodules | **Decided: monorepo (done)** |
| D2 | Old git history of the 3-4 sub-repos | Not kept. Initialize a new repo, first commit is a snapshot with secrets cleaned up. If history must be kept, use `git subtree add` (more effort) | **Decided: history dropped entirely (user decision)** |
| D3 | Transaction PIN | Implement for real: PIN hashed with bcrypt/argon2 on the server, set/change PIN endpoint, lock after 5 wrong attempts | **Decided: real server-side PIN** |
| D4 | Idempotency key | UNIQUE `(initiated_by, idempotency_key)` plus `request_hash`; same key with a different payload returns 409 | Default |
| D5 | Docs about other products | Move into `docs/reference/` and label "reference, not an HPay specification". Remove any internal repo name of another product | Default |
| D6 | Mock data in admin | Enabled only when `import.meta.env.DEV`; production shows real errors | Default |
| D7 | Canonical KYC limits | You must pick a single table (currently 4 sources disagree). See section 5.2 | **Decided: take the seed values in code (L1 = 50M, L2 = 200M VND)** |
| D8 | Docs say asynq but code uses goroutines | Default: fix the docs to match the code (goroutine + lease). Add asynq only if you want to run multiple replicas | Default |
| D9 | `docs/reference/external/` (external wallet docs, `.docx`) | Keep out of the repo (not committed), or place in `docs/reference/` if you have the right to share | Default |
| D10 | Reversal when the receiver has already spent the money (see Wave 2) | Create a debt (receivable), hold the balance, or allow refund only when funds suffice and report manually | **Decided: reversal failure -> `needs_manual` status + audit; no negative balances** |
| D11 | Fraud/LLM failure | Block the transaction (fail-closed) or let it pass and flag it (fail-open) | **Decided: fail-closed (hold for review)** |

---

## 1. Operating principles

1. **Each agent owns its own set of files** (table in section 4). Two agents never edit the same file in the same wave. If a file owned by another agent needs changes, write it into `docs/handoff/<wave>.md` for a later wave to handle.
2. **Each agent works in its own git worktree**, branch `fix/<id>`. You or I merge in wave order.
3. **Test first, fix second** (red -> green) for every money and security bug. Evidence is test command output, not an assertion.
4. **Every PR must pass the gate**: `go vet`, `golangci-lint`, `go test ./... -race`, `tsc --noEmit`, `eslint`, frontend tests, and the `fintech-review` skill.
5. **The repo's CLAUDE.md requires GitNexus**: run `impact` before editing a symbol, run `detect_changes` before committing. After the monorepo merge, the index must be re-run (`node .gitnexus/run.cjs analyze`).
6. **No agent may commit secrets.** Scan with `gitleaks` before every merge.

---

## 2. Wave 0 â€“ Foundation (do first, sequentially, by you or 1 agent)

| ID | Task | Done when |
|---|---|---|
| W0-1 | **Rotate keys** for Neon, Cloudinary, JWT. Only you can do this (dashboard login). Set `JWT_SECRET` to a random value >= 32 bytes | New `.env`, old keys invalidated |
| W0-2 | Write a root `.gitignore`: `.env*` (except `.env.example`), `data/`, `e2e/.auth/`, `*.exe`, `tmp/`, `node_modules/`, `dist/`, `.expo/`, `playwright-report/`, `test-results/`, `__pycache__/`, `.gotmp/`, `bin/` | `git status` lists no sensitive files |
| W0-3 | Clean up stray files: `src/backend/*.exe`, `tmp/go-build*`, `scripts/__pycache__`, `e2e/.auth/sessions.json`, `docs/push-to-github.ps1` | Deletion list approved by you beforehand |
| W0-4 | `git init`, run `gitleaks detect`, commit baseline | gitleaks clean |
| W0-5 | Install the new skill set (section 3) into `.claude/skills/` at the root | `/skills` lists them all |
| W0-6 | Run baseline: `go test ./... -race`, admin tests, mobile tests, record pass/fail counts **before fixing** in `docs/handoff/baseline.md` | Numbers available for later comparison |

You confirm W0-3 because it is a deletion.

---

## 3. New skill set (cloned at `D:\hpay-skills-research`, all MIT)

A pipeline from a vague requirement to a test-fix loop, assembled from several sources because no single set covers the whole chain:

| Step | Skill | Source | Integration notes |
|---|---|---|---|
| 1. Clarify requirement | `interview-me`, `idea-refine` | addyosmani/agent-skills | `interview-me` needs the user to answer directly, **do not run in a background agent** |
| 2. Spec and scope | `spec-driven-development` + `spec-template.md` template, `/speckit.clarify` | agent-skills, spec-kit | Output as **`.md`** in `docs/specs/<slug>/spec.md` |
| 3. Plan and tasks | `planning-and-task-breakdown`, `writing-plans`, `/speckit.analyze` | agent-skills, superpowers, spec-kit | Replaces `gen-feature-plan` (which forces HTML output) |
| 4. Design critique | `doubt-driven-development` | agent-skills | Run with fresh context for money/security changes |
| 5. Parallel execution | `subagent-driven-development`, `dispatching-parallel-agents`, `using-git-worktrees` | superpowers | Matches the wave split in section 4 |
| 6. TDD implementation | `test-driven-development`, `incremental-implementation` | superpowers, agent-skills | `TestEdge_<ID>_*` tests per test case |
| 7. Fix loop | `systematic-debugging`, `debugging-and-error-recovery` | superpowers, agent-skills | Rule: no fix without a root cause |
| 8. Verification | `verification-before-completion`, `/speckit.converge` | superpowers, spec-kit | Must not report done without pasting test command output |
| 9. Review and security | `code-review-and-quality`, `security-and-hardening`, `fintech-review` (kept, upgraded) | agent-skills, yours | |
| 10. E2E | `browser-testing-with-devtools`, ideas from `bmad-qa-generate-e2e-tests` | agent-skills, BMAD | Do not install all of BMAD (needs `uv` and a `_bmad/` directory, heavy) |

**Your skills kept and upgraded:**
- `fintech-review`: add checks: idempotency per user + request hash, ledger does not double-post, admin RBAC, no `Number()` for amounts, default secrets per env.
- `gen-migration`, `gen-workflow-step`, `scaffold-module`, `gen-api-test`, `seed-demo-data`: keep, update paths and remove mentions of the "HTML plan".

**Skills to rewrite:**
- `gen-feature-plan`: output `.md`, no longer "BOTH html" mandatory, point to `docs/`.
- Merge `docs/skills/` and `src/backend/.claude/skills/` into a single source (7/7 hashes differ, contents not yet compared, needs a diff first).

**Integration risks to handle:**
- Superpowers forces a skill call before every response (`using-superpowers`). Install only the skills needed, drop `using-superpowers` or adjust it so it does not conflict with CLAUDE.md.
- There are two overlapping TDD skills (superpowers and agent-skills) and two debug skills. Keep only one of each (proposal: superpowers for TDD and debug).
- Every third-party skill must have its content read before being enabled (no blind installs), and the source commit hash recorded in `.claude/skills/SOURCES.md`.

---

## 4. Bug-fix waves

Notation: **P0** = do before going live. A file owned by an agent may only be edited by that agent in that wave.

### Wave 1 â€“ DB and money foundation (P0, 3 agents in parallel)

**Agent A1 â€“ Migration 000014 (constraints)**
- Owns: `src/backend/db/migrations/000014_*`, `internal/store/queries/*.sql`, `internal/store/gen/` (sqlc generate).
- Tasks:
  1. UNIQUE `(initiated_by, idempotency_key)`, add a `request_hash` column, drop the global UNIQUE. Check existing data before the change.
  2. `ledger_entries`: UNIQUE to prevent duplicate posting by `(transaction_id, posting_kind, wallet_id/account_id, direction)`, CHECK `wallet_id IS NOT NULL OR account_id IS NOT NULL`. Add a `posting_kind` column (forward/compensate).
  3. DEFERRABLE constraint trigger checking total debit = total credit per `transaction_id`.
  4. Partial UNIQUE `transactions(reversal_of) WHERE reversal_of IS NOT NULL`; partial UNIQUE `approval_requests(subject_type, subject_id) WHERE status='pending'`.
  5. `transactions`: CHECK `sender_wallet_id <> receiver_wallet_id`. `fee_policies`: CHECK (`fixed_fee>=0`, `percent_bp BETWEEN 0 AND 10000`, `min_fee<=max_fee`) and UNIQUE `(txn_type, kyc_status, version)`. `wallets.currency` CHECK.
  6. `wf_status` gains `compensation_failed`. Offerguard index `(initiated_by, type, created_at)`. `outbox` gains `attempts`, `last_error`, `dead_at`, and an index on `id`.
  7. Block TRUNCATE on `ledger_entries` and `REVOKE` excess DB privileges from the app role.
- Skill: `gen-migration`. Include a down file. Run up/down/up on a clean DB.
- Done when: migration runs cleanly, `sqlc generate` compiles, existing tests still pass.

**Agent A2 â€“ Configuration and baseline security**
- Owns: `internal/platform/config/`, `cmd/server/main.go`, `cmd/seed/main.go`, `internal/api/router.go` (mount section), `docker-compose.yml`, `.env.example`.
- Tasks:
  1. `APP_ENV` is mandatory, whitelist `dev|ci|demo|production`, unify every comparison. Require `JWT_SECRET >= 32` and a separate `MOCK_BANK_SECRET` when not dev. `godotenv.Load()` only in dev.
  2. Gateway: choose real HTTP or Mock by explicit configuration, not by the ambiguous `"production"` string.
  3. Mock-bank callback is mounted only when not production. Add rate limiting and an optional IP allowlist.
  4. Return `otp_dev` only when `Env == "dev"`.
  5. `SetTrustedProxies`, add Read/Write/Idle timeouts and `MaxHeaderBytes` to the HTTP server, `healthz` does not leak error details, add security headers (nosniff, HSTS) and a CORS allowlist.
  6. Seed: refuse to run if env is not dev/ci/demo or the DB host is not localhost (needs an explicit force flag). Seed passwords are randomly generated and printed once, or taken from env.
  7. `docker-compose.yml`: Redis has a password, bind to `127.0.0.1`.
- Done when: config tests for each env, server refuses to start with a wrong config.

**Agent A3 â€“ Repo consolidation (monorepo)**
- Owns: root (`.gitignore`, `README.md`, top-level `Makefile`, `.github/`), does not touch application code.
- Tasks: carry out W0-2 to W0-5; a single CI that runs backend, admin, mobile; `gitleaks` in CI; update paths in the old workflows; re-run the GitNexus index.
- Done when: CI is green on an empty branch.

### Wave 2 â€“ Fix money logic (P0, sequential by dependency, 2 agents)

Wait for Wave 1 to merge (A1 must finish before B1).

**Agent B1 â€“ Ledger, workflow, idempotency**
- Owns: `internal/ledger/`, `internal/workflow/` (engine, consumer, handlers, recovery), `internal/outbox/`, `internal/transfer/`, `internal/wallet/`, `internal/moneyrequest/`, `internal/api/middleware.go` (idempotency part).
- Tasks in order (each: write a failing test first):
  1. **BE-C1**: record `step_executions` (claim `running`) and post to the ledger safely on replay. Use A1's `posting_kind` and UNIQUE to prevent duplicate posting. Tests: call `ledger.Post` twice with the same `txnID`; simulate a crash between Commit and `recordAndEmit` and then replay.
  2. **BE-C2 (engine part)**: `request` stops at the AUTH step and only continues after a valid confirm. State model `request â†’ confirm â†’ verify` with an expiry, confirming twice is safe, fee locked at `request`.
  3. **BE-H1**: idempotency per user + `request_hash`, 409 on a different payload, mandatory key on every money-writing endpoint (add `RequireIdempotencyKey` for `/money-requests`, `/split`, `/vaults`, round-up). Remove the auto-generated UUID when the key is missing.
  4. **BE-H5**: money request stays `pending`/`paying` until the transaction is `completed`, and reverts on failure.
  5. Compensation: use `compensation_failed`, call `FailTransactionGuarded`, let the recovery sweep scan `compensating`, support retry (remove the fixed Attempt 101).
  6. Outbox: dead-letter after N attempts, `PumpLocal` blocked in production, consumer uses `select ctx.Done()`.
  7. Ledger: check wallet `status` and `currency`, guard against int64 overflow, cap amount at every input.
- Done when: all new tests pass with `-race`, existing tests do not break, `fintech-review` PASS.

**Agent B2 â€“ Offerguard, fee, reversal, approval**
- Owns: `internal/offerguard/`, `internal/fee/`, `internal/reversal/`, `internal/approval/`, `internal/org/`, `internal/vault/`, `internal/qr/`, `internal/recon/`, `internal/fraud/`.
- Tasks:
  1. **BE-H2**: `pg_advisory_xact_lock` in the same tx as the INSERT; fix the timezone to Asia/Ho_Chi_Minh.
  2. **BE-H3, H4**: reversal locks the original transaction `FOR UPDATE`, requires `status='completed'`, checks the row count, `reversal_failed`/`needs_manual` status when the receiver is out of funds. A business decision (D10 below) is needed before writing this.
  3. Self-approval: add `checkerID != req.CreatedBy` to `eligibility()`. Re-check balance and limits at the MOVE_FUNDS step after approval.
  4. Fee: settle rounding, block `fee > amount`, do not swallow negative values.
  5. `vault`: truncate label by rune, validate `target`. `qr`: token is single-use or explicitly documented as reusable.
  6. `recon`: check `ErrAlreadyRan` before running the LLM; lease for the cron when running multiple replicas.
  7. `fraud`: time in a fixed timezone, avoid overwriting `reasons`; settle behavior on LLM failure (fail-open or fail-closed, D11).
- Done when: offerguard race test (N goroutines), concurrent reversal, reversal when the receiver is out of funds.

> **D10** (decided): when the receiver has already spent the money, the reversal fails into a `needs_manual` status with an audit record; balances never go negative. **D11** (decided): fraud/LLM failure is fail-closed (the transaction is held for review).

### Wave 3 â€“ Authentication, authorization, KYC (P0, 2 agents in parallel)

**Agent C1 â€“ Auth, PIN, RBAC**
- Owns: `internal/auth/`, `internal/user/`, `internal/api/` (auth_handler, admin_handler, auth middleware, errmap), `internal/gateway/`.
- Tasks:
  1. **BE-C2 (authentication part)**: real PIN (D3): hash table/column, set/change PIN endpoint, verify in confirm, count wrong attempts and lock, check `initiated_by == user` (404 otherwise). Remove PIN/OTP fields that are accepted but unused. Add `ERR_*` to `errmap.go`.
  2. **BE-H6**: `RequireAuth` checks `users.status` (short-lived Redis cache), `jti` or token version for revocation. Admin gets refresh, status check on every request, RBAC by `admin_role` for each route (especially `/ai/*`, `/reversals`, `/services`, `/orgs/:id/policies`, KYC review). Separate secret for admin.
  3. **BE-H8**: lockout per account (phone) and `(phone, IP)` with backoff; run bcrypt on a dummy hash when the phone does not exist (prevent timing enumeration); rate limit for logout, files, admin; cap password length; refresh checks `users.status`; bind `device_id`.
  4. SSRF: validate the gateway URL (allowlist host/scheme `https`, block private/loopback/link-local/metadata IPs, disable or limit redirects, fix the method).
  5. Admin AI: encrypt `api_key` (AES-GCM, key from env), audit every key/setting operation, use `errors.Is(pgx.ErrNoRows)`, fix the `is_active` default, use `model_name` from the DB.
  6. Admin decide: audit includes IP, KYC review requires a reason on reject.
- Done when: IDOR tests, brute-force tests, per-route RBAC tests, SSRF tests with the blocked IP list.

**Agent C2 â€“ KYC and file storage**
- Owns: `internal/kyc/`, `internal/storage/`, `internal/ai/`.
- Tasks: **BE-H9**: `image.DecodeConfig` and a pixel limit, `http.MaxBytesReader`, write the correct extension, Cloudinary upload as `authenticated/private` with signed URLs, a separate signing key (not `JWT_SECRET`), `nosniff` and `Content-Disposition`, audit when a link is issued, clean up orphan files, network errors when fetching images are retryable. Settle the policy for sending national ID (CCCD) images to a third-party LLM (D12: with user consent, or send only extracted data).
- Done when: tests for forged files, polyglots, oversized images, expired links.

### Wave 4 â€“ Frontend (parallel with Wave 3, after Wave 2 for the contract)

**Agent F1 â€“ Contract and OpenAPI**
- Owns: `src/backend/api/openapi.yaml`, `src/admin/src/api/schema.d.ts` and `src/api/client.ts`, `src/mobile/src/api/`.
- Tasks: unify endpoints (the mismatch list is in the verification appendix): decide for each endpoint whether the backend adds it or the frontend is fixed; update the `workflow status` enum; regenerate `schema.d.ts`; add a CI step checking that openapi matches the real routes; compare transaction/money-request/KYC status enums with the DB (not yet done).

**Agent F2 â€“ Mobile**
- Owns: `src/mobile/`.
- Tasks: remove the PIN `'123456'` (a real PIN entry screen connected to the server, per C1); app lock uses real authentication with a limit on attempts; `useSubmitGuard` (ref flag + disabled button) for every money action, store the idempotency key per transaction id; remove `Number()/parseInt()` on amounts (use bigint, add a lint rule); refresh clears the session only on 401/403; `app.config.ts` changes env `dev` to localhost, add `bundleIdentifier` and `package`; check the KYC draft in SecureStore.

**Agent F3 â€“ Admin**
- Owns: `src/admin/`.
- Tasks: `RequirePermission` for sensitive routes and hide buttons by `can()`; separate mock from production (D6) and show real errors, show "â€”" when a number is missing; Idempotency-Key and disabled button on writes; remove `Number()` for amounts; CSP in `index.html`; handle non-JSON responses; delete `Placeholder.tsx` if confirmed unused; remove the hard-coded e2e password (read from env).

### Wave 5 â€“ Docs converted to Markdown (parallel, independent of code)

**Agent D1 â€“ Convert HTML â†’ MD**
- Owns: `docs/` â†’ `docs/` (root).
- Proposed structure:
  ```
  docs/
    README.md                    (table of contents)
    architecture/00..07-*.md
    guide/{functional,technical}-overview.{vi,en}.md
    planning/{master-plan,session-prompt}.md
    design/ui-design-spec.md
    design/mockups/              (keep HTML mockups as design assets; has README)
    qa/{plan,test-cases}/        (xlsx kept as is; .py scripts kept)
    specs/                       (per-feature spec and plan, from the new skills)
    handoff/                     (notes between agents)
    reference/                   (other products, labeled as reference)
    adr/
  ```
- Conventions: diagrams use ```mermaid in md; HTML mockups (28KB+ each) are **not** converted to md because they are design deliverables, only a README describing them. Must run without losing content: use `pandoc` (or a script) and then a **human reviews tables, mermaid and links**.
- Done when: all internal links work (checked by script), no `.html` files remain in `docs/` outside `mockups/`.

**Agent D2 â€“ Fix docs content** (runs after D1 to avoid editing twice)
- Tasks, following the appendix and sections 3-5 of `docs/audit/AUDIT-REPORT.md`:
  1. Separate information from other projects (D5): README, doc 06 (use the name "HPay Service Engine"), `index`, doc 00, doc 04, master-plan. The comment in `approval/service.go` is already fixed.
  2. Figures: number of migrations (13, 14 after A1), E01â€“E20 and "39 cases", number of mockups, F11 into the master-plan, EN version "$200" â†’ 5,000,000 VND, "US wallet" â†’ state explicitly that it is a UI style.
  3. KYC limits: a single table (D7), other places reference it.
  4. Enums: `processing_async`, `cancelled`, `reversal`, `compensation_failed`, roles L1/L2 mapped to `org_role`.
  5. Fix docs to match the code: asynq â†’ goroutine + lease (D8), Cloudinary storage, add `/v1` to endpoints, relationship between `verify` and `confirm`.
  6. Write an "Invariants and exceptions" section: idempotency, reversal, balance hold while awaiting approval, lock order, system wallet allowed to go negative, a reconciliation job `wallets.balance = SUM(ledger_entries)`, fee (rounding, who pays, fee refund), timezone for limits, behavior on LLM failure.
- Done when: no contradictions remain in the section 4 list of the report.

### Wave 6 â€“ Tests, test-case-driven verification, fix loop (after Waves 2-4)

**Agent T1 â€“ Backend integration tests**
- Tasks: map test cases (xlsx `qa/test-cases/*`) to `TestEdge_<ID>_*`. Add the missing tests: ledger post twice, crash-recovery MOVE_FUNDS, offerguard race, idempotency across users and with different payloads, concurrent reversal and receiver out of funds, confirm by a non-owning user, forged signed URL, path traversal, property-based `fee.Calc`. DB tests use real Postgres + Redis via docker compose (the repo currently does not use testcontainers, none needed).
- Loop: run â†’ fail â†’ `systematic-debugging` finds the root cause â†’ fix â†’ rerun, at most 3 rounds per bug; beyond 3 rounds, stop and report to the user.

**Agent T2 â€“ E2E**
- Tasks: Playwright for admin (`e2e/` already exists), add tests for mobile web or Maestro if feasible. Cross-check against the xlsx test cases. E2E login uses a dedicated account from env.

**Agent T3 â€“ Static scanning and regression**
- Tasks: `govulncheck`, `gosec`, `gitleaks`, `npm audit`, `gitnexus analyze --pdg` then `explain` to review taint, `detect_changes` against baseline, report in `docs/handoff/final-verification.md`.

---

## 5. Items that need your decision or input

### 5.1 Only you can do
- Rotate Neon and Cloudinary keys (W0-1).
- Approve the deletion file list (W0-3).
- Provide or confirm business policy for D3, D7, D10, D11, D12 (D3, D7, D10, D11 are answered; D12 remains open).

### 5.2 KYC limits (D7) â€“ currently 4 sources disagree
Decision: take the seed values in code (`OFFER_CUST_L1` = 50M, `OFFER_CUST_L2` = 200M VND). The remaining sources below must be aligned to them.

| Source | Value |
|---|---|
| Mockup `01-auth-kyc.html:118` | 100 million VND/day |
| Seed `OFFER_CUST_L1/L2` (`primary_pocket_max`) | 50 million / 200 million |
| `offer_services` default | 500 million/day, 2 billion/month |
| Doc 05 | 100M/txn, 500M/day, 2 billion/month |
| Vietnamese functional overview | 5 million (tier threshold) |

You still need to choose the canonical table per KYC tier (L0/L1/L2) for the remaining fields: limit per transaction, per day, per month, maximum balance.

---

## 6. Parts still unverified (stated explicitly so they are not missed)

The items below have not been read or run by an agent. Each item is assigned to the agent that will handle it.

| Item | Assigned to |
|---|---|
| Compare transaction/money-request/KYC status enums with the DB, compare each response field | F1 |
| The two largest docs files (`00-backend-design-v2-base.html`, `technical-overview.*`) have not been read line by line | D2 |
| The xlsx test case files and `docs/reference/external/*.docx` have not been opened | T1, D2 |
| `workflow/engine.go` only had the compensation area checked; `round_up.go`, `fraud/rules.go`, `fraud/admin.go`, `product` (write part), `moneyrequest/sweeper.go`, `recon/cron.go` have not been read | B1, B2 |
| Bank callback signature (`wallet/callback.go`), `hmac.Equal` in the QR signer | B2, C2 |
| `org_handler`: who may create a disbursement; role changes after the order was created | B2 |
| Recon cron with multiple replicas | B2 |
| Whether `kyc-draft.ts` stores images in SecureStore | F2 |
| Whether the sample URL at `ServicePipelineModal.tsx:66` is a default sent to the backend | F3 |
| `docs/skills` and `src/backend/.claude/skills`: differ in content or only in CRLF | W0-5 |
| Mobile `LICENSE`, bundle name | F2 |
| No test or application has been run to confirm bugs empirically | W0-6, T1 (this is the most reliable way) |

**Refuted:** mojibake. The files are valid UTF-8; only the console displayed them wrongly, so there is no encoding fix to make.

---

## 7. Order and dependencies (summary)

```
W0 (foundation, sequential)
 â””â”€ W1: A1 (migration) â”€â” A2 (config) â”€â” A3 (monorepo/CI)   [parallel]
        â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”´â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
 â””â”€ W2: B1 (ledger/workflow/idempotency) â†’ B2 (offerguard/reversal/approval)
 â””â”€ W3: C1 (auth/PIN/RBAC) â€– C2 (KYC/storage)      (after W1; C1 before F2)
 â””â”€ W4: F1 (contract) â†’ F2 mobile â€– F3 admin       (after W2/W3 for the contract)
 â””â”€ W5: D1 (HTMLâ†’MD) â†’ D2 (fix content)            (independent, can run right after W0)
 â””â”€ W6: T1 â€– T2 â†’ T3 (final confirmation, fix loop)
```

Wave 5 depends only on W0, so it can run in parallel with W1 to save time. Each wave ends with a merge and a re-run of the baseline.

## 8. Main risks of the plan

- **Migration 000014 changes the UNIQUE of the idempotency key on a DB that already has data.** Check whether the DB in use (Neon) has real data, and that a backup exists before applying.
- **Changing the `request â†’ confirm` flow** affects both mobile and admin; F1 must settle the contract before F2/F3 start.
- **Background agents cannot ask back.** Every D* decision must be settled before assigning work, otherwise the agent will guess.
- **The audit results are based on reading code**, with no tests actually run. Step W0-6 and T1 are where bugs are proven or refuted.

## Decision update 2026-11-21: KYC limits are attached to offers

**Decision (user):** limits depend on the user's *offer* (`offers` + `offer_services` + `offer_auth_matrix`). A user's KYC tier selects the offer; the offer defines the limits. The hard-coded `wallet.TxnLimit` (2M / 5M / 100M by KYC status) is not the source of truth and will be removed once the offer path covers every money-moving flow.

**What this means in code (not yet built, tracked as work for `hpay-feature`):**
1. Define the tier -> offer mapping (which KYC status gets `OFFER_CUST_L1`, which gets `OFFER_CUST_L2`, what an unverified user gets). **Open question for the user:** the exact limit table below.
2. CORRECTION (verified in migration 000006:106-112): `OFFER_CUST_L1` DOES have `offer_services` rows for every active service, but they carry placeholder limits identical to L2 (100,000,000 per transaction, 500,000,000 per day, 2,000,000,000 per month), plus the standard PIN/OTP/2FA matrix. So L1 and L2 differ today only in `primary_pocket_max` and bank-account count, and the final per-offer limits still have to be set. `OFFER_BIZ_ENT` has no `offer_services` rows in any migration, so assigning it would block every service.
3. Assign the offer when KYC changes (`SetUserKycFromPending` / `SetUserKycFromManualReview` paths) in the same transaction, instead of backfilling everyone to `OFFER_CUST_L2`.
4. Apply the offer guard on every money path (the engine path today skips `wallet.TxnLimit`; the legacy paths are dead code), then delete `TxnLimit`.
5. Inactive offer must block, not fall back to the default offer.
6. `primary_pocket_max` (50M / 200M) is a wallet balance ceiling, not a per-transaction limit; enforce it on credits or remove it from the offer model.

**Limit table to be confirmed by the user (values to fill in; nothing below is decided):**

| Offer | KYC tier | Per transaction | Per day | Per month | Wallet balance ceiling (`primary_pocket_max`) |
|-------|----------|-----------------|---------|-----------|-----------------------------------------------|
| (unverified) | none / rejected | ? | ? | ? | ? |
| OFFER_CUST_L1 | tier 1 | 100,000,000 (placeholder, migration 000006) | 500,000,000 (placeholder) | 2,000,000,000 (placeholder) | 50,000,000 (seed) |
| OFFER_CUST_L2 | tier 2 | 100,000,000 (seed) | 500,000,000 (seed) | 2,000,000,000 (seed) | 200,000,000 (seed) |

Cells marked "seed" or "placeholder" exist in the migrations today; "placeholder" means a default inserted so the guard does not block demo flows, not a product decision. Every "?" is a product decision still to be made.

## Status 2026-11-21 (end of the first implementation pass)

State: Waves 1 to 4 and the knowledge base are implemented in one working tree and committed together; Wave 6 is partly done. ADR-0010 to ADR-0015 are accepted by the owner. Evidence: `docs/handoff/final-report.md`, `docs/quality/security-scans.md`, `docs/bugs/INDEX.md`.

### Done
BE-C1 (ledger posting guard), BE-C2 (server-side PIN gate), BE-H1 (idempotency per user, including org), BE-H3/H4 (reversal integrity, `needs_manual`, `compensation_failed`, stuck-compensation recovery), BE-H6 (admin RBAC from the database), BE-H7 (explicit APP_ENV, environment guards), SSRF guard, KYC upload validation (upload part of BE-H9), KYC limits attached to offers (ADR-0007: L0/L1/L2, KYC outcome selects the offer, `wallet.TxnLimit` removed), admin and mobile hardening, foreign-project names removed from code, documentation as an English Markdown knowledge base.

### Open gaps (each needs its own `hpay-feature` or `hpay-bugfix` run)
| # | Gap | Why it matters | Next step |
|---|-----|----------------|-----------|
| G1 | Go toolchain 1.25 has 18 reachable `govulncheck` findings, all fixed in Go 1.26.x (`net/http`, `crypto/tls`, `x/net`, `x/text`, plus `excelize` pseudo-versions) | known CVEs in the HTTP stack | owner decides on Go 1.26; then bump `go.mod`, CI image and Dockerfile |
| G2 | Mobile `npm audit`: high findings in `@xmldom/xmldom` (Expo toolchain) | build-time dependency | Expo SDK bump |
| G3 | `gosec`, `gitleaks` not run (not installed); Playwright e2e not run | static and browser coverage missing | add both scanners to CI; run e2e against backend plus seed |
| G4 | QA workbooks: only TC-IDM merged; TC-LED, ENV, REV, AUT, RBA, SSRF, KYC, offer-limits cases live only in `docs/specs/*/test-cases.md` | traceability from case to test | run `hpay-qa-testcases` with `--allow-new-ids`, record results |
| G5 | **Backend done 2026-11-21** (ADR-0017): TOTP second factor (encrypted secret, single-use codes, recovery codes, operator `totp.reset`) and a Twilio Messaging delivery adapter behind the Sender port. Still open: mobile and admin screens, Twilio keys from the owner (production cannot send codes until configured), a biometric factor and the legal question of State Bank Decision 2345 (biometric above 10 million VND), a Vietnamese SMS adapter, key rotation | business flow blocked | mobile screens, owner keys, legal decision |
| G6 | **Backend done 2026-11-21** (ADR-0016): password plus one-time code self-service reset and an operator `pin.reset` fallback, a 24 hour cooling cap, audit and notice. Still open: the mobile set-PIN and reset screens (planned in `docs/specs/w5-mobile-security/plan.md`) and an admin UI for the operator clear (the Twilio adapter now exists; production needs its keys from the owner) | a user cannot recover a locked or forgotten PIN | mobile screens, provider ADR |
| G7 | BE-H8: no login lockout and no constant-time handling | brute force | bugfix run |
| G8 | BE-H9 remainder: public Cloudinary delivery, file-link signing key is the JWT secret, no identity-bound audited file access, no EXIF strip, no orphan cleanup | KYC images are personal data | ADR plus feature run |
| G9 | Weekly rolling limits, balance-ceiling enforcement (`primary_pocket_max` is stored only), single-vs-multi currency, per-tier auth thresholds; 25,000 VND per USD is an assumption | the approved T0/T1/T2 table is only partly enforced | spec plus ADR for the window feature |
| G10 | Nothing assigns `OFFER_CUST_L2`; the L2 qualification is undefined; registration creates no `user_offers` row | tier T2 unreachable except by hand | product decision |
| G11 | Compliance review per jurisdiction for the limit table | numbers come from third-party blogs, not regulation | legal review |
| G12 | **Done 2026-11-24** (`specs/g12-needs-attention-queue`): admin "Needs attention" page, `status=needs_manual` approvals filter, `GET /v1/admin/attention/summary`, sidebar badge. Still open: resolving a parked case is manual and undefined; no push or e-mail alert; browser check not run | parked money cases were invisible | owner: browser check, resolution policy |
| G13 | Admin cannot edit offers (no write route); offers change only by migration | operations | feature run |
| G14 | DONE 2026-11-24: lint warnings fixed at the cause (admin 6 -> 0: memoised `rows`/`flags`/`offers`, KYC detail key handler via latest-handler ref; mobile 8 warnings -> 0: unused imports/vars removed, receiver effect depends on stable `mutate`). Admin lint now enforced in admin-ci (job lint-unit); mobile has no CI workflow | quality gate | done |
| G15 | Fraud LLM advisory is fire-and-forget; rule errors are retryable (fail-closed verified) but there is no test for an advisor outage | regression guard | add a test |
| G16 | Wave 5 content fixes still open: mockups state old KYC numbers, `docs/reference/api.md` section on unreachable endpoints needs a status note | knowledge-base accuracy | docs pass |
| G17 | Key rotation (Neon, Cloudinary, JWT) postponed by the owner; the local `.env` JWT secret is the public placeholder | secrets | owner, when ready |
| G18 | Daily and monthly windows use the server time zone and read-then-insert (BE-H2) | fixed 2026-11-24, BUG-0017 | none |
| G19 | Outbox and consumer idempotency for non-money steps | fixed 2026-11-24, BUG-0019, BUG-0020 (follow-up: unique index on `fraud_flags(transaction_id)`, partner must honour `Idempotency-Key`) | none |

### Breaking changes of this pass
`APP_ENV` is required; confirming a payment needs a PIN (users must set one); the 2fa band is refused; users without an offer fall to L0 (2,500,000 VND per transaction).

### Load test findings, 2026-11-24 (owner's run; details in `docs/quality/performance/2026-11-24-load-test-baseline.md`)
Correctness held (600 of 600 transfers, 0 ledger mismatches) but throughput was about 8 completed transfers per second. Order chosen by the owner: G20, G21, G22, then G23, G24, G25.

| # | Gap | Why it matters | Next step |
|---|-----|----------------|-----------|
| G20 | P4: a fresh clone does not build (`internal/store/gen` is gitignored, sqlc is not run by the quick start) | a reviewer cannot run the project | state `make sqlc` first in the README and run it in the test scripts |
| G21 | P5: `scripts/loadtest/k6-transfer.js` calls routes that no longer exist and ignores login and confirm rate limits | `make loadtest` is broken | point `make loadtest` at the Go load test, or rewrite the k6 script for request plus confirm |
| G22 | P6: integration packages share one database, so `go test ./...` in parallel deadlocks and steals outbox events; `PumpLocal` marks events published before processing | flaky local runs; a real deadlock on `SYSTEM_CASH_IN` may exist in production too | run with `-p 1` or one database per package in the Linux script; check lock order of funding |
| G23 | P1: one consumer goroutine per workflow stream (about 13 events per transfer) | caps completion near 8 transfers per second | several consumers per stream with per-instance ordering and a test with 2 or more workers; re-measure |
| G24 | P3: the outbox relay waits for the next 200 ms tick even when the batch was full | next ceiling after G23 (about 38 transfers per second) | relay again at once when a batch is full; configurable batch and interval |
| G25 | P2: PIN verification uses login-grade argon2id (64 MiB) on every payment | 50 concurrent confirms need about 3.2 GB and saturate the CPUs | bound concurrent hashes with a semaphore, keep the strong parameters (owner's choice); an ADR if the parameters ever change |


Status 2026-11-24: G20, G21, G22 (PumpLocal part), G23, G24, G25 (semaphore) are done; results in docs/quality/performance/2026-11-24-load-test-baseline.md. Open: lighter PIN hashing parameters (needs an ADR) and the SYSTEM_CASH_IN funding lock-order check.

Status 2026-11-24 (later): the SYSTEM_CASH_IN funding deadlock was reproduced and fixed (FOR NO KEY UPDATE in GetWalletsForUpdate). Only open item from the load-test findings: lighter PIN hashing parameters, which need an owner decision and an ADR.

G6 status 2026-11-24: mobile Set PIN (security/set-pin) and Forgot PIN (security/reset-pin) screens, PIN prompt link and account rows are built with jest tests (plan steps 2 and 3); on-device run still pending (HUMAN cases TC-MSEC-029/030 Blocked). Step-up at confirm and TOTP are a separate worktree.
