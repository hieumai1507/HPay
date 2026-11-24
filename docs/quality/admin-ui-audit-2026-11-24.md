# Admin UI audit, 2026-11-24

Scope: full pass over the Admin console (http://localhost:5173) against the seeded demo database `hpay_demo`, roles superadmin (e2e-checker) and ops (e2e-maker). No application code was changed and no data was written. Method: Playwright walkthrough (`admin-ui-audit/walkthrough.mjs`, 4 viewports x 2 roles, 104+ page dumps, console/network capture) plus manual probes, compared with API responses and `psql` queries.

Environment note (important): the running API container `hpay-api` was started at 12:31 UTC (`go run`) and does NOT serve `GET /v1/admin/orgs/:id/policies`, although `src/backend/internal/api/router.go:246` registers it. AUD-003 is therefore probably a stale API container, not a code defect; restart the API and re-run before fixing anything there.

## Defects (sorted by severity)

| ID | Severity | Page | Role | What you did | What happened | What should happen | Evidence | Suggested cause |
|---|---|---|---|---|---|---|---|---|
| AUD-001 | blocker | Offer Factory > offer detail (all 5 offers) | superadmin, ops-blocked | Open Offer Factory, click Configure on any offer (or deep link /offers/b0000000-0000-0000-0000-000000000001). | Every limit input shows the text `undefined`. All 5 service cards show a blank name and ` · Configured`. The authentication matrix is empty. Clicking a service changes nothing. Console: React warning "Each child in a list should have a unique key" (OfferDetailPage) because `serviceCode` is undefined for every service. | Service names (Request money & split bill, ...), codes, limits (e.g. 62,500,000 / 125,000,000 / 625,000,000) and the 3-tier auth matrix (pin / otp / 2fa) from offer_auth_matrix. | AUD-001-offer-detail-undefined.png. API `GET /v1/admin/offers/{id}` returns `services[]` with `service_code, service_name, max_amount_per_txn, daily_limit, monthly_limit, interval_seconds` (snake_case) and no auth matrix; DB has 60 offer_auth_matrix rows. | `queries/useOffersQueries.ts:55-59` returns the raw API rows typed as camelCase `OfferServiceConfig` with no mapping; the API also does not return the auth matrix per service (backend gap or separate call needed). `OfferDetailPage.tsx:62-69` then does `String(undefined)`. |
| AUD-002 | major | Offer Factory list | superadmin | Open /offers. | Every plan is badged `Draft` although `is_active` is true for all 5 in the API. Columns "Authorized services" and "Updated by" are empty although the API returns `authorized_services_count: 5` and `updated_by` (e.g. policy-2026-11-21, hieumt). | Status Active; 5 services; updater shown. | AUD-002-offer-list-blank-columns.png; API `GET /v1/admin/offers`. | `useOffers` (`useOffersQueries.ts:42-46`) does not map snake_case (`is_active`, `updated_by`, `authorized_services_count`) to the camelCase fields the page reads. Same root cause as AUD-001. |
| AUD-003 | major | Organizations & Policy | superadmin, ops | Open /orgs. | Panel shows "Could not load the approval policies: Request failed with HTTP 404 (response was not JSON)." `GET /v1/admin/orgs/<id>/policies` returns 404 from the API. The wording mentions "response was not JSON", which is developer wording. Edit/start-discard could not be exercised. | List the active policy "Large spend" (min 10,000,000, 2 tiers x 1 approval, v3) and allow starting an edit. | AUD-003-orgs-policies-404.png; DB approval_policies has 1 row. | Likely stale `hpay-api` container (see environment note). Separately, the generic 404 message in `api/client` should not say "response was not JSON". Re-verify after restart. |
| AUD-004 | major | Reconciliation | superadmin, ops | Open /recon. | Page opens on a hardcoded date "Reconciliation for 2026-07-07" and requests `GET /v1/admin/recon/runs/2026-07-07`, which returns 404 (console error on every visit). Empty table says "Nothing here yet". | Default to today (or the latest run) and show a clear "no run for this date" state without a failed request. | AUD-004-recon-hardcoded-date.png. | `pages/ReconPage.tsx:18` `useState('2026-07-07')`. |
| AUD-005 | major | Service Catalog | superadmin | Open /services, compare the "Action Pipeline" column with the modal. | Every service row says "Low-Code Pipeline (2 steps)". Modal for VAULT_SAVINGS, MONEY_REQUEST, ORG_DISBURSEMENT, TOPUP_BANK says "Action Pipeline (0 steps)" / "No actions configured". Only P2P_TRANSFER really has 2. | Row count equals the modal count (0 for four services). | AUD-005-service-steps-fallback.png; API `actions` is base64 `W10=` (= `[]`) for those four. | `pages/ServiceCatalogPage.tsx:126` `actions?.length \|\| 2` (an empty array is falsy-length 0 so the fallback 2 is shown). |
| AUD-006 | major | Workflow detail | superadmin | Open /workflows, open the failed instance a177692f... and the waiting_approval one. | Step outputs are shown as raw base64 (`out: e30=`, `eyJjb250ZXh0Ijo...`), the INPUT/OUTPUT panel shows a base64 string in quotes, Transaction and Amount show "—" although the list row shows MP4B76AC717DD9 and 200,000 ₫, Workflow type shows the word "workflow", the header reads "<uuid> · workflow". Three identical `CONFIRM err: no rows in result set` rows (retries, raw DB error text shown to operator). | Decoded JSON, the same transaction code and amount as the list, the definition name (p2p_transfer v3), operator-friendly error text. | AUD-006-workflow-detail-base64.png. | JSON columns returned base64 by the API are not decoded in `queries/useWorkflowQueries.ts` / `WorkflowDetailPage.tsx`; detail does not read ref_code/amount. (Same base64 encoding of jsonb exists on /services, which is decoded by `decodeJsonColumn`; reuse it.) |
| AUD-007 | major | All pages | both | Open any page at 390x844. | Sidebar stays 220 px wide, leaving about 170 px for content: title wraps to 5 lines, search box and warning banner are cut off at the right edge, table columns are off-screen. Page body itself does not scroll horizontally (scrollWidth 390). | Collapsed or drawer navigation on phone widths; content uses the full width. | AUD-007-phone-sidebar-takes-half.png (also m_* probes for dashboard, services, offers, workflows, orgs: same). | `ui/Sidebar.tsx:44-49` (fixed `w-58`, `collapsed` only toggled by hand, no breakpoint logic); `ui/Layout.tsx:25-26`. |
| AUD-008 | major | Dashboard | ops | Log in as ops, open /dashboard. | Cards "Pending KYC" and "Open Fraud flags" show "Could not load this count" and the browser logs two 403s (`/v1/admin/kyc?tab=manual`, `/v1/admin/fraud-flags?status=open`). The Needs attention card shows "—". | Hide or show "No access" for cards the role cannot use, and do not call forbidden endpoints. | AUD-008-ops-dashboard-could-not-load.png. | `pages/DashboardPage.tsx` queries are not gated by permission (compare `Sidebar.tsx` which uses `canAccessRoute`). |
| AUD-009 | major | KYC Review | ops | Log in as ops, open /kyc. | Page renders the tabs and an empty table with the text "your role does not allow this action" and a 403 on `GET /v1/admin/kyc` (twice per visit, console errors). No route guard, unlike /fraud ("Your role (ops) cannot open this page..."). | Same guard message as /fraud, or hide the item in the sidebar for roles without `kyc.review`. Also /kyc detail for ops was not checked. | AUD-013-ops-kyc.png. | `router.tsx:50-51` `/kyc` and `/kyc/:id` are not wrapped in `guarded(...)`; `lib/access.ts` has no `/kyc` entry. |
| AUD-010 | minor | Fraud Monitor | superadmin | Open /fraud. | Page title reads "Fraud monitoring (TanStack Table v8)". | "Fraud monitoring". | AUD-009-fraud-dev-title.png | `pages/FraudPage.tsx:185`. |
| AUD-011 | minor | Dashboard | both | Open /dashboard. | "Workflows stuck > 5 min" card says "API coming in A3". The four KPI cards and chart say "(no analytics API yet)" / "no data yet". | Plain product wording or hide the cards that have no data source. The stuck-workflow count could use `/v1/admin/workflows`. | AUD-010-dashboard-dev-wording.png | `pages/DashboardPage.tsx:100`. |
| AUD-012 | minor | AI Settings | superadmin | Open /ai-settings. | Title "AI & model routing settings (F11)" (ticket code), routing cards show the raw value `mock` as a badge, provider option "Mock Engine (Dev / Offline)". | No internal ticket code; human label for the provider; mock choice hidden or flagged as demo-only. | AUD-011-ai-settings-copy.png | `pages/AISettingsPage.tsx:104,321`. |
| AUD-013 | minor | KYC Review | superadmin | Open /kyc and click the row. | Clicking the row does nothing; only the small "Open →" link and the name navigate. Shortcut text says "Enter open request" but no row is focused. List and detail show raw enum `manual_review` for AI Verdict / status. | Whole row navigates; labelled verdict ("Manual review"). | AUD-012-kyc-row-click.png | `pages/KYCPage.tsx:80,118` (only inner elements have onClick). |
| AUD-014 | minor | Workflow list | both | Open /workflows. | Statuses mix formats: `Failed`, `Completed` (capitalised) next to `waiting_approval` (raw enum). Completed rows show an empty "Current step" and ages like `1492m`. | One consistent label set ("Waiting approval") and ages like "1d 1h". | text dump `superadmin_1920x1080__workflows.txt` (from walkthrough output) | `pages/WorkflowPage.tsx` status badge mapping. |
| AUD-015 | minor | Organizations & Policy | both | Open /orgs. | "1 orders awaiting approval ..." (plural with 1). | "1 order awaits approval". | AUD-003-orgs-policies-404.png | `pages/OrgsPage.tsx` (grep "orders awaiting"). |

Note on AUD-007 and other layout checks: only horizontal overflow and the five screenshots above were examined at 390x844; 1024x768 only through the overflow test (no page produced horizontal page overflow at any viewport, `h-overflow` count 0).

## Coverage matrix (C = checked, P = partly, N = not checked)

Viewport columns: 1920 / 1440 / 1024 / 390. "C" for a viewport means the page opened with no crash, overflow test and text/console capture ran; visual review was done only where a screenshot is listed above.

| Page | superadmin | ops |
|---|---|---|
| Dashboard | C/C/C/C | C/C/C/C |
| KYC Review list | C/C/C/C | C/C/C/C (403, AUD-009) |
| KYC detail | P (1 row, 1440 only, manual) | N |
| Fraud Monitor | C/C/C/C | C (guard message) |
| Workflow list | C/C/C/C | C/C/C/C |
| Workflow detail (first 8 rows) | C/C/C/C (text dump); manual read of 2 | N (script covers it; not read) |
| My Tasks | C/C/C/C | C/C/C/C |
| Needs attention | C/C/C/C | C/C/C/C |
| Reconciliation | C/C/C/C | C/C/C/C |
| Service Catalog list | C/C/C/C | C (guard message) |
| Transaction Design Manager, 5 services x 5 tabs | C at 1440 (manual probe); script at other viewports ran but phone modal fit not reviewed | N/A (no access) |
| Offer Factory list | C/C/C/C | C (guard message) |
| Offer detail (5 offers) | C/C/C/C (broken, AUD-001) | N/A |
| Organizations & Policy | P (list only; policies 404, edit/discard N) | P |
| AI Settings | C/C/C/C | C (guard message) |
| Audit Log | C (empty: audit_logs has 0 rows in the demo DB) | C |
| 404 page | C | C |
| Login: wrong password, expired/bad token, token removed + reload, deep link logged out | C (manual, redirects to /login, error text correct) | N |
| Sorting, search, filters, page size, last page, no-result search | N (not exercised beyond default load) | N |
| Back/forward, focus visibility, keyboard beyond Escape | N (Escape closes the modal: C) | N |
| Maintenance toggle success path | N (not run; no write probes made) | N |

## Verified correct

- No error boundary, blank page or uncaught exception on any page, either role, any viewport. No page overflows horizontally at any of the 4 viewports.
- No Vietnamese text and no mojibake rendered on any page.
- Service modal (re-verified fix): each of the 5 services shows its own code, name and counts; P2P_TRANSFER is the only one with 2 validations and 2 steps; reopening in a different order shows each service's own data; Escape closes the modal.
- Fraud flag (VELOCITY_5MIN, score 87, "5 transactions in 5 minutes", AI advisory text) matches `GET /v1/admin/fraud-flags`; 1 open of 3 flags in the DB.
- KYC queue: 1 pending row (Tran Van Ba, 0900000003, score 64) matches the API; detail renders its own data.
- Workflows: 45 results; DB status counts (41 completed, 2 failed, 1 compensated, 1 waiting_approval) are consistent with the list. Amounts use comma grouping and the dong sign; dates use dd/mm/yyyy.
- Services list: all 5 services, categories, action priority labels and status match the API.
- Orgs list (Acme Corp, 4 members, 1 active policy) matches `GET /v1/admin/orgs`.
- Needs attention: summary 0 / 0 / 0 matches the API; badge hidden accordingly. My Tasks: 1 reversal, 500,000 ₫ L1.
- Wrong password shows "Incorrect email or password."; a bad or removed token sends the user to /login.
- Ops sees forbidden pages (/fraud, /services, /offers, /ai-settings) with a clear guard message and no data.

## Assumptions and gaps

- Dates: audit_logs is empty in the demo DB, so audit row correctness could not be compared.
- "Needs attention" badge count compared with the API summary, not with a DB query on failed compensation.
- The walkthrough's `no-login-redirect` finding in the first run was a timing race in the script (fixed with a wait); manual probe confirmed the redirect works, so it is not a defect.
- The walkthrough script flags candidates only; every defect above was reproduced manually or from page text dumps.