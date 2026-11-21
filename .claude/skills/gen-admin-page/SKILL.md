---
name: gen-admin-page
description: Scaffold a new hpay-admin page (queue/table + optional detail) following project conventions. Trigger when the user asks to "add the X page", "build the Y screen", or when implementing a mockup A1-A10 that still renders a Placeholder.
---

# gen-admin-page — Route + table/detail page matching the ops conventions

## Project conventions (MUST follow)

- Pages live in `src/pages/<Name>Page.tsx`, lazy-registered in `src/router.tsx`
  under the `RequireAdmin` + `Layout` branch. Sidebar entry goes in
  `src/ui/Sidebar.tsx` (`NAV_ITEMS`) and the title map in `src/ui/Layout.tsx`.
- UI is built ONLY from `src/ui/*` primitives (`Table`, `Badge`, `Button`, `Input`,
  `Modal`). Need something new (Tabs, Timeline, JSON viewer, Toast)? Add it to
  `src/ui/` first — no one-off styled divs copy-pasted between pages.
- Server state: fetch via hooks in `src/features/<domain>/hooks.ts`
  (see `gen-feature-hook`). Pages never call `api()` from `src/api/client.ts` directly.
- Filters/tabs/pagination state lives in **URL search params** (useSearchParams) —
  a re-login after 401 must restore the exact view (edge A03). No filter state in
  Zustand or component state that a reload would lose.
- Money is displayed from string minor units — never `Number()` on an amount.
- Status chips: `<Badge tone=...>` with a per-domain `toneFor(status)` helper next to
  the hook — never pick colors inline in a page.
- Errors: show `err.message` only if `err instanceof ApiError` (already mapped to
  Vietnamese in `src/api/errors.ts`); otherwise a generic message. A 409 means "someone
  else already handled it" (edge A01): toast + update the row in place, don't reload
  the whole table.
- Destructive/irreversible actions (reject, lock, force-fail, disable key) require a
  confirm `Modal` with a reason field validated to ≥ 10 characters.
- RBAC: gate action buttons with `useAdminStore().can('<permission>')` — hidden means
  hidden from UX only; the backend still enforces (never rely on the FE check).
- Auto-refresh (workflow 10s / fraud 30s) uses TanStack Query `refetchInterval`,
  not setInterval + manual fetch.
- Every table page needs loading skeleton, empty state, and error state.

## Steps

1. Read the mockup (`hpay-docs/design/mockups/`) and the phase's approved plan in
   `docs/plans/<phase>/` — the page must match the plan; divergence goes back into
   the plan file in the same commit.
2. Confirm every endpoint exists in `hpay-backend/api/openapi.yaml`. Missing → STOP,
   contract-first: propose the backend + openapi.yaml change before building UI.
3. Create/extend the domain hooks (`gen-feature-hook`), then compose the page from
   `src/ui` primitives.
4. Replace the `Placeholder` usage for this route; delete it from the page file.
5. `npm run typecheck && npm run lint && npm run build`, then verify by hand in the
   dev server (login → navigate → exercise the main action + one failure branch).

## Checklist before finishing

- [ ] Filters/tabs live in URL search params, restored after re-login
- [ ] 409 handled as row-update-in-place + toast (A01), not full reload
- [ ] Confirm modal + reason ≥ 10 chars on destructive actions
- [ ] Action buttons wrapped in `can()` checks
- [ ] Loading / empty / error states present
- [ ] Plan file updated if implementation diverged
