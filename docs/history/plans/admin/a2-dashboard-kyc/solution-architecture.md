# Solution Architecture — A2 Dashboard & KYC hpay-admin

Architecture for the overview Dashboard and the KYC review flow (queue → detail → approve/reject). Corresponding backend: F8 KYC + AI. Built on top of A1 (shell, client, store, Table).

## 1. Context — the parts of the system this phase touches

```mermaid
flowchart LR
  subgraph FE["hpay-admin"]
    dash["DashboardPage\nKPI + alert cards"] --> kq["KYCPage\n3 tabs by ai_verdict"]
    kq --> kd["KYCDetailPage\n2 images + AI reasons + decision"]
    hooks["features/kyc/hooks.ts\nuseKYCQueue · useKYCDetail · useKYCReview"]
    kq & kd --> hooks
    hooks --> client["api/client.ts"]
  end
  client -->|"GET /v1/admin/kyc?status=…"| be["hpay-backend F8"]
  client -->|"POST /v1/admin/kyc/:id/review"| be
  kd -->|"img src = HMAC-signed URL 5'"| files["GET /v1/files/{key}\n(self-auth, no JWT)"]
  be --> pg[("PostgreSQL\nkyc_requests")]
  files --> storage[("Storage\nCCCD + selfie images")]
```

## 2. Main flow — reviewing one KYC application (including error branches)

```mermaid
sequenceDiagram
  autonumber
  participant R as Reviewer
  participant Q as KYCPage (queue)
  participant D as KYCDetailPage
  participant API as /v1/admin/kyc
  participant F as /v1/files (HMAC)

  R->>Q: Open the manual_review tab
  Q->>API: GET /v1/admin/kyc?status=manual_review
  API-->>Q: [{id, user, ai_score, ai_reasons, wait_ms}]
  R->>D: Enter / click (J/K navigation)
  D->>F: GET CCCD + selfie images (5-minute signed URL)
  alt Signed URL expired (A04)
    F-->>D: 410 ERR_LINK_EXPIRED
    D->>API: refetch detail → receive new URL (auto, once)
  else OK
    F-->>D: image bytes
  end
  R->>D: Press A (approve) or R (reject + reason ≥ 10 characters)
  D->>API: POST /v1/admin/kyc/:id/review {decision, reason}
  alt Another reviewer already handled it first (A01)
    API-->>D: 409 + new state
    D-->>R: Toast "already handled" + update the row in place, no table reload
  else Success
    API-->>D: 200
    D->>Q: invalidate cache → move to the next case
  end
```

## 3. State — KYC application lifecycle as seen from admin

```mermaid
stateDiagram-v2
  [*] --> manual_review : AI verdict requires a human reviewer
  [*] --> auto_approved : AI auto-approved (spot-check only)
  manual_review --> approved : reviewer Approve
  manual_review --> rejected : reviewer Reject (+ reason)
  note right of manual_review
    FE does not change state itself —
    it only renders per the API; 409 = state changed elsewhere
  end note
```

## 4. Data model delta

**No schema change** — read/write through the existing F8 API. Client cache: query keys `['kyc', status]` and `['kyc', id]`; the review mutation invalidates both.

## 5. Decisions & trade-offs

| Decision | Alternative | Reason |
|---|---|---|
| Transaction/volume KPIs use mock data in A2 | Wait for the backend to build an analytics API | The backend has no analytics endpoint yet; the dashboard can still demo the layout, and alert cards use real counts from the KYC/fraud queues. Documented explicitly to avoid mistaking them for real numbers |
| J/K/A/R shortcuts handled at page level | Global hotkey manager | Only 2 screens need them; avoids conflicts when focus is in an input (disabled while typing the reason) |
| Images loaded directly via HMAC-signed URL | Proxy through FE / fetch blob | The backend already designed a 5-minute self-auth URL (F8); a direct `<img>` is the simplest, retry once on expiry (A04) |
| Pure CSS BarChart | Recharts | Only one chart in A2 — not worth adding a dependency; reconsider Recharts if A4 needs many charts |

## 6. Invariants & edge cases to uphold

| ID | Situation | How it is upheld |
|---|---|---|
| A01 | 2 reviewers open the same application, the later one acts | Backend returns 409 + new state → toast + update the row in place |
| A02 | Long queue, new data pushed in while on a later page | Cursor pagination from the API (not offset); J/K navigates over the cached list |
| A04 | Signed image URL expires while viewing | onError → request a new URL exactly once, beyond that show a message |
| — | Reject requires a reason ≥ 10 characters | Validate in FE before POST; the backend remains the real enforcer |
| — | Full audit | Every decision goes through the review API — no shortcut on the FE side |
