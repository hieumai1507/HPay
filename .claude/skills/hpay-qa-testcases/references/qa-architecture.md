# Solution Architecture — Manual QA Excel Test-Case Suite

[← HPay Docs](../../README.md) · qa · plan · solution architecture

> Companion to [implementation-plan.md](./implementation-plan.md). The focus here: the data structure (directories/files/columns), the data flow from the sources (functional-overview + master-plan) into Excel, and the execution flow Claude will use in a later phase — **no code runs in this document**, it only describes the design.

## 1. Context — data sources → Excel set → human/Claude executors

```mermaid
flowchart LR
  FO["functional-overview.vi.html\nFEATURES array — 28 functions\n4 groups: personal/business/admin/platform"] -->|"≥ 1 test case row per feature"| GEN["Later phase (not done yet):\ngenerate 5 .xlsx files per schema"]
  MP["master-plan.html / .txt\n37 edge IDs: E01-E20, M01-M12, A01-A07"] -->|"1 row/ID + cross Related Edge ID column"| GEN
  GEN --> F1["01-personal-user.xlsx"]
  GEN --> F2["02-business-maker-checker.xlsx"]
  GEN --> F3["03-admin-ops.xlsx"]
  GEN --> F4["04-platform-crosscutting.xlsx"]
  GEN --> F5["05-edge-cases.xlsx"]
  F1 & F2 --> PARTIAL["Claude: PARTIAL\n(API verify + leave UI blank for a human)"]
  F3 --> AUTOWEB["Claude: AUTO-WEB\n(Browser tool on hpay-admin)"]
  F4 --> AUTOAPI["Claude: AUTO-API\n(curl directly to backend)"]
  F5 --> MIX["Claude: mix of AUTO-API/AUTO-WEB/HUMAN\nby E/A/M prefix"]
  PARTIAL --> HUM["👤 Human: confirm the mobile UI part\n+ every HUMAN case (camera/biometric)"]
```

## 2. Directory structure

```mermaid
flowchart TB
  ROOT["D:\\hpay\\docs\\qa\\"]
  ROOT --> PLAN["plan\\\nimplementation-plan.html\nsolution-architecture.html\n(these 2 files — current session)"]
  ROOT --> TC["test-cases\\\n(phase AFTER approval — NOT created yet)"]
  TC --> X1["01-personal-user.xlsx"]
  TC --> X2["02-business-maker-checker.xlsx"]
  TC --> X3["03-admin-ops.xlsx"]
  TC --> X4["04-platform-crosscutting.xlsx"]
  TC --> X5["05-edge-cases.xlsx"]
  X1 -.-> SH1["sheet: Test Cases\nsheet: Read Me"]
  X5 -.-> SH5["sheet: Test Cases\nsheet: Read Me"]
```

## 3. Sample execution flow — AUTO-API (example sheet 04, idempotency)

```mermaid
sequenceDiagram
  actor Claude
  participant XL as 04-platform-crosscutting.xlsx
  participant BE as hpay-backend API

  Claude->>XL: Read row TC-PLT-005\n(Steps + Test Data + Expected Result)
  Claude->>BE: POST /v1/transactions/transfer\nIdempotency-Key: X (1st time)
  BE-->>Claude: 202 {refCode: R1}
  Claude->>BE: POST /v1/transactions/transfer\nIdempotency-Key: X (2nd time, simulating a re-press)
  BE-->>Claude: 202 {refCode: R1} — SAME refCode
  Claude->>Claude: Do the 2 responses == Expected Result?
  Claude->>XL: Write Actual Result + Status=Pass/Fail\n+ Executed By=Claude (auto) + Last Run Date
```

## 4. Sample execution flow — AUTO-WEB (example sheet 03, admin approves My Tasks)

```mermaid
sequenceDiagram
  actor Claude
  participant XL as 03-admin-ops.xlsx
  participant BR as Browser tool
  participant ADM as hpay-admin (dev server)

  Claude->>XL: Read row TC-ADM-016 (My Tasks — admin B approves)
  Claude->>BR: preview_start hpay-admin, navigate /tasks
  BR->>ADM: render the pending-approval list
  Claude->>BR: computer click "Approve" on admin A's request
  BR->>ADM: call the approve API, update the UI
  Claude->>BR: read_page / get_page_text to confirm the toast "Approved"
  Claude->>BR: screenshot → save as Evidence
  Claude->>XL: Write Actual Result + Status + Evidence path
```

