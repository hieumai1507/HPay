[← Implementation Plan](implementation-plan.md) · f1-foundation

# F1 Foundation — Solution Architecture

## 1. Context — F1 builds the foundation (bold); the dimmed parts are later phases

```mermaid
flowchart LR
  subgraph Repo["hpay-backend"]
    main["cmd/server\n--role api|worker|admin|all"]
    cfg["platform/config\nenv 12-factor"]
    log["platform/logger\nslog + request_id"]
    dbp["platform/db\npgx pool"]
    rdc["platform/redisclient"]
    rt["api/router\nGET /healthz"]
    mig["db/migrations\n000001–000008 = FULL schema"]
    q["store/queries + sqlc"]
    future["(F2+) auth · ledger · engine · approval…"]:::dim
  end
  ci["GitHub Actions\nlint · sqlc vet · migrate↑↓↑ · test · build"]
  dc["docker-compose\npg16 · redis7 · app"]
  main --> cfg & log & rt
  rt --> dbp & rdc
  mig --> ci
  dc -.local dev.-> main
  classDef dim fill:#F3F4F6,color:#9CA3AF,stroke:#E5E7EB
```

## 2. Main flow — boot by role

```mermaid
sequenceDiagram
  autonumber
  participant OS as Process start
  participant CF as config
  participant PG as pgx pool
  participant RD as redis
  participant HTTP as Gin HTTP
  participant WK as workers (F4+)
  OS->>CF: load env (.env / environment)
  CF-->>OS: Config{role, ports, dsn...} — fail-fast if a required key is missing
  OS->>PG: pgxpool.New + Ping (timeout 5s)
  OS->>RD: redis.NewClient + Ping
  alt role = api | admin | all
    OS->>HTTP: mount routes by role + /healthz
  end
  alt role = worker | all
    OS->>WK: (F1: log "no consumers registered" — placeholder)
  end
  Note over OS: SIGTERM → graceful shutdown:\nHTTP drain 10s → close pools
```

## 3. State — no new lifecycle in F1

F1 adds no state machine (the schema defines the enums but the transition logic belongs to F3–F6). The stateDiagram is skipped per the "if applicable" rule.

## 4. Data model — full F1 schema (ER)

```mermaid
erDiagram
  users ||--o{ org_members : "is a member"
  organizations ||--o{ org_members : "has"
  users ||--o{ wallets : "owns (owner)"
  organizations ||--o{ wallets : "owns (owner)"
  wallets ||--o{ ledger_entries : "entries"
  transactions ||--|{ ledger_entries : "generates ≥2"
  transactions ||--o| transactions : "reversal_of"
  workflow_definitions ||--o{ workflow_instances : "version"
  transactions ||--|| workflow_instances : "1-1"
  workflow_instances ||--o{ step_executions : "log"
  organizations ||--o{ approval_policies : "defines"
  approval_policies ||--o{ approval_requests : "snapshot from"
  approval_requests ||--o{ approval_actions : "signatures"
  users ||--o{ approval_actions : "checker"
  users ||--o{ otp_challenges : ""
  users ||--o{ kyc_requests : ""
  transactions ||--o{ fraud_flags : ""
  reconciliation_runs ||--o{ reconciliation_items : ""
  bank_statements ||--o{ reconciliation_items : "matches"
  users ||--o{ money_requests : "requester/payer"
  money_requests ||--o| transactions : "paid_txn"
  users {
    uuid id PK
    varchar phone UK
    kyc_status kyc_status
    account_status status
  }
  wallets {
    uuid id PK
    owner_type owner_type
    uuid owner_id
    varchar system_code UK
    bigint balance "CHECK >= 0"
    varchar purpose "main|vault"
    varchar label
    bigint version
  }
  transactions {
    uuid id PK
    varchar ref_code UK
    varchar idempotency_key UK
    txn_type type
    txn_status status "incl processing_async"
    bigint amount "CHECK > 0"
    bigint fee
    uuid reversal_of FK
  }
  ledger_entries {
    bigint id PK
    uuid transaction_id FK
    uuid wallet_id FK
    entry_direction direction
    bigint amount "CHECK > 0"
    bigint balance_after
  }
  workflow_instances {
    uuid id PK
    uuid definition_id FK
    uuid transaction_id FK "UNIQUE"
    wf_status status
    jsonb context
  }
  approval_requests {
    uuid id PK
    approval_subject subject_type "org_txn|reversal|account_lock|policy_change"
    uuid subject_id
    jsonb policy_snapshot
    approval_status status
    timestamptz expires_at
  }
  money_requests {
    uuid id PK
    uuid requester_id FK
    uuid payer_id FK
    bigint amount "CHECK > 0"
    uuid split_group_id
    money_request_status status
    uuid paid_txn_id FK
    timestamptz expires_at
  }
```

## 5. Decisions & trade-offs

| Decision | Alternative | Why |
| --- | --- | --- |
| Merge the 04§2 delta into the original migrations (approval_requests generalized from the start) | Build transaction_approvals first and migrate afterwards | No environment is running yet — no reason to keep a schema that will be discarded; saves one round of churn |
| DB trigger blocks UPDATE/DELETE on ledger_entries | Rely only on review + app user permissions | The most important invariant should be enforced at the deepest layer; testable right away in CI |
| Auth tables (refresh_tokens, admin_users, audit_logs) deferred to F2 | Include them in F1 | Matches the phase boundary; F2 will use gen-migration through the proper process — also demonstrates the skill in action |
| go.sum generated on the dev machine/CI (sandbox has no Go) | Wait for a toolchain before scaffolding | Does not block progress; CI is the real compile gate |

## 6. Invariants touched

| Invariant (index.html §8) | F1 guarantees it by |
| --- | --- |
| Money is BIGINT | Every amount/balance/fee column in the 8 migrations is BIGINT + CHECK |
| Ledger is INSERT-only | Trigger `ledger_entries_append_only` + test in the CI migrate job |
| Retry does not create duplicate transactions | UNIQUE `idempotency_key` present from the first migration |
| Events are neither lost nor early | Outbox table + partial index ready for the relay (F4) |
