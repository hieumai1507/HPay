---
title: Admin app (src/admin)
type: architecture
status: verified-against-code
last_verified: 2026-11-21
code: [src/admin/package.json, src/admin/src/router.tsx, src/admin/src/features/auth/RequireAdmin.tsx, src/admin/src/stores/admin.ts, src/admin/src/api/client.ts, src/admin/src/queries, src/admin/src/pages, src/admin/vite.config.ts, src/admin/e2e]
---

# Admin app

## Tech stack (package.json)

React 18.3, react-router-dom 7, Vite 5, TypeScript 5.9, TanStack Query 5 and Table 8, zustand 5, Tailwind 3 (plus `src/ui/tokens.css`), lucide-react icons. Dev tools: eslint 9, Playwright 1.62 (e2e), openapi-typescript 7 (`npm run gen:api`). Scripts: `dev`, `build` (`tsc --noEmit && vite build`), `typecheck`, `lint`, `test` (vitest), `test:e2e`. Unit tests (fixed in working tree, 2026-11-21, not yet committed): vitest 2 + jsdom + @testing-library/react, files `src/**/*.test.ts(x)` (api client, money, env/idempotency, route guard); config in `vitest.config.ts`; they need no backend or DB. End-to-end tests are Playwright specs in `e2e/` (audit, dashboard, edge-cases, fraud, kyc, login, my-tasks, orgs, recon, workflow), run in CI by `admin-ci.yml`. The e2e admin password comes from `E2E_ADMIN_PASSWORD` (default is the backend seed value, e2e folder only).

Dev server on 5173 and preview on 4173 both proxy `/v1` and `/healthz` to `http://localhost:8080` (`vite.config.ts`). `VITE_API_URL` overrides the base URL (`api/client.ts:4`).

## Folder structure (`src/admin/src`)

| Folder | Content |
|---|---|
| `main.tsx`, `App.tsx`, `router.tsx` | bootstrap, providers, route table |
| `api/` | `client.ts` (fetch wrapper), `errors.ts` (ApiError + Vietnamese messages), `schema.d.ts` (generated types) |
| `lib/` | `access.ts` (route and action permissions), `money.ts` (integer minor-unit parsing and formatting), `env.ts` (mock policy), `idempotency.ts` (Idempotency-Key) |
| `mocks/` | demo data, loaded only through guarded dynamic imports (see Mock data policy) |
| `features/` | `auth/LoginPage`, `auth/RequireAdmin`, `auth/RequirePermission`, `fraud/ResolveDialog`, `services/ServicePipelineModal` |
| `pages/` | 14 lazy-loaded pages + `ErrorPage` (`Placeholder` was dead code and was removed) |
| `queries/` | one TanStack Query hook module per backend area (`useKYCQueries`, `useFraudQueries`, `useWorkflowQueries`, `useServiceQueries`, `useOffersQueries`, `useOrgsQueries`, `useReconQueries`, `useAuditQueries`, `useApprovalQueries`, `useAIQueries`) and `common.ts` (query-string builder, `Paged<T>`) |
| `stores/` | `admin.ts` (session + permissions), `toast.ts` |
| `components/table/` | `DataTable`, `DataTablePagination` (TanStack Table) |
| `hooks/` | `useTableQueryState` (pagination/sorting to query params) |
| `ui/` | Layout, Sidebar, Topbar, Modal, Badge, Button, Input, Table, Tabs, Timeline, BarChart, Toasts, Logo |

## Routing and guards (`router.tsx`)

`createBrowserRouter`: `/login` (public) and `/` wrapped in `<RequireAdmin><Layout/></RequireAdmin>` with `errorElement`; `*` renders a 404 `ErrorPage`. Children (each lazy-loaded with `Suspense`): `dashboard`, `kyc`, `kyc/:id`, `fraud`, `workflows`, `workflows/:id`, `tasks`, `recon`, `services`, `offers`, `offers/:id`, `orgs`, `ai-settings`, `audit`; index redirects to `/dashboard`.

