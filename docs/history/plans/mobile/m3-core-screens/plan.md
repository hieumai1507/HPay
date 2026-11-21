Implementation Plan — M3 Core Screens & Flows (hpay-mobile)

# 📋 Implementation Plan — M3 Core Screens & Flows mobile

Phase that builds all the main screens: Home (shows balance, quick actions, recent transactions), History (filter by type, cursor pagination), Topup (mock bank top-up), P2P Transfer (async transfer through the workflow engine), and Txn Detail (timeline to view transaction status).

Backend dependency: F2 (Auth) + F3 (Wallet/Topup) + F5 (Transfer/Fee) + F8 (KYC) — already completed.

## 1. Scope & non-goals

### Ships

- Home Screen:
  - Greeting header + user avatar (name taken from `session.user.full_name`).
  - Balance Card: `AmountDisplay` display size, show/hide button (👁) — HIDDEN by default when the app opens (`balanceVisible` Zustand store, persisted to MMKV/AsyncStorage).
  - 4 Quick actions: Top up, Transfer, QR, History — navigate accordingly.
  - "Recent transactions": the 5 latest `TxnListItem` taken from `GET /v1/transactions`, a "See all" button → History tab.
  - Pull-to-refresh refetches both balance + recent txns.
- History Screen:
  - Filter chips: All / Money in / Money out / Processing (client-side filter over the already-fetched data).
  - List grouped by day (section headers: "Today", "Yesterday", "12/07/2026").
  - Infinite scroll with cursor-based pagination (`next_cursor` + `next_cursor_id`) using React Query `useInfiniteQuery`.
  - Complete Skeleton / Empty / Error states (UX rule §6).
- Topup Flow (2 steps):
  - AmountInput screen: New `AmountInput` component (large number in the middle of the screen, thousands separators, dimmed ₫ unit on the right). Validate: > 0 and ≤ KYC limit (show the message "Exceeds limit").
  - Result screen: Calls `POST /v1/wallets/topup` (synchronous). Shows a success (✓) or error result. Uses `useIdempotencyKey` to protect against double-tap (M01).
- P2P Transfer Flow (4 steps):
  - Choose recipient: Enter phone number (debounce 500ms), look up the masked name when found. Show recent contacts (top 5 from history).
  - Enter amount: `AmountInput` + show available balance + a note field. Validate the KYC limit in real time. Call `GET /v1/fees/preview` to compute the fee.
  - Confirm: Summary (recipient, amount, fee, total). Warn if this is a first-time recipient. "Transfer" button (Loading variant, disabled immediately on press — M01). Calls `POST /v1/transactions/transfer` → 202 Accepted.
  - Processing → Result: After submit, go to a processing screen with a spinner + condensed timeline. Uses `PollingTransactionWatcher` (already present from M1) to poll `GET /v1/transactions/{refCode}/timeline`. Result has 3 variants: success (✓), failed (reason + retry button), timeout/processing ("The transaction is being processed, please check back later").
- Txn Detail Screen:
  - `AmountDisplay` + `StatusBadge` + `TimelineStep` list (public view).
  - `ref_code` shown in mono + a copy button.
  - Full transaction information: type, status, fee, note, time.
- New UI Components:
  - `AmountInput` — full-screen-style amount entry input, live-format using `money.ts`.
  - `TxnListItem` — a transaction item in a list (avatar/icon + name + note + amount + status).
  - `TimelineStep` — a step in the timeline (done/active/failed/waiting variants).
  - `FilterChips` — a group of chips to filter by transaction status/type.
  - `EmptyState` / `ErrorState` / `Skeleton` — the 3 mandatory states for every list.
  - `BalanceCard` — a card showing the balance, with a show/hide button.

### Non-goals

- QR for receiving money (M4) — the QR tab stays a placeholder.
- Request Money / Split Bill (M4).
- Business Wallet maker/checker flow (M5).
- Detailed Account screen (settings, change password, biometric toggle — M4/M5).

## 2. Data flow & API mapping

