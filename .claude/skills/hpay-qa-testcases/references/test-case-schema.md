# Implementation Plan — Manual QA Excel Test-Case Suite

[← HPay Docs](../../README.md) · qa · plan

> **Feature slug:** `qa-test-case-suite` · Source: [guide/functional-overview.vi.md](../../guide/functional-overview.vi.md) (28 functions, 4 role groups) + [planning/master-plan.md](../../planning/master-plan.md) (37 edge cases E01–E20 / M01–M12 / A01–A07, which already have automated `TestEdge_*` tests). This is a **manual checklist suite in parallel** with the existing automated test suite — it does not replace it and does not duplicate its scope.

> **⛔ The scope of this working session stops at the 2 plan documents.** No `.xlsx` file is created, no test case is executed, no code is written in this session. The user stated explicitly: "I approve first, only then are you allowed to proceed" — these two documents (implementation-plan.md + solution-architecture.md) are the entire output, awaiting review before moving to the phase of creating the real Excel files.

## 1. Current state surveyed

| Component | Reality | Conclusion |
|---|---|---|
| Source of "WHAT to test" | `functional-overview.vi.html` has a `FEATURES` array with **28 functions** (not 29 — recounted manually: personal 8, business 9, admin 8, platform 3), each function having a description + step-by-step + mermaid | ✅ use as-is as the original function list for 4 of the 5 Excel sheets |
| Source of "edge case ID" | `master-plan.txt` + `index.html` confirm exactly 37 cases: **E01–E20** (backend, already have `TestEdge_*` in `src/backend/test/integration` and `internal/*`), **M01–M12** (mobile, `describe('TestEdge_M..')` in `src/mobile/src/**/__tests__`), **A01–A07** (admin, mentioned in scattered places in the hpay-admin plan docs, no centralized ID list yet) | ✅ every ID can be verified against a real name in the source — use as a mandatory reference column, do not invent new IDs |
| Convention for plan-document pairs | Each feature in 1 code repo generates `docs/plans/<feature>/{implementation-plan,solution-architecture}.html` — for example `src/backend/docs/plans/approval-cancel/` | ⚠️ this artifact does not belong to 1 code repo — it tests all 3 repos at once and the final output (Excel files) will live in the `hpay-docs` repo, not in any code repo. Reuse the "docs/plans/<feature>/" mold exactly but moved from the root to `docs/qa/plan/` — see section 3 for the detailed justification. |
| Claude's current execution capability | Has a browser automation tool (can drive hpay-admin — a React web app), can call HTTP/curl directly against the Go backend. **Does not have** a simulator/real device for Expo/React Native — cannot see or tap the mobile screens | ❌ this is a hard limit — the plan must clearly classify which cases Claude can run itself, which can only run halfway (via API), and which need a human |

## 2. Scope & Non-goals

### Ships (in these 2 plan documents)

- The location + directory structure for the Excel file set (files not created yet, only the location + naming specified).
- Detailed column schema, applied uniformly to all 5 files.
- Test ID naming convention, traceable back to both the feature (functional-overview) and the edge-case ID (E/M/A).
- Estimate of the number of test cases per function (a real task breakdown table, not "it depends").
- Execution model: which tool Claude will use to run each group of cases in a later phase, clearly classified as AUTO-API / AUTO-WEB / PARTIAL / HUMAN.

### Non-goals (explicitly excluded from this session)

- **Do not** design an automated test framework — that is a parallel effort, by a different person, a different artifact (`TestEdge_*` + the `gen-api-test` skill already cover that).
- **Do not** create any `.xlsx` file in this session.
- **Do not** run any test case, even to "see if it works" — even when it seems useful.
- **Do not** write an Excel-generation script (openpyxl/xlsxwriter...) — that is the job of the implementation phase after approval.

## 3. Location & directory structure

