# Solution Architecture — A3 Workflow Monitor & Fraud hpay-admin

Architecture for 2 near-realtime operations screens: Workflow Monitor (observability of the F4 engine) and Fraud Monitor (handling F9 flags). Guiding principle throughout: **the FE does not decide business logic** — available actions are read from the API, and the consequences (lock/cancel/reversal) are carried out by the backend through the proper engine.

## 1. Context

```mermaid
flowchart LR
  subgraph FE["hpay-admin"]
    wm["WorkflowPage\nfilter stuck/status/definition\nauto-refresh 10s"] --> wd["WorkflowDetailPage\ntimeline step_executions\n+ JSON I/O viewer"]
    fq["FraudPage\nSLA timer · multi-select"] --> rd["ResolveDialog\n3 consequence options"]
    hooksW["features/workflow/hooks.ts"] & hooksF["features/fraud/hooks.ts"] --> client["api/client.ts"]
    wm & wd --> hooksW
    fq & rd --> hooksF
  end
  client -->|"GET fraud-flags · POST resolve"| be["hpay-backend"]
  client -->|"GET instances · POST retry/compensate"| be
  be --> eng["Workflow Engine F4\n(instances · step_executions)"]
  be --> fr["Fraud module F9\n(flags · reversal via approval)"]
  fr -.->|"reverse = create approval request"| ap["Approval Engine F6\n(My Tasks — A4)"]
```

## 2. Main flow — resolving 1 fraud flag (including error branches)

```mermaid
sequenceDiagram
  autonumber
  participant O as Risk Officer
  participant P as FraudPage
  participant D as ResolveDialog
  participant API as /v1/admin/fraud-flags
  participant BE as Backend (engine/approval)

  O->>P: Open the open queue (auto-refresh 30s, SLA > 30' shown in red)
  P->>API: GET /v1/admin/fraud-flags?status=open
  API-->>P: [{id, rule_code, score, reasons, txn, created_at}]
  O->>D: "Confirm fraud" → open the consequence dialog
  O->>D: Choose 1 of 3: lock+cancel / lock+reversal / mark-only, enter reason ≥ 10 characters
  D->>API: POST /v1/admin/fraud-flags/:id/resolve {action, reason}
  alt Flag already handled elsewhere (A01/A05)
    API-->>D: 409 + new state
    D-->>O: Toast + update the row in place
  else Txn completed → reversal needed
    API->>BE: Create a reversal approval request (no direct ledger write)
    BE-->>API: 201
    API-->>D: 200 {result: "reversal_pending_approval"}
    D-->>O: Toast "pending approval in My Tasks"
  else Handled directly OK
    API-->>D: 200
    D->>P: invalidate → row updated
  end
```

## 3. State — valid actions by workflow instance status

```mermaid
stateDiagram-v2
  [*] --> running
  running --> waiting_approval : PauseHook (human gate)
  running --> failed : terminal step error
  waiting_approval --> running : approve → resume
  failed --> running : Retry step (action)
  failed --> compensating : Force fail + compensate (action, needs reason)
  compensating --> compensated
  running --> completed
  note right of failed
    FE renders buttons per allowed_actions from the API,
    does not infer from status
  end note
```

## 4. Data model delta

**No schema change.** Client state: query keys `['workflow', filters]`, `['workflow', id]` (refetch 10s), `['fraud', status]` (refetch 30s). The SLA timer is derived state (created_at → now, tick 60s) and is not stored.

## 5. Decisions & trade-offs

| Decision | Alternative | Reason |
|---|---|---|
| Auto-refresh via polling interval (10s/30s) | SSE / WebSocket push | Internal app with few users; polling through TanStack Query is sufficient and simple; the backend has no push channel yet |
| FE reads `allowed_actions` from the API | FE infers from status | The state machine lives in the backend (doc 03 §4.4); avoids drift when the engine adds states |
| Fraud consequence (lock/cancel/reversal) is a single resolve call | FE calls several APIs sequentially | The backend guarantees atomicity + goes through the approval engine for reversal — the FE never orchestrates money business logic |
| JSON I/O viewer uses a custom component (collapse + copy) | react-json-view | Narrow need; the backend already masks PII before returning (A07), so it only needs to render |

## 6. Invariants & edge cases to uphold

| ID | Situation | How it is upheld |
|---|---|---|
| A01 | Action taken just as the record changes state | 409 → toast + update the row in place, no table reload |
| A05 | 2 tabs acting at the same time | The backend checks state on every mutation; shared 409 interceptor |
| A07 | Step JSON contains PII | The backend pre-masks (`***`); the FE never receives raw data — cannot leak from DevTools |
| — | No "back door" ledger write | Reversal always goes through an approval request → engine; the FE only calls the resolve API |
| — | Force fail needs confirmation + reason | Two-step confirm dialog, reason ≥ 10 characters, audit written in the backend |
