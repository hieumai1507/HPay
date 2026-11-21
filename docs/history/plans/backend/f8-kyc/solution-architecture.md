[← implementation-plan](implementation-plan.md) · f8-kyc

# F8 KYC + AI — Solution Architecture

## 1. Context — the parts of the system F8 touches

```mermaid
flowchart LR
  mob["📱 Mobile\n(user KYC)"] -->|"POST /v1/kyc/submit\nmultipart 2 images"| kycapi["internal/api\nkyc_handler.go"]
  adm["🖥 Admin SPA\n(A2 queue)"] -->|"JWT aud=admin"| admapi["internal/api\nadmin auth + kyc review"]
  kycapi --> kycsvc["internal/kyc\nservice.go"]
  kycsvc -->|"write images first"| st["internal/storage\nlocal driver + HMAC signed URL"]
  kycsvc -->|"tx: kyc_requests +\nusers→pending + engine.Start\n(txn = NULL — first time)"| eng["workflow.Engine (F4)\nunchanged"]
  eng --> h1["handler kyc_scorer\n(load images → Scorer)"]
  h1 --> sc{"Scorer interface"}
  sc -->|"ANTHROPIC_API_KEY present"| claude["🤖 Claude vision\n(net/http, JSON score+reasons)"]
  sc -->|"key missing (CI/offline)"| mock["mockScorer\ndeterministic by image hash"]
  eng --> h2["handler kyc_classifier\n≥85 auto · 50–84 manual · <50 reject\n(guarded, idempotent)"]
  admapi --> st
  h2 --> pg[("PostgreSQL\nkyc_requests · users.kyc_status\naudit_logs")]
  kycsvc --> pg; admapi --> pg
```

## 2. Main flow — submit → score → classify → (review)

```mermaid
sequenceDiagram
  autonumber
  actor U as User
  participant API as API /v1/kyc
  participant ST as storage (local)
  participant ENG as Engine
  participant SC as kyc_scorer
  participant CL as kyc_classifier
  actor AD as Admin
  U->>API: POST /kyc/submit (id_card + selfie)
  API->>API: guard: already pending/manual? already verified? → 4xx
  API->>ST: write 2 images ({request_id}/id_card.jpg, selfie.jpg)
  API->>ENG: tx: INSERT kyc_requests(pending)\n+ users.kyc_status none|rejected→pending (guarded)\n+ engine.Start("kyc_verification", txn=NULL, ctx={kyc_request_id})
  API-->>U: 202 {request_id} — poll GET /kyc/status
  ENG->>SC: SCORE (retry 2, timeout 30s)
  SC->>ST: load 2 images
  SC->>SC: Scorer.Score() → claude or mock\nnetwork error/5xx → RetryableError (engine retry+backoff)
  SC->>ENG: output {score, reasons} + SetKYCScore
  ENG->>CL: CLASSIFY
  alt score ≥ 85
    CL->>CL: kyc_requests→auto_approved + users→auto_approved
  else 50 ≤ score < 85
    CL->>CL: kyc_requests→manual_review + users→manual_review\n(enters the admin queue with ai_reasons)
    AD->>API: GET /admin/kyc?tab=manual (images = signed URL 5')
    AD->>API: POST /admin/kyc/:id/review {approve|reject, reason}
    API->>API: tx: WHERE status='manual_review' (0 rows → 409)\n+ users→verified|rejected + audit_logs
  else score < 50
    CL->>CL: kyc_requests→rejected + users→rejected\n(user may submit again)
  end
```

## 3. State — users.kyc_status (enum exists since F1, only now runs the full cycle)

```mermaid
stateDiagram-v2
  [*] --> none : registration (F2)
  none --> pending : submit KYC
  rejected --> pending : re-submit (re-KYC)
  pending --> auto_approved : CLASSIFY score ≥ 85\n(transaction limit raised to 100M — wallet.TxnLimit F3)
  pending --> manual_review : 50 ≤ score < 85
  pending --> rejected : score < 50
  manual_review --> verified : admin approve\n(guarded + audit)
  manual_review --> rejected : admin reject
  auto_approved --> [*]
  verified --> [*]
  note right of manual_review : 2 reviewers racing →\nUPDATE WHERE status='manual_review'\nthe later one gets 0 rows → 409 (foundation for A01)
```

## 4. Data model delta — no new tables

