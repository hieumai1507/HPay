[← Implementation Plan](implementation-plan.md) · f3-ledger-wallets

# F3 Ledger & Wallets — Solution Architecture

## 1. Context

```mermaid
flowchart LR
  cl["📱 Client (JWT)"] --> mw["RequireAuth\n+ RequireIdempotencyKey (money POSTs)"]
  mw --> wh["wallet handlers\n/v1/wallets · /v1/transactions"]
  wh --> ws["wallet.Service\nEnsureMainWallet · Topup · History"]
  ws --> lg["ledger.Post(tx, txnID, legs)\n⭐ the only place allowed to touch balances"]
  lg --> pg[("PostgreSQL\nwallets · transactions · ledger_entries")]
  eng["(F4+) workflow wallet handler"] -.->|"also only calls"| lg
```

**Central principle:** `ledger.Post` is the ONLY door that changes a balance — every module (topup today, engine F5, reversal F9, vault move P2) goes through it. There is no UPDATE wallets anywhere else.

## 2. Main flow — Topup (sync) & Post

```mermaid
sequenceDiagram
  autonumber
  actor C as Client
  participant H as POST /v1/wallets/topup
  participant W as wallet.Service
  participant L as ledger.Post
  participant DB as PostgreSQL
  C->>H: {amount} + Idempotency-Key + JWT
  H->>W: Topup(userID, amount, key)
  W->>W: validate amount > 0, ≤ KYC tier limit (E13)
  W->>DB: BEGIN
  W->>DB: INSERT transactions (status=processing, key UNIQUE)
  alt key already exists (23505 — E01)
    W->>DB: ROLLBACK + SELECT old txn by key
    W-->>C: 200 old transaction (money is not credited a second time)
  else
    W->>L: Post(tx, txnID, [Dr SYSTEM_CASH_IN, Cr user wallet])
    L->>DB: SELECT ... FOR UPDATE (sorted by id — E03)
    L->>L: check non-system wallet not negative (E02) · sum Dr = sum Cr
    L->>DB: UPDATE balance + INSERT ledger_entries (running balance_after)
    W->>DB: UPDATE txn status=completed, completed_at
    W->>DB: COMMIT — all or nothing
    W-->>C: 201 {refCode, status: completed, new balance}
  end
```

## 3. State — transaction (the part F3 activates)

```mermaid
stateDiagram-v2
  [*] --> processing : Topup creates (inside tx)
  processing --> completed : ledger post OK (same tx)
  processing --> [*] : tx rollback — txn does not exist\n(sync = no dangling state)
  completed --> [*]
  note right of completed : immutable from here\nthe other branches of the enum are activated in F5/F6
```

## 4. Data model delta

No structural change — seed only. Relationships used:

```mermaid
erDiagram
  wallets ||--o{ ledger_entries : "balance = derived from"
  transactions ||--|{ ledger_entries : "Topup = 2 lines\nDr SYSTEM_CASH_IN / Cr user"
  wallets {
    varchar system_code "SYSTEM_CASH_IN goes progressively negative as users top up - the true nature of double-entry"
  }
```

## 5. Decisions & trade-offs

| Decision | Alternative | Why |
| --- | --- | --- |
| Sync topup in 1 DB tx, not through the engine | Route topup through a workflow like P2P | The bank mock answers instantly — adding 4 async hops just to look alike is over-engineering; the engine is for flows with risky steps (fraud check, approval). Trade-off: 2 code paths — accepted, both converge at ledger.Post |
| Idempotency via UNIQUE constraint + return the old txn on conflict | Full response-cache table | Simple, atomic, correct enough: the client gets back the transaction (the entity) rather than a byte-for-byte response; add a response cache when there is a real need |
| balance_after written as a running value in leg order within one Post | Write only the final balance | Audit can read each step; internal recon can match each line |
| Per-txn limits as constants in code | Limit policy table | F5 will have fee_policies — fold limits into it in one go, avoid doing it twice |

## 6. Invariants touched

| Invariant | Guaranteed at |
| --- | --- |
| Sum Dr = Sum Cr per transaction | `ledger.Post` validates before touching the DB + unit test |
| balance = SUM(ledger) per wallet | Post updates balance and entries in the same tx; the integration test re-asserts via SQL |
| No deadlock on cross transfers | Lock ordering sorted by wallet_id — E03 tests both directions concurrently |
| User wallet never negative | Check inside Post after locking + CHECK constraint as the last line — E02 |
| Retry does not double money | UNIQUE idempotency_key — E01 |
