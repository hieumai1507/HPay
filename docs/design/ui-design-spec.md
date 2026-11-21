# UI Design Spec — HPay (source for building Figma)

[← HPay Docs](../README.md)

> This spec is a detailed "in words" design sufficient to build a proper Figma file: design tokens → components → screens. When coding the FE, use the Figma MCP (`get_design_context`, `get_variable_defs`) to read back from Figma into code, so **token and component names in Figma must match this spec exactly** for automatic mapping.

## 1. Proposed Figma file structure

```
HPay Design
├── Page: 🎨 Foundations      # color/type/spacing variables + styles
├── Page: 🧩 Components        # component library (2 themes: Mobile, Admin)
├── Page: 📱 Mobile            # frames 390×844 (iPhone 15) — flows by section
├── Page: 🖥 Admin             # frames 1440×900 — 1 section per route
└── Page: 🔀 Flows             # prototype wiring + user journeys
```

Figma Variables are named after the tokens below (collections `color`, `space`, `radius`, `type`). Components are named `Category/Name/Variant` (for example `Button/Primary/Default`).

## 2. Design Tokens

### 2.1. Colors (collection `color`)

| Token | Light | Role |
|---|---|---|
| `color/brand/400` | #1ECBE1 | Main accent — hero, BalanceCard gradient, active icon (not used as a background for white-text buttons) |
| `color/brand/600` | #0E90A3 | Primary (main button, link) — darker derivative of brand #1ECBE1, meets AA for white text |
| `color/brand/700` | #0B7181 | Pressed |
| `color/brand/50` | #E6FAFD | Brand badge/chip background |
| `color/success/600` | #16A34A | Successful transaction, matched |
| `color/warning/600` | #D97706 | pending_approval, manual_review |
| `color/danger/600` | #DC2626 | failed, fraud, rejected |
| `color/info/600` | #0891B2 | processing, running |
| `color/neutral/900→50` | 10-step gray scale | Text, border, background |

Transaction statuses are mapped strictly 1-to-1 to colors (used by both FEs — do not invent your own):
`completed=success`, `processing/running=info`, `pending_approval/manual_review=warning`, `failed/rejected/expired=danger`, `created=neutral`, `reversed/compensated=neutral+icon ↩`.

### 2.2. Typography (collection `type`)

Font: **Inter** (tabular-nums for every monetary figure — mandatory, to avoid column jumping).

| Token | Size/Line | Usage |
|---|---|---|
| `type/display` | 32/40 Bold | Balance on Home |
| `type/h1` | 24/32 SemiBold | Screen title |
| `type/h2` | 18/26 SemiBold | Section |
| `type/body` | 15/22 Regular | Default |
| `type/caption` | 13/18 Regular | Meta, timestamp |
| `type/mono` | 13/18 JetBrains Mono | ref_code, JSON viewer (admin) |

### 2.3. Spacing & radius

`space`: 4, 8, 12, 16, 20, 24, 32, 40. `radius`: sm 8, md 12, lg 16, full 999. Mobile cards use `radius/lg`, admin uses `radius/md`.

## 3. Component Library (named correctly so MCP can map)

| Component | Variants | Design notes |
|---|---|---|
| `Button` | Primary/Secondary/Ghost/Danger × Default/Pressed/Disabled/Loading | Loading = spinner + keeps width (no layout jump). The transaction-creating button must have a `Loading` variant in the prototype |
| `AmountInput` | Default/Focus/Error | Large number in the center of the screen, thousands separator, dimmed ₫ unit on the right; error text below |
| `AmountDisplay` | size sm/md/display; sign +/− | Money in is success color + sign +, money out is neutral-900 + sign − |
| `StatusBadge` | 8 statuses in §2.1 | Chip with color-50 background, color-600 text, dot before the text |
| `TxnListItem` | in/out/pending | Avatar/icon of the transaction type, counterparty name, 1-line ellipsis note, right side: AmountDisplay + StatusBadge |
| `TimelineStep` | done/active/failed/waiting | Used for txn detail (mobile, public view) and workflow monitor (admin, full view) |
| `OTPInput` | 6 cells | Auto-advance, paste the whole string, TTL countdown + resend button disabled until cooldown ends |
| `PolicyTierRow` | view/edit | "Level 1 — needs 2 signatures" + stepper; used in Admin policies and Mobile (view-only in org order detail) |
| `ApprovalCard` | pending/mine-done/finalized | Order awaiting approval: large amount, creator, payout destination, progress "1/2 approved" |
| `KYCPhotoFrame` | id-front/selfie × empty/preview/error | Guide overlay frame (ID card outline / face oval) |
| `DataTable` (admin) | — | Sticky header, row hover, numeric columns right-aligned tabular |
| `JSONViewer` (admin) | collapsed/expanded | Mono, brand-colored keys, copy button |
| `KPICard` (admin) | default/alert | Large number + delta vs yesterday |
| `EmptyState` / `ErrorState` / `Skeleton` | per-list | Every list must have all 3 states in Figma |

