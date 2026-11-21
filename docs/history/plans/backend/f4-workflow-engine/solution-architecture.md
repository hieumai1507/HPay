[← Implementation Plan](implementation-plan.md) · f4-workflow-engine

# F4 Workflow Engine — Solution Architecture

## 1. Context — components and event flow

```mermaid
flowchart LR
  api["API / any module\nengine.Start(defName, txnID, ctx)"] -->|"same DB tx:\ninstance + outbox(step.start)"| pg[("PostgreSQL")]
  rel["outbox relay\nclaim SKIP LOCKED\navailable_at <= now"] --> pg
  rel -->|"XADD"| st1[["wf:steps:echo"]]
  rel -->|"XADD"| st2[["wf:steps:wallet (F5)"]]
  rel -->|"XADD"| ev[["wf:events"]]
  st1 --> w1["step worker (group: workers)\nidempotency → handler(timeout)\n→ step_execution + outbox(result)\n→ XACK"]
  w1 --> pg
  ev --> ec["engine consumer (group: engine)\nOnResult: advance/retry/fail/pause\n→ outbox(next step.start) → XACK"]
  ec --> pg
  rec["recovery (60s)\nXAUTOCLAIM dead pending\n+ sweep lease of stuck instances"] --> st1
  rec --> pg
```

## 2. Main flow — full lifecycle of one auto step (including failure branches)

```mermaid
sequenceDiagram
  autonumber
  participant EN as engine
  participant OB as outbox→relay
  participant ST as wf:steps:H
  participant W as step worker
  participant DB as PostgreSQL
  participant EV as wf:events
  EN->>OB: step.start {inst, step, attempt}
  OB->>ST: XADD
  ST->>W: XREADGROUP
  W->>DB: does step_executions (inst,step,attempt) already exist?
  alt already completed (redelivery — E04/E06)
    W->>OB: re-emit the old result (no side-effect)
  else not yet
    W->>W: handler.Execute(ctx, timeout_ms)
    alt ok
      W->>DB: tx: side-effects + step_execution(completed) + outbox(result ok)
    else RetryableError
      W->>DB: step_execution(failed, attempt) + outbox(result failed retryable)
    else terminal
      W->>DB: step_execution(failed) + outbox(result failed terminal)
    end
  end
  W->>ST: XACK
  EV->>EN: result event
  EN->>DB: SELECT instance FOR UPDATE\nguard: status=running AND current_step matches
  alt ok → more steps remain
    EN->>OB: step.start(next step, attempt=1)
  else ok → no more steps
    EN->>DB: instance=completed
  else failed retryable & attempt < retry limit
    EN->>OB: step.start(same step, attempt+1)\navailable_at = now + 2^attempt s (capped at 60s)
  else failed terminal
    EN->>DB: per on_failure: failed / compensating / manual\n+ FailTransactionGuarded if there is a txn
  end
```

## 3. State — workflow instance (F4 fully activates it except resume)

```mermaid
stateDiagram-v2
  [*] --> running : Start()
  running --> running : step completed → advance / retry
  running --> waiting_approval : hits a human_approval step\n(resume in F6)
  running --> completed : no more steps
  running --> failed : on_failure=fail / manual
  running --> compensating : on_failure=compensate
  compensating --> compensated : every step needing reversal has been reversed\n(F4: framework, F5: money execution)
  running --> recovering : sweep lease (E20)
  recovering --> running : re-emit the current step
  completed --> [*]
  failed --> [*]
  compensated --> [*]
```

## 4. Data model delta

```mermaid
erDiagram
  outbox {
    bigint id PK
    varchar event_type "workflow.step.start | workflow.step.result"
    jsonb payload "handler lives in the payload so the relay can route"
    timestamptz available_at "NEW - retry backoff"
    timestamptz published_at "NULL = not yet published"
  }
  workflow_instances ||--o{ step_executions : "unique(instance,step,attempt) = idempotency anchor"
```

## 5. Decisions & trade-offs

| Decision | Alternative | Why |
| --- | --- | --- |
| Backoff via `outbox.available_at` | asynq delayed task / Redis ZSET | A retry is a decision already committed in the DB — keeping it in the DB preserves the invariant "an event is not lost after commit"; the relay only needs one more WHERE condition |
| The worker re-emits the result event on redelivery instead of staying silent | Ignore the duplicate message | If a crash happens AFTER step_execution is written but BEFORE the relay emits the result, staying silent = the engine waits forever; re-emitting is always safe because the engine is idempotent (E06) |
| Guard advance by (status, current_step) inside FOR UPDATE | Dedup by event id | No dedup table needed: the instance state ITSELF is the source of truth; stale events neutralize themselves |
| Lease recovery = UPDATE…WHERE updated_at < threshold RETURNING | Advisory lock / separate flag | Learned from TMDS (pending→attempt): 1 SQL statement, race-safe, 2 sweepers never claim the same instance — exactly E20 |
| Test handlers (echo/flaky/boom) instead of waiting for business handlers | Test the engine together with F5 | The engine is proven independently; F5 only has to deal with business logic |

## 6. Invariants touched

| Invariant | Guaranteed at |
| --- | --- |
| An event is not emitted before commit and not lost after commit | outbox is written in the same tx as the instance/step_execution; the relay only reads committed rows — E05 |
| Exactly-once effect on at-least-once delivery | unique(instance,step,attempt) + engine guard — E04/E06 |
| No dangling transaction is forgotten | XAUTOCLAIM + sweep lease — E20 |
| State changes go through guarded transitions | Every UPDATE of instance/transaction carries a WHERE on a valid status |