**Proposed location:** `D:\hpay\hpay-docs\qa\test-cases\` for the real `.xlsx` files (later phase), and `D:\hpay\hpay-docs\qa\plan\` for these 2 plan documents. Reasons for choosing `hpay-docs` over one of the 3 code repos:

- This test-case set **does not belong to 1 repo** — each file usually has steps on mobile, backend, and sometimes admin within the same business scenario (e.g. a checker approving a payout order needs both the checker's mobile app and the backend API).
- `hpay-docs` is already where the "overall design" and cross-repo documents are kept (see `planning/master-plan.md`, `guide/functional-overview.vi.md`) — the QA checklist is a cross-repo operational document of the same nature, not the code implementation plan of a single repo.
- QA/testers do not need to check out all 3 code repos to find the test-case files — one single place, in the spirit of `hpay-docs` being the "index" for the whole system.

### File structure — 5 sheets, matching the 4 role groups of functional-overview + 1 separate edge-case sheet

| File | Content | Source |
|---|---|---|
| `01-personal-user.xlsx` | 8 functions of the 👤 Personal user group | `FEATURES.filter(g==='personal')` |
| `02-business-maker-checker.xlsx` | 9 functions of the 🏢 Business wallet group (maker + checker) | `FEATURES.filter(g==='business')` |
| `03-admin-ops.xlsx` | 8 functions of the 🛡️ Operations / admin web group | `FEATURES.filter(g==='admin')` |
| `04-platform-crosscutting.xlsx` | 3 background mechanisms 🔧 running silently (automatic fraud, idempotency, self-recovery) | `FEATURES.filter(g==='platform')` |
| `05-edge-cases.xlsx` | 37 risk scenarios that already have automated tests — verified manually in parallel | `master-plan.txt`: E01–E20, M01–M12, A01–A07 |

**Why split sheet 05 out instead of stuffing all edge cases into the "Related Edge ID" column of sheets 01–04?** Both are done — they are not mutually exclusive. Sheets 01–04 **still have** at least 1 case referencing the edge ID right at the related function (e.g. the case "P2P transfer — double-tap" in sheet 01 has Related Edge ID = `M01`). But many edge cases (especially E02/E03/E08/E20 — race condition, deadlock, lease claim) require **staging multiple actors at once / multiple parallel steps** that an ordinary Excel row cannot express clearly — sheet 05 dedicates 1 row per ID with a Steps column long enough to describe a multi-actor scenario, and serves as the "master checklist" so that no ID of the 37 IDs committed to in `master-plan.md` section 5 (Definition of Done) is missed.

## 4. Column schema (applies to all 5 files, 1 "Test Cases" sheet + 1 "Read Me" sheet per file)

| # | Column | Type / Valid values | Notes |
|---|---|---|---|
| A | `Test ID` | `TC-<GROUP>-<NNN>` or `TC-EDG-<E/M/A-ID>` | See section 5 — detailed naming convention |
| B | `Feature` | Text — exactly the Vietnamese name in `FEATURES[].title` | Copy verbatim from functional-overview so it is search-matchable |
| C | `Role / Actor` | Enum: `Personal User` · `Business Maker` · `Business Checker` · `Org Admin` · `Ops Admin` · `System` | Who performs the main step in the case |
| D | `Test Type` | Enum: `Happy Path` · `Negative` · `Edge Case` · `Boundary` · `Security/AuthZ` | Every feature must have ≥1 `Happy Path`; features involving money/approval must have ≥1 `Edge Case` |
| E | `Priority` | Enum: `P0` (blocks release) · `P1` · `P2` | P0 by default for anything touching money (ledger, transfer, approval, topup) |
| F | `Precondition / Setup` | Multi-line text | Seed data needed beforehand — reference demo users/orgs from `seed-demo-data` where possible, so every case is repeatable |
| G | `Test Steps` | Text, numbered `1. 2. 3.` | Detailed enough for a stranger (or Claude) to follow without asking again |
| H | `Test Data` | Short text/JSON | Demo phone numbers, specific amounts, mock OTP codes... — separated from Steps so data can be changed without editing the scenario |
| I | `Expected Result` | Text | Clearly describes the final state + expected response code / UI state, not vague like "works correctly" |
| J | `Related Edge ID` | `E01–E20` / `M01–M12` / `A01–A07` or empty | Mandatory traceability column — the bridge between function cases and edge cases; sheet 05 references its own ID |
| K | `Executor` | Enum: `AUTO-API` · `AUTO-WEB` · `PARTIAL` · `HUMAN` | See section 6 — decide beforehand who/which tool runs this case, do not leave it until execution time |
| L | `Execution Method` | Short text | Makes column K concrete: e.g. "curl POST /v1/transactions/transfer" or "Browser tool click Approve on /tasks/{id}" or "Needs a real device — KYC camera" |
| M | `Status` | Enum: `Not Run` · `Pass` · `Fail` · `Blocked` · `N/A` | Defaults to `Not Run` when the file is generated; conditional color formatting in the real file creation phase |
| N | `Actual Result` | Text | Filled in during execution — for AUTO-API/AUTO-WEB cases, Claude fills in the real response/screenshot ref |
| O | `Evidence` | Text (path or link) | Screenshot (Browser tool), response JSON log, or "N/A — see TestEdge_X in CI" when cross-referencing an automated test |
| P | `Executed By` | Text | `Claude (auto)` or a person's name |
| Q | `Last Run Date` | Date | ISO `yyyy-mm-dd` |
| R | `Defect ID / Notes` | Text | Jira/GitHub issue link if Fail, or free-form notes |

## 5. Test ID naming convention

```
TC-<GROUP>-<NNN>          # sheets 01-04, numbered sequentially within the group
TC-EDG-<EDGE-ID>          # sheet 05, ID = the original edge-case ID itself