```mermaid
graph LR
  subgraph "React Query Hooks"
    A["useWallets()"] -->|GET /v1/wallets| B["{ wallets: Wallet[] }"]
    C["useTransactions()"] -->|GET /v1/transactions?cursor&cursor_id| D["{ transactions[], next_cursor, next_cursor_id }"]
    E["useFeePreview(amount)"] -->|GET /v1/fees/preview?amount| F["{ amount, fee, total }"]
    G["useTimeline(refCode)"] -->|GET /v1/transactions/{refCode}/timeline| H["{ ref_code, txn_status, timeline }"]
    I["useTxnDetail(refCode)"] -->|GET /v1/transactions/{refCode}| J["Transaction detail"]
  end
  subgraph "React Query Mutations"
    K["useTopup()"] -->|POST /v1/wallets/topup| L["TopupResult"]
    M["useTransfer()"] -->|POST /v1/transactions/transfer| N["TransferResult (202)"]
  end
```

### Response shapes (from backend code analysis)

| Endpoint | Key fields | Notes |
|---|---|---|
| `GET /v1/wallets` | `wallets[].{id, currency, purpose, label, balance}` | Main wallet = `purpose:"main"`. Balance = int64 minor units (VND). |
| `GET /v1/transactions` | `transactions[].{id, ref_code, type, status, sender_wallet_id, receiver_wallet_id, amount, fee, note, created_at, completed_at}` | Keyset pagination: `next_cursor` (RFC3339Nano) + `next_cursor_id` (UUID). Page size = 20. |
| `POST /v1/wallets/topup` | `{ref_code, status, amount, new_balance, created_at, duplicate?}` | Sync — completes immediately. Duplicate idem-key → 200 + original. |
| `POST /v1/transactions/transfer` | `{ref_code, status, amount, fee, total, created_at, duplicate?}` | Async — returns 202. Client must poll timeline. |
| `GET /v1/fees/preview?amount` | `{amount, fee, total}` | Fee based on caller's KYC tier. |
| `GET /v1/transactions/{refCode}/timeline` | `{ref_code, txn_status, timeline: {instance_status, current_step, steps[]}}` | Steps: `{step, status, attempt, finished_at?}`. Settled when `instance_status ∈ {completed, failed, compensated}`. |
| `GET /v1/transactions/{refCode}` | Full `Transaction` row (owner-only, 404 otherwise) + `direction, counterparty_name, counterparty_phone` | Used for the Txn Detail screen. |
| `GET /v1/users/lookup?phone=` | `{found, masked_name}` | Backend m3-api-support. 404 = not found; 429 = rate-limited. |
| `GET /v1/users/me` | `{user:{..., kyc_status}, txn_limit}` | Backend m3-api-support. The only source for the displayed limit. |

Note: `GET /v1/transactions` items also carry additional `direction/counterparty_*` (masked); the timeline additionally carries `failure_code?` when failed.

## 3. Task breakdown

| # | Task | Files | Size | Depends |
|---|---|---|---|---|
| T1 | Build the API hooks layer: `useWallets`, `useTransactions` (infinite), `useTopup` (mutation), `useTransfer` (mutation), `useFeePreview`, `useTimeline`, `useTxnDetail` | `src/features/wallet/hooks/` | L | M1 (client, polling) |
| T2 | Build the new UI components: `AmountInput`, `TxnListItem`, `TimelineStep`, `FilterChips`, `BalanceCard`, `EmptyState`, `ErrorState`, `Skeleton` | `src/ui/` | L | M1 tokens |
| T3 | Build the Home screen: balance card, quick actions, recent txns (5 items), pull-to-refresh | `app/(main)/home.tsx`, `src/features/wallet/components/` | L | T1, T2 |
| T4 | Build the History screen: filter chips, list grouped by date, infinite scroll cursor pagination, empty/error/skeleton | `app/(main)/history.tsx`, `src/features/wallet/components/TransactionList.tsx` | L | T1, T2 |
| T5 | Build the Topup flow: AmountInput → Result. `useTopup` mutation + idempotency key + auto-invalidate wallet balance | `app/(main)/topup/` (amount.tsx, result.tsx) | M | T1, T2 |
| T6 | Build the Transfer flow: Receiver → Amount → Confirm → Processing/Result. Debounced search, fee preview, polling timeline, 3 result variants | `app/(main)/transfer/` (receiver.tsx, amount.tsx, confirm.tsx, processing.tsx, result.tsx) | XL | T1, T2 |
| T7 | Build the Txn Detail screen: Amount + Status + Timeline steps + ref_code copy + detailed information | `app/(main)/txn-detail.tsx` | M | T1, T2 |
| T8 | Update the tab layout: icon + Vietnamese label, notification count badge (groundwork), QR placeholder, Account placeholder | `app/(main)/_layout.tsx` | S | T3 |
| T9 | Write unit tests: hooks (mocked fetch), components (TxnListItem render, AmountInput formatting), edge cases (M01 double-tap, M02 poll timeout, M03 partial failure copy) | `src/features/wallet/__tests__/`, `src/ui/__tests__/` | L | T1–T7 |