## 4. Mobile Screens (390×844) — 24 frames

### Auth & KYC flow (6)

1. **Onboarding** — 3 value-prop slides, Register CTA.
2. **Register** — phone, password (strength meter), terms checkbox.
3. **OTP** — OTPInput, 120s countdown.
4. **KYC Intro** — 2 steps to do, lighting note.
5. **KYC Capture** ×2 — full-screen camera + KYCPhotoFrame overlay, retake/use-photo buttons.
6. **KYC Status** — 3 variants: pending (illustration + "AI is checking"), verified, manual_review ("1–2 business hours").

### Main flow (10)

1. **Home** — Greeting header + avatar; balance card (AmountDisplay display size, show/hide button 👁 — HIDDEN by default when the app opens); 4 quick actions (Top up, Transfer, QR, History); "Recent transactions" with 5 TxnListItem.
2. **Transfer: Recipient** — search by phone number (debounce, shows masked name when found), scan-QR button, recent contacts.
3. **Transfer: Amount** — AmountInput + available balance + note; realtime limit validation (shows "exceeds KYC tier limit" if applicable).
4. **Transfer: Confirm** — summary table (recipient, amount, fee, total), warning if the recipient is new; "Transfer" button (Loading variant when pressed — disabled immediately, M01).
5. **Transfer: Processing** — spinner + condensed timeline (Sent → Checking → Completed); copy matches the actual workflow state.
6. **Transfer: Result** — 3 variants: success (✓ + light confetti), failed (reason via error map + retry button), pending_approval ("Awaiting business approval").
7. **Topup** ×2 — enter amount → OTP → result (reuse components).
8. **History** — filter chips (All/In/Out/Processing), list grouped by day, FlashList.
9. **Txn Detail** — AmountDisplay, StatusBadge, TimelineStep list (public view), ref_code mono + copy, "Report an issue" button.
10. **Receive-money QR** — large QR on a white background, name + masked phone, share/save buttons.

### Business flow (6)

1. **Org Switcher** — in Account: choose personal/organization context (role clearly shown: Maker/Checker L1…).
2. **Maker: Create payout order** — recipient + amount + note form; shows PolicyTierRow (view-only) to indicate who will need to approve the order.
3. **Maker: Order status** — ApprovalCard + per-level progress, who approved and when.
4. **Checker: Approval Inbox** — badge with the number of pending orders; ApprovalCard list; empty state "No pending orders".
5. **Checker: Order detail** — full information + action history; 2 buttons Approve (primary) / Reject (danger, reason required).
6. **Checker: OTP confirmation** — OTPInput; after approval shows the result "Approved — awaiting level 2" or "Completed, transaction in progress" (the M09 conflict state is also drawn here).

### Other (2)

1. **Account** — profile, KYC tier + limits, security (change password, biometric toggle, signed-in devices), Business Wallet entry.
2. **Locked/Idle** — biometric screen after 5 minutes idle.

## 5. Admin Screens (1440×900) — 12 frames