```mermaid
erDiagram
  users ||--o{ kyc_requests : "user_id (multiple re-KYC rounds)"
  users ||--o{ kyc_requests : "reviewer_id (admin, nullable)"
  workflow_instances }o..|| kyc_requests : "linked via instance context\n{kyc_request_id} — NO FK column,\ninstance.transaction_id = NULL"
  admin_users ||--o{ audit_logs : "actor_id (actor_type=admin)"
  kyc_requests {
    text id_card_url "storage key, not a public URL"
    text selfie_url "storage key"
    numeric ai_score "written from int 0-100"
    jsonb ai_reasons "LLM explanation — admin reads it during review"
    kyc_status status "pending → auto_approved | manual_review | rejected → verified|rejected"
  }
```

Migration 000014 only seeds: definition `kyc_verification` v1 + 1 dev admin user. No ALTER.

## 5. Decisions & trade-offs

| Decision | Alternative | Why |
| --- | --- | --- |
| **2 steps SCORE → CLASSIFY** instead of 1 step doing both | 1 step calls the LLM and classifies right away | Separates the *non-deterministic + slow + retryable* part (the LLM call) from the *deterministic* part (thresholds): SCORE can be retried without re-running classification; CLASSIFY is purely idempotent thanks to the guarded UPDATE; the admin timeline (A3) clearly shows where it is stuck |
| Link instance ↔ kyc_request via **workflow context** (`{kyc_request_id}`), no new column | Add a column `kyc_requests.instance_id` or `workflow_instances.subject_id` | The JSONB context exists for exactly this (docs 00 §3.3 "data passed between steps"); adding a generic subject column to workflow_instances is a large schema change not yet needed — if F9+ needs frequent reverse lookup, propose a docs fix first |
| Scorer chosen by **presence of ANTHROPIC_API_KEY**, mock deterministic by image hash | A separate config flag KYC_SCORER=claude\|mock | Matches the design in docs 00 §8.2 ("mock fallback when the key is missing"); a hash-based mock makes demos repeatable (same image → same verdict) instead of purely random |
| Score is `int` 0–100 throughout Go, written into NUMERIC(5,2) | float64 matching NUMERIC | There is no reason to need decimals; keeping the codebase 100% float-free makes the review/interview story absolutely clean |
| HMAC signed URL (key+exp, secret=JWT_SECRET) for images, a separate route without JWT | Admin endpoint streaming images behind RequireAdmin | An SPA rendering `<img src>` cannot attach an Authorization header — a signed URL is self-authenticating, matching doc 01 §3.4 "URL signed with a 5-minute lifetime"; the short TTL limits leakage |
| **Minimal admin auth in F8** (login + middleware, access-only) | Wait for a dedicated admin phase | Master plan F8 explicitly says "admin review API" — without a login path that API is unusable; the F2 JWT issuer already supports audience so the cost is near zero; refresh/detailed RBAC wait until the A-phases need them |
| Admin review uses a guarded UPDATE (0 rows → 409), not FOR UPDATE | SELECT FOR UPDATE like the F6 approval | Review is a single-statement transition, with no multi-step evaluation like approval — the WHERE guard is atomic enough and takes fewer locks; it returns exactly the 409 semantics that A01 (admin repo) needs |

## 6. Invariants touched (index.html §8)

| Invariant | How F8 upholds it |
| --- | --- |
| State transitions always have a WHERE status guard | `SetUserKycStatusGuarded` follows a fixed transition map; `ClassifyKYCGuarded WHERE status='pending'`; `ReviewKYCGuarded WHERE status='manual_review'` — a replayed CLASSIFY (at-least-once) becomes a no-op |
| Events are emitted only via the outbox | The KYC workflow follows the F4 path unchanged: engine.Start writes the outbox in the submit tx; handler results go through recordAndEmit — no manual XADD |
| The LLM never decides on its own to block/unblock money | The LLM only returns score+reasons; the decision is the deterministic thresholds in CLASSIFY + a human in manual_review (per the principle in index.html §9) |
| No logging of secrets/PII | No images/base64/API key are logged; the scorer logs only score + request_id; the admin password appears in no log |
| Two-layer AuthZ | Queue/review sit behind RequireAdmin (aud=admin — a user token cannot be used even if leaked, doc 01 §3.1); review writes audit_logs with actor+reason; images are reachable only via a 5' signed URL |
| Biometric/KYC does not change the money rules | KYC only raises `wallet.TxnLimit` via kyc_status — no shortcut writes the ledger |

> **Interview talking point of this phase:** the same workflow engine now runs 3 kinds of flow — automatic (P2P), a human gate mid-way (org approval), and **no-transaction** (KYC) — proving that F4's nullable "transaction_id" is a deliberate design rather than technical debt. And AI is placed in the right spot: the LLM scores, deterministic thresholds + humans decide.
