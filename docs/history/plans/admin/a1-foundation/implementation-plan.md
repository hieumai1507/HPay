# Implementation Plan — A1 Foundation hpay-admin

Initial phase that sets up the whole codebase for **HPay Admin (hpay-admin)** — an SPA for internal operations staff. It connects directly to the `/v1/admin/*` API (JWT audience=admin). Built with **Vite + React 18 + TypeScript + React Router v7**, designed according to mockups A1–A10.

## 1. Scope & non-goals

### Ships in A1

- **Vite scaffold**: initialize the project with TypeScript and React. TailwindCSS is *not* used — vanilla CSS tokens are used, the same as mobile, for design consistency.
- **Design system**: `src/ui/tokens.css` — CSS variables from the design spec (brand colors #0E90A3/#1ECBE1, dark sidebar #0B1220, etc.). Components: `Button`, `Badge`, `Table`, `Sidebar`, `Topbar`.
- **Layout shell**: Sidebar (collapsible) + Topbar + main content area — used by every page.
- **Routing**: React Router v7 with lazy routes and a protected route wrapper (`RequireAdmin` checks the JWT).
- **Auth flow**: The `/login` page calls `POST /v1/admin/auth/login` → stores the JWT in `sessionStorage` (not persisted across tabs — admin does not remember sessions). Auto-refresh is not needed: admin is logged out when the tab is closed.
- **API client**: typed fetch wrapper (`src/api/client.ts`) that automatically injects the Authorization header from the store and maps error codes (`src/api/errors.ts`). Types are generated from `openapi.yaml` via `npm run gen:api`.
- **RBAC store**: Zustand store `useAdminStore` holds `{token, role}` and exposes a `can()` helper that checks permissions (`risk_officer`, `kyc_reviewer`, `superadmin`).
- **Placeholder pages**: Dashboard (static numbers), KYC, Fraud, Workflow, Recon, Orgs, Audit — they only render the shell; real data will be filled in A2-A4.
- Global **404 / Error boundary**.

### Non-goals A1

- Real data from the API (done in A2-A4).
- Real charts/graphs (A2).
- Dark mode toggle (the sidebar is already dark, the main area is light — as in the mockup).

## 2. Stack

| Layer | Technology | Reason |
|---|---|---|
| Build | Vite 5 + TypeScript 5 | Fast HMR, native ESM |
| UI | React 18 + Vanilla CSS | Consistent with mobile (no Tailwind — as decided in the master-plan) |
| Routing | React Router v7 (Data Router) | Lazy loading, nested routes, loader/action pattern |
| State | Zustand 4 | Same as mobile |
| Data fetching | TanStack Query v5 | Same as mobile; cache + background refetch |
| Icons | Lucide React | Tree-shakeable, consistent stroke style |
| Type gen | ts-node script (same as mobile) | Reuse `gen-api.ts` from mobile, pointed at the same openapi.yaml |

## 3. Directory structure

```
src/admin/
├── index.html
├── vite.config.ts
├── tsconfig.json
├── package.json
├── scripts/
│   ├── gen-api.ts          # copied from mobile, points to ../hpay-backend/api/openapi.yaml
│   └── tsconfig.json
├── src/
│   ├── api/
│   │   ├── client.ts       # fetch wrapper with auth header
│   │   ├── schema.d.ts     # generated types
│   │   └── errors.ts       # error code → Vietnamese message
│   ├── stores/
│   │   └── admin.ts        # useAdminStore: token + role + can()
│   ├── ui/
│   │   ├── tokens.css      # CSS variables from the design spec
│   │   ├── Layout.tsx      # Sidebar + Topbar shell
│   │   ├── Sidebar.tsx
│   │   ├── Topbar.tsx
│   │   ├── Table.tsx       # generic sortable table component
│   │   ├── Badge.tsx       # status chip (success/warning/danger/info/neutral)
│   │   ├── Button.tsx
│   │   └── index.ts
│   ├── features/
│   │   └── auth/
│   │       └── LoginPage.tsx
│   ├── pages/              # placeholder pages (filled A2-A4)
│   │   ├── DashboardPage.tsx
│   │   ├── KYCPage.tsx
│   │   ├── FraudPage.tsx
│   │   ├── WorkflowPage.tsx
│   │   ├── ReconPage.tsx
│   │   ├── OrgsPage.tsx
│   │   └── AuditPage.tsx
│   ├── router.tsx          # React Router config with lazy imports
│   ├── App.tsx
│   └── main.tsx
└── docs/
    └── plans/
        ├── a1-foundation/  # this document
        ├── a2-dashboard-kyc/
        ├── a3-workflow-fraud/
        └── a4-recon-orgs-audit/

```

## 4. Tasks

| # | Task | Main files | Size | Depends |
|---|---|---|---|---|
| T1 | Scaffold Vite + TS + React 18, configure `vite.config.ts`, `tsconfig.json`, path alias `@/` | `vite.config.ts`, `package.json` | S | — |
| T2 | Copy `gen-api.ts` from mobile, run `npm run gen:api` to generate `schema.d.ts` | `scripts/`, `src/api/schema.d.ts` | S | T1 |
| T3 | API client + error map (`client.ts` fetch wrapper that injects the JWT, `errors.ts`) | `src/api/` | S | T2 |
| T4 | Zustand store `useAdminStore`: `token`, `role`, `can(permission)` | `src/stores/admin.ts` | S | — |
| T5 | CSS design tokens (`tokens.css`) + reset; import in `main.tsx` | `src/ui/tokens.css` | S | — |
| T6 | UI components: `Sidebar`, `Topbar`, `Layout` (shell exactly as in the mockup — dark sidebar, light main area, badge count) | `src/ui/` | M | T5 |
| T7 | Generic `Table` component: column config, sortable header, row click, pagination props | `src/ui/Table.tsx` | M | T5 |
| T8 | `Badge`, `Button`, shared primitives (Input, Modal shell) | `src/ui/` | S | T5 |
| T9 | Auth: `LoginPage` → `POST /v1/admin/auth/login` → set store → redirect. `RequireAdmin` HOC checks the token. | `src/features/auth/` | M | T3,T4 |
| T10 | React Router config: lazy routes, `RequireAdmin` wrapper, 404, Error boundary | `src/router.tsx`, `src/App.tsx` | S | T9 |
| T11 | Placeholder pages (shell layout only — title + skeleton): Dashboard, KYC, Fraud, Workflow, Recon, Orgs, Audit | `src/pages/` | S | T6,T10 |
| T12 | Typecheck + ESLint config + verify a clean build (`npm run build`) | `.eslintrc` | S | T11 |

## 5. Architecture flow

```mermaid
flowchart LR
  browser[Browser] --> login[/login]
  login --> |POST /v1/admin/auth/login| api[Backend API]
  api --> |JWT aud=admin| store[useAdminStore\ntoken + role]
  store --> shell[Layout Shell\nSidebar + Topbar]
  shell --> dash[Dashboard]
  shell --> kyc[KYC Queue]
  shell --> fraud[Fraud Monitor]
  shell --> wf[Workflow Monitor]
  shell --> recon[Reconciliation]
  shell --> orgs[Orgs & Policy]
  shell --> audit[Audit Log]
```

## 6. Key design tokens

**Palette from the mockup (dark sidebar):**
`--sidebar-bg: #0B1220` · `--sidebar-text: #94A3B8` · `--sidebar-active: #1ECBE1`
`--brand: #0E90A3` · `--brand-600: #0B7181` (button pressed)
`--main-bg: #F9FAFB` · `--card-bg: #FFFFFF`
Badge colors reused from mobile: success `#16A34A`, warning `#D97706`, danger `#DC2626`, info `#0891B2`

## 7. Verification

```
npm run typecheck    # tsc --noEmit
npm run lint         # eslint .
npm run build        # vite build — no warnings
npx vite preview     # manual check: login → view sidebar → navigate pages
```
