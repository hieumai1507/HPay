[← HPay Docs](../../../../docs/index.html) · repo `hpay-admin` · plan

# Implementation Plan — Playwright E2E Suite for hpay-admin

**Feature slug:** `e2e-playwright-suite` · **Source:** [functional-overview.vi.html §🛡️ Operations Team — Admin web](../../../../docs/guide/functional-overview.vi.html#admin) (8 features) + [03-admin-solution-architecture.html §5](../../../../docs/architecture/03-admin-solution-architecture.html) (edge cases A01–A07). This is plan-only: **do not install Playwright, do not write specs, do not create real CI files** — wait for user approval before doing the work (per the request "I must approve before you are allowed to do it").

## 1. Current state as surveyed

| Layer | Reality in the code | Conclusion |
|---|---|---|
| Existing test tooling | hpay-admin `package.json`: no Playwright, no Vitest/Jest, no test runner of any kind — only `tsc --noEmit` + `eslint` | ❌ The entire E2E infrastructure is new, nothing to reuse on the FE side |
| Real routes | `src/router.tsx`: `/login`, `/dashboard`, `/kyc`, `/kyc/:id`, `/fraud`, `/workflows`, `/workflows/:id`, `/tasks`, `/recon`, `/orgs`, `/ai-settings`, `/audit` — all behind the `RequireAdmin` guard except `/login` | ✅ The route list matches the scope of the 8 functional-overview features (dashboard/KYC/fraud/workflow/recon/my-tasks/audit/orgs); `ai-settings` and `services/offers` (Service Catalog, Offer Factory) are outside the Admin scope per functional-overview → P2/not required in this round |
| Real auth | `LoginPage.tsx` calls `POST /v1/admin/auth/login {email,password}` → `useAdminStore.setSession(token, admin)` writes into `sessionStorage['hpay-admin-session'] = {token, admin}` (sessionStorage is intentional — the session does not survive closing the tab). Backend: `AdminService.Login` (`internal/auth/admin.go`) issues a JWT with audience `admin`, TTL **15 minutes** (`AccessTokenTTL`, `internal/auth/jwt.go`), **access-only — there is no refresh token for admin yet** (comment in the code: "Access-only tokens for now") | ⚠️ A 15-minute TTL is long enough for a Playwright run (usually <5 minutes/spec) but leaves no margin if the suite runs sequentially for a long time → mint a new token per worker, do not share 1 token across the whole suite |
| Existing admin account | Migration `000001_users_admin_auth.up.sql` seeds exactly 1 row: `admin@HPay.dev`, role `superadmin`, password hashed with argon2 — **the plaintext appears nowhere in the repo** (no log, no comment, no doc) | ❌ This demo account cannot be used for E2E because the plaintext is unknown, and My Tasks needs **≥2 admins with different roles** (maker-checker) while the current seed has only 1 → a dedicated admin fixture for E2E must be added (section 3) |
| `cmd/seed/main.go` | 643 lines, idempotent (upsert by phone/idempotency key), creates exactly the data admin needs: user `0900000003` in the KYC queue (ai_score 64), user `0900000004` locked + confirmed_fraud + reversal pending in My Tasks, varied workflows for the monitor — but this seed takes the first admin in the table (`ORDER BY created_at LIMIT 1`) for the reversal approval, meaning only 1 real admin exists after the seed runs | ✅ ~90% reusable for business data (KYC pending, fraud flag, stuck workflow) — only the admin fixture part needs to be added, no need to rewrite the seeder |
| Backend CI pattern | `src/backend/.github/workflows/ci.yml` job `test`: service containers `postgres:16-alpine` + `redis:7-alpine` with healthchecks, `migrate` tool installed via curl, runs migrations then `go test` | ✅ The service-container + healthcheck pattern is reusable for the new E2E job in hpay-admin, but hpay-admin is a separate repo — the backend code is not available in its checkout (section 5) |

## 2. Scope & Non-goals

### Ships (this plan covers, to be implemented after approval)

- A Playwright project (TypeScript) in `src/admin/e2e/`, configured by `playwright.config.ts`, running Chromium in CI.
- Fixture strategy: extend `cmd/seed/main.go` (or add a `--profile=e2e` flag) to create 2 fixed admin_users with known plaintext, serving maker-checker (My Tasks) and concurrency tests (A01/A05).
- Auth strategy: mixed — most tests bootstrap the session through the real login API (not through the form) + exactly 1 test goes through the real `LoginPage` form (details in the "Auth strategy" section).
- 8 spec files, one for each Admin feature per functional-overview: dashboard, KYC queue, fraud monitor, workflow monitor, reconciliation, org/policy, My Tasks, audit log.
- A new CI job (`e2e`) in `src/admin/.github/workflows/ci.yml`, mirroring the hpay-backend service-container pattern, checking out hpay-backend as a sibling to build the real backend.
- Flakiness strategy (retry, wait, trace) + the blocking vs non-blocking decision for the initial period.

### Non-goals (defer)

- **Cross-browser** (Firefox/WebKit) in CI — Chromium only, see the trade-off in section 3.
- `ai-settings`, `services` (Service Catalog), `offers` (Offer Factory) — not among the 8 "Operations Team" items of functional-overview; leave for a separate E2E plan if needed.
- Visual regression / screenshot diffing — out of scope, can be added once the behavior suite is stable.
- Load/performance testing — Playwright here only verifies correct behavior, not load.
- Publishing the backend Docker image to a registry (GHCR) for admin CI to pull — proposed in the "API changes" section as Option B but not implemented in this round; Option A (sibling checkout) is chosen.

## 3. Playwright project setup

| Decision | Value | Why |
|---|---|---|
| Test location | `src/admin/e2e/` (specs: `e2e/*.spec.ts`, helpers: `e2e/fixtures/`, `e2e/utils/`) | Separate from `src/` — not mixed with app code, not scanned by the app's `tsc --noEmit`; kept out of `vite build` |
| Language | TypeScript, a dedicated `tsconfig.json` for `e2e/` (like `scripts/` already has `scripts/tsconfig.json` for `gen-api.ts`) | Consistent with the existing toolchain (100% TS in the repo), can obtain API response types if needed |
| Config | `playwright.config.ts` at the repo root, `testDir: './e2e'`, `baseURL` from env `E2E_BASE_URL` (default `http://localhost:4173` — the `vite preview` port) | Allows running locally (dev server) and in CI (preview build) without changing test code |
| CI browser matrix | **Chromium only** (project `chromium`) in default CI; Firefox/WebKit locally/manually (`--project=firefox`) or a separate nightly job if needed later | **Deliberate trade-off:** full-stack E2E (real backend + DB) is already slow; multiplying by 3 browsers multiplies CI time by 3 for little benefit — the admin web has no CSS-engine-specific logic, so cross-browser risk is much lower than the benefit of PR feedback speed. We accept the risk of missing Firefox/Safari-specific bugs, offset by a non-PR-blocking nightly cross-browser job |
| Reporter | `html` (local) + `github` (annotate the PR) + `junit` (so GitHub Actions shows the standard test summary) | Fast feedback loop for reviewers directly on the PR, no need to open an artifact |
| Retries | `retries: 2` only when `process.env.CI`, `0` when running locally | Shields against network/timing flake in CI without hiding real bugs when debugging locally (see Rollout) |
| Parallelism | `fullyParallel: true` but `workers: 2` in CI (not the default = number of CPUs) — because each worker mints its own token and some specs (My Tasks, Orgs) write shared data | Avoids 2 workers modifying the same data row at the same time → false-positive 409 (different from the intentional 409 tests in A01/A05) |

## 4. Test data / fixtures strategy

**Confirming the premise in the brief:** every CI run will have a **completely empty** postgres container (a fresh service container, no volume) — this is already a "fresh DB per CI run" for free, no extra cleanup mechanism is needed. The remaining problems are: (a) the shared baseline data, and (b) isolation between tests when they write/modify data.

### 4a. Baseline data (global setup, runs once)

- After migrating the empty DB, run **exactly once**: `go run ./cmd/seed` (backend, builds the existing business data: KYC pending, fraud flag, varied workflows, org + policy) **plus** a small extra step that creates **2 fixed admin_users** with known plaintext — e.g. `e2e-maker@HPay.dev` (role `ops`) and `e2e-checker@HPay.dev` (role `superadmin`), fixed password `E2ETest@123`. Approach: add an `--e2e-admins` flag to `cmd/seed/main.go` (separate from the default dev seed so test credentials do not mix into the demo) — implementation details belong to a later phase, this is only an architectural decision.
- Do not create the admin fixtures through a new migration — the seed script is the right place (there is precedent for seeding a single admin in migration 000001, but adding 2 test-only accounts to a migration would leak into the production schema; the seed script runs in CI/dev without touching migrations).

### 4b. Isolation between tests (per-spec, not per-test)

- **Read-only specs** (Dashboard, Workflow Monitor filters, Audit search, Orgs list) use the seed data directly — no dedicated fixture needed, safe to run in parallel.
- **Specs with mutations** (KYC approve/reject, Fraud resolve, My Tasks approve/reject, Orgs policy save) create their own records in `test.beforeEach` by calling the real backend API directly (not through the UI) with the minted admin token — e.g. create 1 new KYC-pending user via an internal endpoint/seed helper instead of sharing user `0900000003` that every other spec also reads. Named by `test.info().testId` so they do not collide when running 2 workers in parallel.
- Do not use DB-test-style transaction rollback (a common pattern in unit tests) because this is E2E over real HTTP with multiple services (async workflow engine) — rolling back between tests would break side effects that were already triggered. Accept data accumulating throughout one CI run, cleaned up by the postgres container being destroyed at the end of the job.

## 5. Auth strategy — a real recommendation, not a neutral one

**Recommendation:** **a mixed approach with a bias** — most tests bootstrap the session by calling `POST /v1/admin/auth/login` directly over HTTP (not through the form), obtain the `access_token`, then use `page.addInitScript` to write directly into `sessionStorage['hpay-admin-session']` in the exact `{token, admin}` format that `useAdminStore` reads — **before** navigating to the route under test. Exactly **1 spec** (`e2e/login.spec.ts`) goes through the real form.

### Why not reuse the "throwaway Go script minting a JWT" as in the previous verification

Last time a Go script directly calling `internal/auth.JWTIssuer.Issue()` was used to mint the token, bypassing the auth service layer entirely. That approach is fast but carries a real risk for E2E: if someone breaks the `AdminService.Login` logic (wrong role mapping, forgetting to check `status=active`, changing the response field...), the tests stay green because they never touch that code. An E2E of the whole admin console where the auth service is on no test path is a significant coverage gap — auth is the first thing every session must pass through.

### Why not make every test go through the UI form

The UI form (click, type, submit, wait for redirect, wait for an error toast if any) costs ~1–2s + a DOM round-trip each time, multiplied by dozens of specs it adds up to minutes of pointless waiting — the login UI is already fully covered by `login.spec.ts` itself, repeating it in every other spec does not increase coverage, it only increases CI time and the flake surface (network, animation, redirect timing).

### Concrete choice

| Test group | How to enter the session |
|---|---|
| `login.spec.ts` (1 spec, ~2–3 cases) | Real form: correct email/password → redirect `/dashboard`; wrong password → inline error message, no redirect; (optional) account with `status=locked` → a clear error |
| All remaining specs (KYC, Fraud, Workflow, Recon, Orgs, My Tasks, Audit, Dashboard) | Global fixture `e2e/fixtures/auth.ts`: calls the real `POST /v1/admin/auth/login` with the fixed seed credentials (section 4a), injects into `sessionStorage` via `addInitScript` before each test that needs that role |
| My Tasks (maker-checker) | 2 dedicated fixtures: `asMaker` (login `e2e-maker@HPay.dev`) and `asChecker` (login `e2e-checker@HPay.dev`) — use `test.use({storageState:...})` or 2 `browserContext`s in the same test to faithfully simulate A01 (2 reviewers at the same time) |

The 15-minute token TTL (access-only, no refresh) is enough for one spec file (usually <1 minute), but the fixture mints a **new token for each test file** (not sharing 1 token across the whole suite) so it does not depend on run order and avoids the last tests of the suite hitting an expired token if the suite runs slowly on a busy CI.

## 6. Task breakdown — by page, with case counts & priority

| # | Page / feature | Expected cases | Priority | Referenced edge cases |
|---|---|---|---|---|
| T0 | Login (real form) | 2–3 | P0 | — |
| T1 | Overview Dashboard | 2 | P1 | — |
| T2 | KYC queue + review action | 5 | P0 | A01 (409 when 2 reviewers approve at the same time), A04 (expired signed KYC image URL → auto refetch) |
| T3 | Fraud monitor + resolve | 4 | P0 | A01 (409 similar to KYC) |
| T4 | Workflow Monitor + filter | 4 | P1 | A02 (cursor pagination when a new instance is inserted while the reviewer is on a later page) |
| T5 | Reconciliation (run + view + export Excel) | 3 | P1 | A06 (heavy export → poll, UI does not hang) |
| T6 | Organizations & Policy | 3 | P2 | A05 (2 tabs editing the policy) — in-flight requests warning |
| T7 | My Tasks (admin maker-checker) | 4 | P0 | A01 (2 checkers decide the same task → 1 wins, 1 gets 409 + toast + row updated in place, no whole-table reload) |
| T8 | Audit Log search | 2 | P2 | — |
| T9 | Cross-cutting: session expires during review | 1 | P1 | A03 (token expired → re-login modal keeping the same URL + filter in search params) |
| T10 | Cross-cutting: PII masking in step detail JSON | 1 | P2 | A07 (step input/output JSON does not expose raw PII via DevTools/response) |

**Total estimate: ~31–33 test cases** spread over 9 spec files (login, dashboard, kyc, fraud, workflow, recon, orgs, my-tasks, audit) + 1 cross-cutting file (session-expiry and pii-masking combined in `edge-cases.spec.ts`).

## 7. CI job design (replaces "API changes" — there is no new API)

There is no new schema/API on the backend or admin side (the test suite purely consumes existing APIs). The real design lies in the CI job — sketch below, **a proposal document only, not a real YAML file committed in this plan round**.

### Decision: check out hpay-backend as a sibling repo in the CI job

**Option A (recommended, the sketch below uses this option):** the hpay-admin E2E job additionally checks out `hieumai1507/hpay-backend` into a subdirectory, builds the Go binary inside the job, migrates the DB, starts the server, then runs Playwright against that server. **Option B (not chosen this round):** hpay-backend CI builds & publishes a Docker image to a registry (GHCR) on every merge to main; hpay-admin CI just `docker pull`s the prebuilt image — faster, separates toolchains, but requires new work in hpay-backend (adding an image-publish job) that does not exist and is outside the scope of this plan. A is chosen because it can be implemented right away in the hpay-admin repo without changing hpay-backend; B is recorded as a future improvement (see Non-goals).

```
# src/admin/.github/workflows/ci.yml — new job "e2e" (SKETCH — not a real committed file)
e2e:
  runs-on: ubuntu-latest
  services:
    postgres:
      image: postgres:16-alpine
      env: { POSTGRES_USER: HPay, POSTGRES_PASSWORD: HPay, POSTGRES_DB: HPay }
      ports: ["5432:5432"]
      options: >-
        --health-cmd "pg_isready -U HPay" --health-interval 3s --health-retries 10
    redis:
      image: redis:7-alpine
      ports: ["6379:6379"]
      options: >-
        --health-cmd "redis-cli ping" --health-interval 3s --health-retries 10
  env:
    DB_URL: postgres://HPay:HPay@localhost:5432/HPay?sslmode=disable
    REDIS_ADDR: localhost:6379
    E2E_BASE_URL: http://localhost:4173
  steps:
    - uses: actions/checkout@v4
      with: { path: hpay-admin }

    - uses: actions/checkout@v4          # Option A: sibling checkout
      with:
        repository: hieumai1507/hpay-backend
        ref: main                        # pin to a tested tag/SHA if you want to reduce drift risk
        path: hpay-backend

    - uses: actions/setup-go@v5
      with: { go-version-file: src/backend/go.mod }
    - uses: sqlc-dev/setup-sqlc@v4
      with: { sqlc-version: "1.27.0" }

    - name: build + migrate + seed backend
      working-directory: hpay-backend
      run: |
        sqlc generate --no-remote
        curl -fsSL https://github.com/golang-migrate/migrate/releases/download/v4.17.1/migrate.linux-amd64.tar.gz | tar xz
        sudo mv migrate /usr/local/bin/
        migrate -path db/migrations -database "$DB_URL" up
        go build -o /tmp/hpay-server ./cmd/server
        DATABASE_URL="$DB_URL" go run ./cmd/seed --e2e-admins   # T4a: baseline data + 2 admin fixtures

    - name: start backend
      working-directory: hpay-backend
      run: |
        DATABASE_URL="$DB_URL" REDIS_ADDR="$REDIS_ADDR" /tmp/hpay-server &
        npx wait-on http://localhost:8080/healthz -t 30000   # healthcheck before the FE build/tests run

    - uses: actions/setup-node@v4
      with: { node-version: 20, cache: 'npm', cache-dependency-path: src/admin/package-lock.json }
    - name: install + build admin
      working-directory: hpay-admin
      run: |
        npm ci
        npm run build
        npx playwright install --with-deps chromium
    - name: serve admin (preview build)
      working-directory: hpay-admin
      run: npx vite preview --port 4173 &
        npx wait-on http://localhost:4173 -t 15000

    - name: run Playwright
      working-directory: hpay-admin
      run: npx playwright test --project=chromium
      env:
        CI: true

    - uses: actions/upload-artifact@v4     # trace/report on failure, to debug flake
      if: failure()
      with: { name: playwright-report, path: src/admin/playwright-report/, retention-days: 7 }
```

**Merge blocking:** add the `e2e` job to the `needs` of any aggregate "gate" job (or make it a required status check in hpay-admin's branch protection itself) — the same way hpay-backend's `build` declares `needs: [lint, test]`. Details of whether to make it "required" from the start or later are in the Rollout section.

## 8. Test plan (summary mapped to concrete cases)

| Edge case | Test |
|---|---|
| A01 | `TestE2E_MyTasks_TwoCheckersRaceOneWins` — 2 `browserContext`s (the same checker or different checkers) both click "Approve" on 1 task; one context receives 200 + the row updates, the other receives an error toast + the row automatically updates to the new state, without reloading the whole table |
| A02 | `TestE2E_Workflow_PaginationStableUnderInsert` — open page 3, create a new workflow instance through a background API, next/prev does not drift the rows being viewed (cursor, not offset) |
| A03 | `TestE2E_SessionExpiry_ReLoginKeepsFilters` — set a filter + query param, simulate token expiry (overwrite the token in sessionStorage with garbage), the next action → re-login modal → log in again → return to the exact same URL + filter |
| A04 | `TestE2E_KYCDetail_ImageURLExpiryRetry` — mock the image URL returning 403 the first time, verify the FE requests a new URL exactly once before reporting an error |
| A05 | `TestE2E_Orgs_TwoTabsEditPolicy` — 2 tabs edit the policy of the same org, tab 2 saves later and receives 409 handled via the shared interceptor (a consistent toast, no crash) |
| A06 | `TestE2E_Recon_ExportDoesNotBlockUI` — click Excel export, verify the UI stays interactive while waiting (no full-page spinner blocking the whole screen), poll until the download link is available |
| A07 | `TestE2E_WorkflowDetail_StepJSONMasked` — open a step detail containing PII, verify the network response + DOM do not contain raw PII (already `***`) |
| — | `TestE2E_Login_HappyPath` / `TestE2E_Login_WrongPassword` — through the real form |
| — | `TestE2E_KYC_ApproveReject`, `TestE2E_Fraud_ConfirmDismiss`, `TestE2E_MyTasks_ApproveReject` — happy-path CRUD for each mutation page |

## 9. Rollout & risk

| Risk | Mitigation |
|---|---|
| Full-stack E2E is inherently flakier than unit/integration tests — network, UI timing, races among multiple services (async workflow, redis) | `retries: 2` only in CI; **no arbitrary `page.waitForTimeout`** — waiting must be condition-based (`expect(locator).toBeVisible()`, `page.waitForResponse()`, network-idle for polling pages like Recon); trace-on-retry is enabled so debugging needs no local rerun |
| hpay-backend `main` changes incompatibly with already-written specs (the sibling checkout always takes the latest `main`) | A failed E2E job is clearly labeled "backend drift" in the step name; consider pinning a specific ref after the trial period if drift causes too much noise |
| Data shared between specs running in parallel causes false-positive 409 (different from the intentional 409 tests) | `workers: 2` limit instead of the CPU-based default; mutation specs create their own records per `testId` (section 4b) |
| CI becomes noticeably slower (Go build + FE build + running 30+ full-stack Playwright cases) | Estimated ~4–6 extra minutes for the `e2e` job (backend build ~1m, FE build ~1m, Playwright Chromium-only ~2–3m) — runs in parallel with the existing hpay-admin `lint`/`test` jobs (if any), not blocking sequentially |

### Blocking or non-blocking from the start — recommendation

**Recommendation: non-blocking (not a required check) for the first 1–2 weeks**, using `continue-on-error: true` for the `e2e` job or not adding it to the branch protection required status check list, with a clear notice in the PR (comment/summary) when it fails. After the trial period — when the suite runs stably across several consecutive PRs with no flake-induced false positives — switch to required/blocking. Reason: this is a brand-new, full-stack suite, with many components coordinating in CI for the first time (sibling checkout, real backend healthcheck, E2E-only seed) — if it must pass from day 1, a CI infrastructure error (not a product bug) could block every unrelated PR, destroying trust in the gate and creating pressure to bypass/skip tests — a larger risk than the benefit of "blocking right away".

There is no migration/schema in this plan → no DB rollback. Rollback if needed: revert the PR that adds the CI job, with no effect on product code.

## 10. Execution checklist (for the implementation phase after approval)

1. `impact()` on any symbol touched when adding the `--e2e-admins` flag to `cmd/seed/main.go` (on the hpay-backend side, per that repo's CLAUDE.md).
1. Scaffold `e2e/playwright.config.ts` + `e2e/fixtures/auth.ts` (T0 auth fixture first, every other spec depends on it).
1. Write specs in P0-first priority order: T0 → T2 → T3 → T7, then P1 (T1, T4, T5, T9), then P2 (T6, T8, T10).
1. Add the `e2e` job to CI (non-blocking) after the suite runs stably green locally + in 1 trial PR.
1. `detect_changes()` before committing (per hpay-admin's CLAUDE.md).
1. After ~1–2 weeks of trial with no flake → enable the required status check.

HPay · plan generated by the `gen-feature-plan` skill · repo hpay-admin · 2026-07-29