`RequireAdmin` (`features/auth/RequireAdmin.tsx`) only checks that a token exists in the store; otherwise `Navigate` to `/login` with `state.from` (path + search, preserving filters). It does not check token expiry or the backend.

**Route guards (fixed in working tree, 2026-11-21, not yet committed).** `RequirePermission` (`features/auth/RequirePermission.tsx`) wraps the pages `/fraud`, `/recon`, `/services`, `/offers`, `/offers/:id`, `/orgs`, `/ai-settings` and `/audit`: it renders the page when the admin holds ANY permission listed for that route in `lib/access.ts` `ROUTE_ACCESS`, otherwise a visible "No access" panel. The sidebar hides the same links. Action buttons (fraud resolve, run recon, org policy save, AI key/routing writes, service status) are disabled with a visible reason when the matching `can()` permission is missing, and while their mutation is pending. The matrix is in [roles-and-permissions.md](../product/roles-and-permissions.md).

**This is UX only, not a security boundary.** The backend has no role enforcement yet (audit BE-H6, to be fixed separately): any authenticated admin can call every `/v1/admin/*` route directly, whatever the UI shows. Do not describe these guards as security controls.

## State

- `stores/admin.ts`: zustand store `{token, admin, setSession, clearSession, can}`. Persisted in `sessionStorage` key `hpay-admin-session` (dies with the tab). `AdminRole` = `viewer | kyc_reviewer | risk_officer | ops | superadmin`, matching the DB enum. `can(permission)` is a static role to permission map (`kyc.review`, `fraud.resolve`, `workflow.action`, `recon.run`, `orgs.edit`, `ai.manage`, `admins.manage`); the file states it is UX-only. It is consulted by the route guards, the sidebar and the action buttons listed under Routing and guards (previously only KYC detail and My Tasks used it).
- Server state: TanStack Query. Some hooks poll (workflows every 10 s, fraud flags every 30 s).
- `stores/toast.ts`: toast queue.

## Data layer

`api/client.ts` exports `api<T>(path, {method, body, headers, signal})` (alias `apiRequest`). It adds `Authorization: Bearer <admin token>`, JSON-encodes the body, treats 204 as `undefined`, and on non-2xx throws `ApiError(status, code, message, payload)` using the backend `{code, message}` envelope. A 401 clears the session (the guard then redirects to login). A response that is not JSON (an HTML 502 from a proxy, a captive portal, a malformed body) is now an `ApiError` with code `ERR_NON_JSON_RESPONSE` and a clear message, never `res.text()` cast to `T`; non-JSON error bodies are not exposed as `payload`. There is no refresh flow for admin (the backend has none). `schema.d.ts` is generated by `npm run gen:api` (`scripts/gen-api.ts`) from `src/backend/api/openapi.yaml` (the legacy `hpay-backend` path was fixed; it runs offline). The hooks hand-write their own response types today and do not import `schema.d.ts` yet.

## Pages and the backend endpoint each uses

"Real" means the endpoint exists in `router.go`. "Drift" means the hook calls a path the router does not have.