## 4. Edge cases (from doc 02 §6)

| Edge | Scenario | Handling |
|---|---|---|
| M01 | User double-taps the "Transfer" or "Top up" button | `useIdempotencyKey` same key → backend is idempotent, FE disables the button immediately on press (Loading variant). Mutation `onMutate` sets `isSubmitting=true`. |
| M02 | Transfer returns 202 but the poll times out after 30s | Show "The transaction is being processed" (do not say "failed"). `PollingTransactionWatcher` returns `{kind:'timeout'}` → navigate to Result(processing). |
| M03 | Transfer workflow fails (e.g. fraud, balance race) | Poll sees `instance_status='failed'` → navigate Result(failed) + show `failure_reason` via the error map. The "Retry" button generates a new idem key. |
| M06 | Money is always BigInt, never use Number/parseInt | All amount fields are parsed through `parseAmountToBigInt` → passed down to the API as int64. Rendered through `AmountDisplay`/`formatAmountVND`. |
| E01 | Idempotency-Key replay | BE returns 200 + the original txn → FE does not create a new transaction, shows the original result. |
| New | Balance hidden when the app opens | `balanceVisible` store default=false. The toggle is persisted. `AmountDisplay` shows "••••••" when hidden. |

## 5. Routing structure

```
app/
├── (main)/
│   ├── _layout.tsx          ← Tab navigator (update icons + labels)
│   ├── home.tsx             ← Home screen (replaces the placeholder)
│   ├── history.tsx          ← History screen (replaces the placeholder)
│   ├── qr.tsx               ← QR placeholder (kept as is)
│   ├── account.tsx          ← Account placeholder (kept as is)
│   ├── topup/
│   │   ├── _layout.tsx      ← Stack navigator for the topup flow
│   │   ├── amount.tsx       ← Enter the top-up amount
│   │   └── result.tsx       ← Top-up result
│   ├── transfer/
│   │   ├── _layout.tsx      ← Stack navigator for the transfer flow
│   │   ├── receiver.tsx     ← Choose recipient
│   │   ├── amount.tsx       ← Enter amount + note
│   │   ├── confirm.tsx      ← Confirm the transaction
│   │   └── result.tsx       ← Processing → Result
│   └── txn-detail.tsx       ← Transaction detail (param: refCode)
```

## 6. Schema changes

None — client-only phase. Backend APIs are ready.

## 7. API changes

Consumes the existing Backend endpoints:

- `GET /v1/wallets`: Get the wallet list (balance)
- `GET /v1/transactions`: Transaction history (cursor pagination)
- `GET /v1/transactions/{refCode}`: Transaction detail
- `GET /v1/transactions/{refCode}/timeline`: Workflow timeline
- `POST /v1/wallets/topup`: Top up (sync)
- `POST /v1/transactions/transfer`: P2P transfer (async 202)
- `GET /v1/fees/preview?amount`: Fee preview
- `GET /v1/users/lookup?phone=`: Recipient confirmation (masked name) — new, backend m3-api-support
- `GET /v1/users/me`: KYC status + `txn_limit` — new, backend m3-api-support

After the backend merges: run `npm run gen:api` to update `src/api/schema.d.ts` before coding T1.

## 8. Test plan