## 5. Sample execution flow — PARTIAL (example sheet 01, P2P transfer on mobile)

```mermaid
flowchart LR
  C["Claude"] -->|"curl the transfer API directly\n(bypassing the mobile UI)"| BE["Backend: check that\nmoney was debited/credited correctly,\nstate machine correct,\nno duplicate transaction created"]
  BE -->|"correct"| P["Record the backend part:\nStatus temporarily = Blocked\n(awaiting human UI confirmation)"]
  P --> H["👤 Human: open the real app,\nredo the exact action through the UI,\nconfirm screen/animation/notification are correct"]
  H -->|"UI correct"| DONE["Status → Pass"]
  H -->|"UI wrong"| FAIL["Status → Fail + Notes describing the deviation"]
```

## 6. Data model — relationships between entities

```mermaid
erDiagram
  FEATURE ||--o{ TEST_CASE : "has 1..n test cases"
  EDGE_ID ||--o{ TEST_CASE : "referenced by (column J)"
  EDGE_ID ||--|| EDGE_CASE_ROW : "has exactly 1 closing row in sheet 05"
  TEST_CASE }o--|| EXECUTOR_TYPE : "assigned 1 of 4 labels"
  TEST_CASE {
    string test_id PK
    string feature
    string role_actor
    string test_type
    string priority
    string related_edge_id FK
    string executor
    string status
  }
  EDGE_ID {
    string id PK
    string source_repo
    string automated_test_name
  }
```

## 7. Decisions & trade-offs

| Decision | Alternative | Why |
|---|---|---|
| Put the Excel set + plan in `hpay-docs`, not in 1 of the 3 code repos | Put it in `hpay-backend` (the "heaviest" repo) following the `docs/plans/<feature>/` mold used for every other feature | Test cases touch all 3 repos in the same scenario (e.g. signing approval: the checker's mobile UI + the backend API); stuffing it into 1 code repo would leave the other 2 repos "unaware" the test set exists. `hpay-docs` already plays the cross-repo index role. |
| 5 files, matching exactly the 4 role groups + 1 separate edge-case sheet | Merge everything into 1 multi-sheet file, or split by code repo (3 files: backend/mobile/admin) | Splitting by code repo would break cross-repo features (1 function usually touches ≥2 repos) — splitting by business role (like functional-overview) keeps the end user's viewpoint, easy to cross-reference back to the source documents. 5 separate files instead of 1 multi-sheet file so each role-owner (mobile QA, admin QA...) opens exactly the 1 file they need, not paging through many sheets. |
| A separate edge-case sheet 05, not relying only on the Related Edge ID column in sheets 01-04 | Use only column J, no separate sheet — look up by filter | Many edge cases (race condition, multi-actor, timing) need a longer scenario description than the usual 1-row "function case" mold allows, and need a "master checklist" to verify all 37/37 IDs — filters scattered across 4 files easily miss one. |
| 4 hard Executor labels (AUTO-API/AUTO-WEB/PARTIAL/HUMAN) assigned from design time, not "computed later" | Do not classify beforehand, Claude decides at run time for each case | Having no mobile simulator is a known hard limit — deciding beforehand prevents Claude from self-claiming "Pass" for a UI part it has never seen; PARTIAL always stops at Blocked awaiting a human, never inferring a correct UI from a correct API response. |
| Do not design an automated test framework in this artifact | Take the opportunity to write scripts that generate + run automated tests for the AUTO-* cases | Outside the assigned scope — another parallel effort already covers automation (`TestEdge_*` + `gen-api-test`); mixing the 2 would bloat the plan and make it hard to review in a single approval pass. |

## 8. Known limits (recorded, not bugs)

| Limit | Impact |
|---|---|
| No Expo/React Native simulator/device | Every mobile-UI case is at most PARTIAL — never AUTO for the display part |
| A-series (admin edge IDs) has no centralized official ID list | The A part of sheet 05 will be the source establishing the IDs for the first time in the file-creation phase — scattered content in `src/admin/docs/plans/*` must be cross-checked carefully so the meaning is not mis-assigned |
| The case count (124) is a floor estimate | May rise ~15-20% when written in detail — does not affect the structure/schema, only the effort of the later phase |

---

HPay Docs · plan for the manual QA checklist suite · repo hpay-docs · 2026-07-29