1. **Login** — email + password + (mock) 2FA.
2. **Dashboard** — 4 KPICards, 24h volume chart, 4 "needs attention" panels (§4.1 of the admin doc).
3. **KYC Queue** — 3 tabs, DataTable (user, score, verdict, case age), keyboard shortcut hint in the footer.
4. **KYC Detail** — split: left 2 zoomable photos, right info + ai_reasons checklist + action form; queue navigation bar (J/K) at the top.
5. **Fraud Queue** — DataTable + row-expand reasons; bulk select "Mark safe".
6. **Fraud Detail Dialog** — radio choices with consequences: clear / lock account / reverse — each choice has a description of its consequence.
7. **Workflow Monitor** — filter bar (status, definition, stuck>N minutes), DataTable, current_step column + age.
8. **Workflow Instance Detail** — txn info header; vertical timeline with TimelineStep full view; each step expands a JSONViewer for input/output; action bar per `allowed_actions`.
9. **Recon Runs** — table of dates, matched/mismatched badges, trigger-run button.
10. **Recon Detail** — matched % KPI, DataTable of items filtered by result, "AI summary" panel, Export button.
11. **Orgs & Policies** — master-detail; PolicyTierRow edit mode + policy snapshot warning banner (E09).
12. **Audit Log** — DataTable + filter, read-only.

## 6. General UX rules (apply to every screen)

- Every monetary amount: tabular-nums, never wraps, negative/positive with a clear sign.
- Every list: full Skeleton/Empty/Error states (components already exist).
- Every dangerous action: a confirm dialog stating the specific consequence, no generic "Are you sure?" wording.
- Every async state: display using the exact system-status language (§2.1 mapping) — do not invent new statuses in the FE.
- Timestamp: relative (< 24h) + absolute tooltip; time zone Asia/Ho_Chi_Minh.
- Accessibility: AA contrast, touch target ≥ 44px (mobile), clear focus ring (admin).

## 7. Workflow with Figma MCP

1. Build Foundations (variables named exactly per the §2 tokens) → Components (§3) → Screens (§4–5).
2. When coding: `get_variable_defs` → generate `tokens.ts` / Tailwind config; `get_design_context` for each frame → build the screen; `get_screenshot` to compare.
3. Name code components identically to the Figma components → use Code Connect mapping if needed.

## 8. v2.1 additions — Request Money · Split Bill · Vaults · My Tasks

Features added from [doc 04](../history/design-v2/04-feature-extensions.md). Corresponding mockup: `mockups/06-requests-vaults.html` (mobile screens 25–31 + admin A11).

### New components

| Component | Variants | Design notes |
|---|---|---|
| `RequestCard` | incoming/outgoing × pending/paid/declined/expired | Large amount, 1-line reason, expiry countdown; incoming has 2 buttons Pay now / Decline |
| `SplitAvatarStack` | — | Avatars overlapped with −12px offset, unpaid people at opacity .45; with a progress bar of the total collected |
| `VaultCard` | progress/new | Emoji icon in a light-colored tile, progress bar by % of goal, tabular amounts |
| `RoundUpToggle` | on/off | Card explaining with a real rounding example + switch |
| `TaskRow` (admin) | reversal/account_lock/policy_change | Type chip colored danger/warning, expandable detail panel with 2 approve/reject sign buttons |

### New screens

| # | Screen | Key points |
|---|---|---|
| 25 | Request Money — create request | Choose person → amount → reason; 72h TTL clearly shown |
| 26 | Request Inbox | 2 tabs "I need to pay / I sent", count badge, swipe-to-pay |
| 27 | Pay request | Prefilled from the request → straight to the P2P engine confirm screen; handles the case where the request has just expired (E19) |
| 28 | Split Bill — setup | Split equally/custom, preview "will create N requests" |
| 29 | Split Bill — tracking | SplitAvatarStack + collected progress, remind-each-person button |
| 30 | Vaults — list | Total-saved hero (green gradient), VaultCard, RoundUpToggle |
| 31 | Vault detail | Progress ring %, put in/withdraw, round-up history; note that vault_move is a transaction with ledger entries |
| A11 | Admin My Tasks | approval_requests queue (reversal/lock/policy); the Maker ≠ Checker principle is shown right in the UI |

---

HPay docs · auto-generated from `design/ui-design-spec.md` by `tools/build-html.py`