| Route | Page | Hook (file) | Endpoint(s) | Status |
|---|---|---|---|---|
| `/login` | LoginPage | inline `api` | `POST /v1/admin/auth/login` | real |
| `/dashboard` | DashboardPage | `useKYCQueue('manual')`, `useFraudFlags('open')` | `GET /v1/admin/kyc`, `GET /v1/admin/fraud-flags` | queues real; no analytics API, so KPI cards and the volume chart show an em dash (mock only with `VITE_USE_MOCKS=true` in dev); a failed count shows an em dash |
| `/kyc` | KYCPage | `useKYCQueue` (`useKYCQueries.ts:55`) | `GET /v1/admin/kyc` | real |
| `/kyc/:id` | KYCDetailPage | `useKYCQueue`, `useKYCReview` | `GET /v1/admin/kyc`, `POST /v1/admin/kyc/:id/review` body `{action, reason}` | real (matches `kycReviewReq`) |
| `/fraud` | FraudPage + ResolveDialog | `useFraudFlags`, `useResolveFraudFlag` | `GET /v1/admin/fraud-flags`, `POST /v1/admin/fraud-flags/:id/resolve` | real. The backend row has no amount or user: both are `null` and show an em dash (the old mapper invented 5,000,000 and `User xxxx`); errors are shown, not turned into an empty list |
| `/workflows`, `/workflows/:id` | WorkflowPage, WorkflowDetailPage | `useWorkflowInstances`, `useWorkflowDetail`, a decide mutation | `GET /v1/admin/workflows`, `GET /v1/admin/workflows/:id`, `POST /v1/admin/approvals/:id/decide` | real. Errors are shown (the list no longer turns a failure into an empty page) |
| `/tasks` | MyTasksPage | `useAdminApprovals`, `useDecideApproval` | `GET /v1/admin/approvals`, `POST /v1/admin/approvals/:id/decide` | real |
| `/recon` | ReconPage | `useReconRun(date)`, `useRunRecon` | `GET /v1/admin/recon/runs/:date`, `POST /v1/admin/recon/run` | real. Errors are shown; numbers the backend omits are `null` (em dash); `ERR_RECON_NOT_FOUND` is an empty state; mock run only with `VITE_USE_MOCKS=true` in dev |
| `/services` | ServiceCatalogPage + ServicePipelineModal | `useServices`, `useUpdateServiceStatus`, `useCreateService` | `GET /v1/admin/services`, `PUT /v1/admin/services/:id`, `POST /v1/admin/services` | list and status update real; **create removed from the hooks and disabled in the UI with a visible reason: backend gap, no `POST /v1/admin/services`**; pipeline-modal save disabled (no endpoint) |
| `/offers` | OfferFactoryPage | `useOffers` | `GET /v1/admin/offers` | real |
| `/offers/:id` | OfferDetailPage | `useOfferDetail`, `useSaveOffer` | `GET /v1/admin/offers/:id` | read real; **save removed and disabled with a visible reason: backend gap, no write routes for offers**; limit inputs are validated integers; the fabricated default services and editor constants were removed |
| `/orgs` | OrgsPage | `useOrgs`, `useSaveOrgPolicy` | `GET /v1/admin/orgs`, `PUT /v1/admin/orgs/:id/policies` | real; errors are shown (mock orgs only with `VITE_USE_MOCKS=true` in dev); the real list has no policy bodies |
| `/ai-settings` | AISettingsPage | `useAIKeys`, `useAISettings`, create/update mutations | `GET/POST /v1/admin/ai/keys`, `PUT /v1/admin/ai/keys/:id`, `GET/PUT /v1/admin/ai/settings` | real. Reads show errors; writes reject on failure (the fabricated success object and fake keys were removed); mock keys only with `VITE_USE_MOCKS=true` in dev |
| `/audit` | AuditPage | `useAuditQueries` | `GET /v1/admin/audit?name=...` | real; errors are shown (demo rows only with `VITE_USE_MOCKS=true` in dev) |

The dead duplicate hooks that called paths the backend does not serve were removed (fixed in working tree, 2026-11-21, not yet committed): `useFraudFlagsQuery`, `useResolveFraudFlagMutation` (`/v1/admin/fraud/flags`), `useKYCQueueQuery`, `useKYCDecisionMutation` (`/v1/admin/kyc/:id/decision`), `useUpdateServiceStatusMutation` (`/services/:id/status`), `useServicesQuery`, `useCreateService*` (`POST /v1/admin/services`) and `useSaveOffer` (`PUT/POST /v1/admin/offers/:id`). Each live hook uses the real route and request body read from the backend handler (`src/backend/internal/api/admin_ops_handler.go`). `ServicePipelineModal.tsx:66` contains a sample partner URL string (`https://api.bankpartner.vn/v1/p2p-notify`) that is display-only sample data and is never sent; there is no `/v1/p2p-notify` call in the app.