GROUP ∈ { PER, BIZ, ADM, PLT }
  PER = 01-personal-user
  BIZ = 02-business-maker-checker
  ADM = 03-admin-ops
  PLT = 04-platform-crosscutting

Examples:
  TC-PER-001   → sheet 01, case #1 (usually the Register happy path)
  TC-PER-014   → sheet 01, case about double-tap when doing a P2P transfer
  TC-BIZ-022   → sheet 02, case Checker signs with OTP
  TC-EDG-E08   → sheet 05, manual verification in parallel with TestEdge_E08_CheckerRaceSingleFinalize
  TC-EDG-M01   → sheet 05, manual verification in parallel with TestEdge_M01_DoubleTapTransfer_SingleRequest
  TC-EDG-A01   → sheet 05, manual verification of 409 ERR_ALREADY_FINALIZED between 2 admin reviews
```

Two-way traceability: every function case (01–04) that touches a risk with an existing edge ID records exactly that ID in column `J — Related Edge ID` (no new duplicate TC-EDG is created); conversely, every ID of the 37 original IDs has exactly 1 "closing" row in sheet 05 even if it is mentioned in another sheet. No ID is missed — verifiable by filtering column J across all 5 files and comparing with the list of 37 original IDs.

## 6. Task breakdown — number of test cases per function

### Sheet 01 — 01-personal-user.xlsx (28 cases)

| Function | Happy | Negative/Edge | Total | Related Edge IDs |
|---|---|---|---|---|
| Register & login | 1 | 3 | 4 | M05 (refresh reuse), E11 (OTP brute force) |
| Identity verification (KYC) | 1 | 3 | 4 | M10, M11 |
| Top up wallet | 1 | 2 | 3 | E01 (idempotent topup), E15 |
| P2P transfer | 1 | 4 | 5 | M01 (double-tap), M02 (poll timeout), M06 (BigInt), E14 (self-transfer) |
| Payment request & Split bill | 1 | 3 | 4 | E19 (expiry race) |
| Receive money via QR | 1 | 2 | 3 | — |
| Savings jars (Vaults) & round-up | 1 | 2 | 3 | — |
| History & timeline | 1 | 1 | 2 | — |
| **Sheet 01 total** | | | **28** | |

### Sheet 02 — 02-business-maker-checker.xlsx (26 cases)

| Function | Happy | Negative/Edge | Total | Related Edge IDs |
|---|---|---|---|---|
| Create organization & invite members | 1 | 2 | 3 | — |
| Set approval policy | 1 | 2 | 3 | E09 (policy snapshot) |
| Create payout order (Maker) | 1 | 2 | 3 | — |
| Track payout order status | 1 | 1 | 2 | — |
| Cancel an order pending approval | 1 | 2 | 3 | — |
| Pending-approval inbox (My Tasks — Checker) | 1 | 2 | 3 | — |
| View order detail + signature history | 1 | 1 | 2 | — |
| Approve / reject with OTP | 1 | 3 | 4 | E08 (checker race), E10 (expired) |
| Result after signing | 1 | 2 | 3 | E08 (checker race) |
| **Sheet 02 total** | | | **26** | |

### Sheet 03 — 03-admin-ops.xlsx (21 cases)

| Function | Happy | Negative/Edge | Total | Related Edge IDs |
|---|---|---|---|---|
| Overview dashboard | 1 | 1 | 2 | — |
| KYC review queue | 1 | 2 | 3 | — |
| Fraud monitoring | 1 | 2 | 3 | — |
| Processing flow monitoring (Workflow Monitor) | 1 | 2 | 3 | E04, E05, E20 |
| Daily transaction reconciliation | 1 | 2 | 3 | E16 (recon cutoff) |
| Admin My Tasks | 1 | 2 | 3 | A01 (409 conflict) |
| Audit log search | 1 | 1 | 2 | — |
| Organization & policy management | 1 | 1 | 2 | — |
| **Sheet 03 total** | | | **21** | |

### Sheet 04 — 04-platform-crosscutting.xlsx (10 cases)

| Function | Happy | Negative/Edge | Total | Related Edge IDs |
|---|---|---|---|---|
| Automatic fraud detection | 1 | 2 | 3 | — |
| Safe on re-press (Idempotency) | 1 | 2 | 3 | E01, E06, E15, M01 |
| Self-recovery on incidents | 1 | 3 | 4 | E04, E05, E17, E20 |
| **Sheet 04 total** | | | **10** | |

### Sheet 05 — 05-edge-cases.xlsx (39 cases = 1 row/ID)

| Group | Number of IDs | ID range | Runs in parallel with |
|---|---|---|---|
| Backend | 20 | E01–E20 | `TestEdge_*` in `src/backend/{internal,test/integration}` |
| Mobile | 12 | M01–M12 | `describe('TestEdge_M..')` in `src/mobile/src/**/__tests__` |
| Admin | 7 | A01–A07 | Scattered across `src/admin/docs/plans/*/implementation-plan.html` — no standardized-name test file yet, sheet 05 will be the first official ID summary table for group A |
| **Sheet 05 total** | | | **39** |

## 7. Total number of test cases

| File | Number of cases |
|---|---|
| 01-personal-user.xlsx | 28 |
| 02-business-maker-checker.xlsx | 26 |
| 03-admin-ops.xlsx | 21 |
| 04-platform-crosscutting.xlsx | 10 |
| 05-edge-cases.xlsx | 39 |
| **Grand total** | **124** |

This is an estimate based on the minimum rule (every feature ≥1 happy path, features involving money/approval ≥1 edge case) — the real number when each row is written in detail may vary by ±15-20%, especially in the Business and Edge Cases groups where a multi-actor scenario sometimes needs to be split into 2 rows (the main case + a timing-variant case).

## 8. Execution model — how Claude will run these cases (later phase, after approval)

Currently available tools: a **browser automation tool** (drives a real browser, can navigate/click/read the DOM/take screenshots — used for `hpay-admin`, a React SPA running on the web); **calling HTTP/curl directly** against the Go backend API (used for every endpoint in `openapi.yaml`, no UI needed). **No** simulator or real device for Expo/React Native — `hpay-mobile` is a mobile app, and Claude cannot see or tap its screens through any existing tool.

| Label | Meaning | Applies to |
|---|---|---|
| `AUTO-API` | Claude runs the whole thing itself via direct HTTP requests (curl/PowerShell) to the backend, reads the response, and fills in Pass/Fail itself — no human needed | Most of sheet 04 (platform), most of the E-series in sheet 05, and the "behind the scenes" part (state/response) of sheets 01-02 even though the UI is mobile |
| `AUTO-WEB` | Claude runs the whole thing itself via the browser automation tool on `hpay-admin` — click, read the DOM, take screenshots as Evidence | All of sheet 03 (admin ops), the A-series in sheet 05 |
| `PARTIAL` | Claude can confirm the **correct backend behavior** via API (did the money move correctly, is the state machine correct, is the response code correct) but **cannot confirm the mobile UI** displays correctly, shows the right notification, or has the right animation/format — a human needs to re-check the UI part for the case to count as a full "Pass" | Most of sheets 01-02 (personal + business both run on the mobile app) — Claude pre-fills the backend part and leaves the UI cell blank for a person to sign off |
| `HUMAN` | Needs a real device/simulator, hardware (real camera for KYC, fingerprint/Face ID sensor), or visual observation that cannot be measured via API (animation, haptics, real push notifications received on the device, OS behavior when the app goes to background) — Claude cannot touch any part of this case | M-series (biometric, KYC camera, screenshot-block, background/foreground), the photo-capture part of KYC in sheet 01, every assertion about "the app displays correctly" that Claude has no way to observe via API |

### Estimated Executor distribution per file

| File | AUTO-API | AUTO-WEB | PARTIAL | HUMAN |
|---|---|---|---|---|
| 01-personal-user.xlsx | — | — | ~18 | ~10 (KYC capture, biometric, QR camera) |
| 02-business-maker-checker.xlsx | — | — | ~22 | ~4 (OTP entry on the device, biometric unlock before signing) |
| 03-admin-ops.xlsx | — | ~21 | — | — |
| 04-platform-crosscutting.xlsx | ~10 | — | — | — |
| 05-edge-cases.xlsx | ~20 (E-series) | ~7 (A-series) | — | ~12 (M-series) |

How Claude will execute concretely in a later phase (not done in this session): (1) build seed data with the `seed-demo-data` skill so that every AUTO-* case can be rerun many times without touching old data; (2) for AUTO-API — use Bash/PowerShell to call the exact endpoint in `openapi.yaml`, compare the response with the Expected Result column, and write the real JSON response into the Actual Result column; (3) for AUTO-WEB — use the Browser tool (`preview_start` pointing at the `hpay-admin` dev server, `navigate`/`computer`/`read_page`), taking screenshots as Evidence; (4) for PARTIAL — Claude fills in the backend part then marks `Blocked (needs human to confirm UI)`, never assigning Pass itself for the unverified part; (5) for HUMAN — Claude does not touch them, leaves them as `Not Run` and states in Notes why a person + device is needed.

## 9. Rollout & risk

Low risk in this plan phase — no files created, nothing run. Risks to note for the next phase (creating the real Excel files): (1) 124 cases is a floor estimate, the real number may reach ~150 when each row is written in detail, requiring corresponding time; (2) the A-series (admin edge cases) has no official centralized ID anywhere yet — sheet 05 will be the first source to establish those IDs, so it must be cross-checked carefully against the scattered passages in `src/admin/docs/plans/*/implementation-plan.html` to avoid duplicating/shifting the meaning of an ID already in use; (3) Claude self-marking a PARTIAL case "Pass" without a person confirming the UI part is a risk of false reporting — hard rule: PARTIAL never auto-transitions to `Pass`, only to `Blocked` awaiting a person.

## 10. Execution checklist (when approved, for the next phase)

1. Review these 2 plan documents together with the user — wait for a clear response before doing anything further.
2. If approved: create `D:\hpay\hpay-docs\qa\test-cases\`, generate 5 `.xlsx` files exactly per the section 4 schema, with data drafted from the `FEATURES` content + the list of 37 edge IDs.
3. Each file gets an extra "Read Me" sheet explaining the column schema + the Executor legend, so users from other teams can read and understand it without going back to this plan document.
4. After the files exist: run 1 AUTO-API case + 1 AUTO-WEB case as a smoke test of the execution process, before running all 124 cases.
5. Do not expand the scope on your own (automation framework, mobile device farm...) — outside the agreed scope.

---

HPay Docs · plan for the manual QA checklist suite · repo hpay-docs · 2026-07-29 · companion to [solution-architecture.md](./solution-architecture.md)
