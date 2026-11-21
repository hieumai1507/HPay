[← Implementation Plan](implementation-plan.md) · f5-p2p-fee

# F5 P2P Transfer & Fee — Solution Architecture

## 1. Main flow — P2P end-to-end

```mermaid
sequenceDiagram
  autonumber
  actor U as User
  participant API as POST /v1/transactions/transfer
  participant T as transfer.Service
  participant E as engine (F4)
  participant W as worker handlers
  participant L as ledger.Post
  U->>API: {receiver_phone, amount, note} + Idempotency-Key
  API->>T: Transfer()
  T->>T: resolve receiver (phone→wallet), fee = fee.Calc(tier, amount)\nsoft checks: self? locked? limit? balance?
  T->>T: 1 DB tx: INSERT transactions(processing, fee)\n+ engine.Start(p2p_transfer) → outbox(VALIDATE)
  T-->>U: 202 {ref_code, fee} — app polls the timeline
  W->>W: VALIDATE: user status × 2, limit, re-soft-balance
  W->>W: FRAUD_CHECK: safe stub (F9 replaces it with the real one)
  W->>L: MOVE_FUNDS: re-check status (E12) then atomic Post:\nDr sender (amount+fee) · Cr receiver amount · Cr SYSTEM_FEE fee
  W->>W: CONFIRM: processing → completed (guarded)
  Note over E,W: any step that fails terminally AFTER MOVE_FUNDS\n→ compensation walks backwards (section 2)
```

## 2. Compensation — backward walk with a queue

```mermaid
flowchart TB
  f["Step X fails terminally\non_failure = compensate"] --> q["Engine builds a queue of steps\nBEFORE X that are completed and compensatable\n(stored in context._compensate, status=compensating)"]
  q --> d["Dispatch compensate event\n(StartEvent.compensate=true, attempt≥101)"]
  d --> h["Handler receives StepInput.Compensating\nwallet: reverses the entries from the txn row\nvalidator/fraud/confirm: no-op"]
  h --> r["Result event → engine (guard accepts compensating)"]
  r -->|"queue not empty"| d
  r -->|"queue empty"| done["instance = compensated\ntxn = failed (money refunded)"]
  r -->|"compensate fails"| manual["instance = failed\nlog ERROR — admin handles it (A3)"]
```

## 3. State — transaction with compensation

```mermaid
stateDiagram-v2
  [*] --> processing : transfer created (idempotent)
  processing --> completed : CONFIRM ok
  processing --> failed : VALIDATE/FRAUD/MOVE_FUNDS fails\n(money has NOT moved - atomic MOVE_FUNDS)
  processing --> failed : a step after MOVE_FUNDS fails\n→ only becomes failed once compensation has refunded the money
  completed --> [*]
  failed --> [*]
```

## 4. Data model delta

No schema change — seed only (definition + fee policies). Fee is selected by: matching `txn_type` + (`kyc_tier` = my tier PREFERRED over a NULL tier) + is_active, latest version.

## 5. Decisions & trade-offs

| Decision | Alternative | Why |
| --- | --- | --- |
| MOVE_FUNDS as 1 atomic step instead of DEBIT+CREDIT | Keep 2 steps as in docs 00 | Removes the dangling-money state by construction; compensation is only needed for non-money steps after it — far fewer failure paths |
| Compensate reverses from the txn row (deterministic) | Store legs in the step output and read them back | The txn row is the immutable source of truth (amount, fee, 2 wallets); does not depend on the output format |
| Fee computed BEFORE creating the txn, frozen into the fee column | Compute at MOVE_FUNDS | The user sees exactly the number that was previewed; changing a policy midway does not affect running transactions (like a policy snapshot) |
| Attempt ≥ 101 for compensation | Dedicated phase column | No migration needed; unique(instance,step,attempt) remains the idempotency anchor in both directions |

## 6. Invariants touched

| Invariant | Guarantee |
| --- | --- |
| Sum Dr = Sum Cr | 3 forward legs balance (amount+fee = amount + fee); the 3 reversing legs balance too — E07 asserts ledger net 0 |
| No negative wallet | MOVE_FUNDS goes through ledger.Post (check after lock) — inherits E02 |
| Retry does not duplicate | Idempotency-Key at the API + unique step attempt in the engine — inherits E01/E04/E06 |
| Ledger append-only | Compensation = INSERT reversing entries, no UPDATE/DELETE |