**Backend gaps found (UI disabled with a visible reason):** `POST /v1/admin/services` (create service), `PUT/POST /v1/admin/offers/:id` (edit offer), and an endpoint to save a service's transaction design. The admin routes also ignore the Idempotency-Key header (backend C-03 gap).

Backend routes with no admin UI: `POST /v1/admin/reversals` (create reversal) has no page; reversals can only be seen as approvals in My Tasks. `GET /v1/files/*key` is used implicitly through signed URLs in KYC and recon payloads.

## Auth and session handling

Login posts email and password and stores `{access_token, admin}` in the store/sessionStorage (`LoginPage.tsx:43-49`). The admin JWT lives 15 minutes (same `AccessTokenTTL`), there is no refresh and no silent re-login: after expiry the next call returns 401, the store clears and the guard redirects to login. Logout is local only (no endpoint).

## Mock data policy (fixed in working tree, 2026-11-21, not yet committed)

Mock/demo data is available ONLY when `import.meta.env.DEV` is true AND `VITE_USE_MOCKS=true` is set explicitly (`lib/env.ts` `MOCKS_ENABLED`). The demo data lives in `src/mocks/*` and is loaded through dynamic imports guarded by that condition, so a production build contains none of it (verified: no mock strings in `dist/`). In every other case:

- a failed read shows the real error (`ErrorNotice`), never demo numbers or an empty list that looks like "no data";
- a number the backend did not send renders as an em dash, never as a default such as 2173 or 4200 ms;
- a failed write rejects and the page shows the failure; no hook swallows errors with an empty `catch {}` or returns a fabricated success;
- the dashboard KPI cards and the hourly chart have no analytics API, so in production they show an em dash.

Known remaining: the KYC and My Tasks hooks never had fallbacks. `FraudPage` still filters by rule client-side on the loaded page.

## Writes and idempotency (fixed in working tree, 2026-11-21, not yet committed)

Every admin mutation (workflow retry/compensate and approval decide, fraud resolve, recon run, KYC review, org policy save, service status, AI key create/update, AI routing update) generates one `Idempotency-Key` per user action (`lib/idempotency.ts`, one key per `mutate()` call) and passes it through `api()`'s `headers`. The buttons disable while the mutation is pending and the handlers return early when a call is in flight. **The backend does not read this header on any `/v1/admin` route yet**, so this is client readiness only; it does not make a retried request idempotent until the backend binds keys per admin and payload (constitution C-03).

## Money in the UI (fixed in working tree, 2026-11-21, not yet committed)

Amounts are integer minor units (constitution C-01). `lib/money.ts` parses operator input with a digits-only grammar and an upper bound (`MAX_MINOR_UNITS` = 1,000,000,000,000; interval seconds <= 86,400), rejecting empty, negative, decimal, exponent, NaN and oversized text, and formats with one shared `formatVnd`/`formatCount`. There is no `Number()` on user text and no float division for money (org tier thresholds use `formatMillions`, integer only).

## Content-Security-Policy (fixed in working tree, 2026-11-21, not yet committed)

`vite.config.ts` injects a CSP `<meta>` into `index.html` for the production build only (the dev server needs inline scripts and a websocket for HMR): `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com data:; img-src 'self' data: blob: https:; connect-src 'self' [+ origin of VITE_API_URL]; object-src 'none'; base-uri 'self'; form-action 'self'`. `'unsafe-inline'` is needed for styles only (React inline style attributes). `img-src https:` allows the signed KYC image links; tighten it to the storage origin once that is fixed. `frame-ancestors` cannot be set from a meta tag: send it as a response header from the static host. Verified: `npm run build` output contains the tag and `vite preview` serves the app (HTTP 200). Not verified in a real browser against a running backend.
