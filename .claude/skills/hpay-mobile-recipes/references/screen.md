# Scaffold a mobile expo-router screen

> Trigger: Scaffold a new expo-router screen for hpay-mobile following project conventions. Trigger when the user asks to "add a screen", "create the X screen", or when implementing a mockup/frame from ui-design-spec.html that doesn't have a route yet.

# gen-screen — Generate a screen that matches the design spec + client rules

## Project conventions (MUST follow)

- Route lives under `app/` using expo-router file-based routing — group by the same
  route group used in `docs/architecture/02-mobile-solution-architecture.html`
  §2/§3 (`(auth)`, `(kyc)`, `(main)`, `transfer/`, `topup/`, `org/`).
- UI is built ONLY from `src/ui/*` primitives (`Button`, `AmountDisplay`, `StatusBadge`,
  design tokens in `src/ui/tokens.ts`). If the mockup needs a component that doesn't
  exist yet, add it to `src/ui/` first — don't hand-roll one-off styled views in a screen file.
- Money: any amount shown or entered goes through `src/lib/money.ts`
  (`parseAmountToBigInt`/`formatAmountWhileTyping`/`formatAmountDisplay`). Never
  `Number(amountString)`, never `parseFloat`.
- Status text/color: use `<StatusBadge status="...">` — never invent a new status
  string or pick a color by hand (doc 02 §6 UX rule).
- Server state: fetch via a hook from `src/features/<domain>/` (see `gen-feature-hook`
  skill) built on TanStack Query + the generated `src/api/schema.d.ts` types. Screens
  do not call `apiRequest` directly.
- Errors: render `describeError(err)` from `src/lib/errors.ts`. Never a raw
  `err.message` or a hardcoded string.
- Money-moving actions (transfer/topup/pay-request confirm): the confirm screen must
  call `useIdempotencyKey(resetKey)` once on mount and pass that key through to the
  mutation — never generate a new key per tap (M01).
- Async transaction results: use `PollingTransactionWatcher`/`pollUntilSettled` from
  `src/lib/polling.ts` — timeout must show "đang xử lý", never "thất bại" (M02/M03).
- Every list screen needs Skeleton/Empty/Error states (ui-design-spec.html §6).

## Steps

1. Read the relevant mockup section in `docs/design/mockups/*.html` and
   the matching row in `ui-design-spec.html` §4/§5 (frame name, components used).
2. Confirm the backend contract exists for any data the screen needs
   (`hpay-backend/api/openapi.yaml`). If it doesn't → STOP, this is a
   contract-first violation; propose the backend/openapi change first.
3. Create the route file(s) under `app/<group>/<screen>.tsx`. Keep the component a
   thin composition of `src/ui` primitives + one `src/features/<domain>` hook.
4. If the screen needs local UI-only state (form draft, step index) — `useState`/
   `useReducer` in the component. Do NOT put server data in Zustand (doc 02 §1: "chỉ
   giữ session + UI state, không nhét server data").
5. Add a component test under `app/**/__tests__/` or co-located `*.test.tsx` covering
   the screen's edge case from the M01–M12 matrix if it owns one (see doc 02 §4).
6. Run `npm run typecheck && npm run lint && npm test`.

## Checklist before finishing

- [ ] No raw `fetch`/`Number(amount)`/hardcoded status color in the screen file
- [ ] Idempotency key generated once per confirm flow, not per tap (if money-moving)
- [ ] Loading/Empty/Error states present for any list or async data
- [ ] Screen added to the correct expo-router group (check it appears in the right tab/stack)
- [ ] `docs/plans/<phase>/` updated if the screen diverged from the approved plan