| Edge | Test (jest) | Content |
|---|---|---|
| M01 | `TestEdge_M01_DoubleTapTransfer_SingleRequest` | Press the "Transfer" button twice quickly → the mutation is called only once, with the same idempotency key |
| M02 | `TestEdge_M02_TransferPollTimeout_ProcessingCopy` | Mock the poll to always return "running" → timeout → show "processing" and not "failed" |
| M03 | `TestEdge_M03_TransferFailed_ErrorMapDisplay` | Mock the poll to return "failed" → show the specific reason from the error map, retry button visible |
| M06 | `TestEdge_M06_BigIntMoney_NoPrecisionLoss` | Check parse/format of large amounts (99.999.999.999 VND) through BigInt, no precision loss |
| — | `Test_AmountInput_LiveFormatting` | Type "1234567" → shows "1.234.567", paste "50000" → "50.000" |
| — | `Test_TxnListItem_DirectionSign` | Txn sender=myWallet → sign="out" (−), receiver=myWallet → sign="in" (+) |
| — | `Test_BalanceCard_HideToggle` | Hidden by default (••••••), tap toggle → shows balance, tap again → hidden |
| — | `Test_FilterChips_ClientFilter` | Chip Money in → only shows in txns, Processing → only shows processing/running |
| — | `Test_CursorPagination_NextPage` | When scrolling to the end, useInfiniteQuery fetches the next page with the correct cursor |

## 9. Dependencies to install

No new dependency needed. All UI uses React Native core components + the existing design tokens. React Query, Zustand, expo-crypto are already present. FlashList can be considered later if lists > 100 items cause lag — for now using the RN core `FlatList` is enough.

## 10. Rollout & risk

- Balance visibility: HIDDEN by default when the app opens (fintech best practice). The preference is persisted so it survives across sessions.
- Receiver search: Uses `GET /v1/users/lookup?phone=` (backend phase `m3-api-support`) — returns `{found, masked_name}`, rate-limited. There is NO dry-run fallback.
- Recent contacts (divergence during implementation): counterparty_phone in history is ALREADY MASKED data, so it cannot be used to prefill a new transfer. v1 uses a session-scoped store (records {full phone typed by the user themselves, masked_name} after each successful transfer — `src/features/wallet/recent-contacts.ts`). A durable address book needs a pay-by-counterparty-token contract → M4 backlog.
- Offline/slow network: React Query `staleTime:30s` + 2 retries already in place. Mutations do not retry (the idem key protects them).
- Displayed KYC limit: Take `txn_limit` from `GET /v1/users/me` (backend `m3-api-support`) — do NOT hardcode a client-side map (the old map in this plan had drifted from the server: manual_review is also 5M, auto_approved is also 100M).
- Failure reason (M03): Read `failure_code` from the timeline response (backend `m3-api-support`) → map via `src/lib/errors.ts`.

✅ Contract gaps handled contract-first: The 4 points previously intended as "temporary workarounds" (receiver lookup, counterparty name in history, hardcoded KYC limit, failure reason) have been added on the backend side in `docs/plans/backend/m3-api-support/`. M3 mobile only starts coding the dependent parts after that backend phase merges and `npm run gen:api` is re-run. No workaround remains in the M3 scope.

## 11. Execution checklist

1. Create the plan (this file + solution-architecture.md) → wait for user approval
2. T1: Build the API hooks layer (useWallets, useTransactions, useTopup, useTransfer, useFeePreview, useTimeline)
3. T2: Build the UI components (AmountInput, TxnListItem, TimelineStep, FilterChips, BalanceCard, EmptyState, ErrorState, Skeleton)
4. T3: Home screen (replace placeholder, wire hooks)
5. T4: History screen (filter, grouped list, infinite scroll)
6. T5: Topup flow (amount → result)
7. T6: Transfer flow (receiver → amount → confirm → processing → result)
8. T7: Txn Detail screen
9. T8: Tab layout update (icons, labels)
10. T9: Unit tests for hooks, components, edge cases
11. Run the checks: `npm run typecheck && npm run lint && npm test`
12. Deliver the results report and commit the code to Git.

hpay-mobile · docs/plans/m3-core-screens · 18/07/2026
