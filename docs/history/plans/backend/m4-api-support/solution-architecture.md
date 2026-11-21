[← implementation-plan](implementation-plan.md) · m4-api-support

# M4 API Support — Solution Architecture

## 1. Context

```mermaid
flowchart TB
  subgraph MOB["📱 hpay-mobile M4"]
    QRS["Receive-money QR screen\n(shows my own QR)"]
    SCAN["Scan someone else's QR"]
    REM["🔔 Remind button (mockup 29)"]
  end
  subgraph BE["hpay-backend (changed parts)"]
    MINT["GET /v1/users/me/qr\nqr.Mint"]
    RESOLVE["POST /v1/qr/resolve\nqr.Resolve → receiver_token"]
    TRF["POST /v1/transactions/transfer\n+ receiver_token (oneOf phone)"]
    RMD["POST /v1/money-requests/{id}/remind\nguarded UPDATE 24h"]
    SIGNER["storage.Signer (F8, reused)\nHMAC-SHA256 + TTL"]
  end
  QRS --> MINT --> SIGNER
  SCAN --> RESOLVE --> SIGNER
  SCAN -->|"token"| TRF
  TRF --> ENG["transfer.Service → workflow engine\n(unchanged since F5)"]
  REM --> RMD --> DB[("money_requests\n+ last_reminded_at")]
```

## 2. Main flow — QR mint → resolve → transfer

```mermaid
sequenceDiagram
  participant B as Device B (receiver)
  participant A as Device A (scanner)
  participant API as Backend
  B->>API: GET /v1/users/me/qr (JWT B)
  API-->>B: {payload: b64({user_id,name_masked,phone_masked,exp}), signature, expires_at(15')}
  B->>A: shows the QR (offline — content is payload+signature)
  A->>API: POST /v1/qr/resolve {payload, signature} (JWT A, rate-limit 30/min)
  alt wrong signature (fake QR / modified payload)
    API-->>A: 400 ERR_QR_INVALID (check sig BEFORE exp — leaks nothing)
  else past TTL
    API-->>A: 410 ERR_QR_EXPIRED
  else valid
    API-->>A: {name_masked, phone_masked, receiver_token(10'), expires_at}
  end
  A->>API: POST /v1/transactions/transfer {receiver_token, amount, note} + Idempotency-Key
  API->>API: VerifyReceiverToken → user_id B (server-side, the client never sees the real phone)
  API->>API: transfer.TransferToUser(...) — F5 flow intact (validate→fraud→move→confirm)
  API-->>A: 202 {ref_code, ...} → poll as in M3
```

## 3. Main flow — remind + 24h guard

```mermaid
sequenceDiagram
  participant R as Requester
  participant API as Backend
  participant DB as postgres
  R->>API: POST /v1/money-requests/{id}/remind (JWT)
  API->>DB: UPDATE money_requests SET last_reminded_at=now()<br/>WHERE id AND requester_id=me AND status='pending'<br/>AND (last_reminded_at IS NULL OR < now()-24h) RETURNING id
  alt 1 row
    API-->>R: 200 {last_reminded_at} — the payer sees it when opening the inbox (polling)
  else 0 rows
    API->>DB: SELECT to classify the error
    API-->>R: 404 not-found / 403 not-requester / 409 not-pending / 429 ERR_REMIND_TOO_SOON
  end
  Note over DB: The guard lives INSIDE the UPDATE statement — of 2 concurrent reminds only 1 wins,<br/>no separate lock needed (same WHERE status guard pattern as the rest of the project)
```

There is no new state machine (money_request_status is unchanged; `last_reminded_at` is metadata, not a state).

## 4. Data model delta

```mermaid
erDiagram
  money_requests {
    uuid id PK
    uuid requester_id FK
    uuid payer_id FK
    bigint amount
    money_request_status status
    timestamptz last_reminded_at "NEW - nullable, 000019"
    timestamptz expires_at
  }
  users ||--o{ money_requests : "requester/payer"
```

The QR/token are fully stateless (HMAC) — no new table, tokens are not stored.

## 5. Decisions & trade-offs

| Decision | Alternative | Why |
| --- | --- | --- |
| QR + receiver_token stateless via HMAC (reuse storage.Signer, add generic Sign/VerifyMsg functions) | A qr_tokens table in the DB; or JWT | The Signer already exists, is tested, and verify-sig-before-exp is to standard; stateless = no garbage cleanup, no extra table; JWT is heavy and gets mixed up with auth tokens |
| Transfer accepts `receiver_token` as oneOf `receiver_phone`, refactor `TransferToUser(receiverID)` | A separate transfer endpoint for QR | 1 endpoint, 1 money flow — identical idempotency, fee, workflow; old clients do not break; the token purpose is later reusable for pay-by-counterparty (M3 backlog) |
| The QR payload contains user_id + masked data, NOT the real phone | Contain only an opaque token | Doc 02 §5 requires the payload to carry user_id + masked phone so the confirm screen can display without a round trip; money can still only move via a signed token |
| Remind = column `last_reminded_at` + guarded UPDATE, no notifications infra | A notifications table + push | Doc 02 §6: polling is enough for v1; the payer sees the reminder via the existing inbox; notifications infra is a separate backlog, rushing it would be half-baked |
| Resolve requires JWT + RateLimitUser 30/min | Public without auth | Prevents probing/brute-force of the payload; the scanner is surely a logged-in user (they want to transfer money after all) |

## 6. Invariants touched (index.html §8)

| Invariant | Upheld by |
| --- | --- |
| Every balance change goes through ledger.Post / the engine | Transfer-by-token flows into the same transfer.Service → F5 workflow; QR/remind do not touch money |
| Money-path idempotency | Transfer-by-token still goes through RequireIdempotencyKey + UNIQUE — unchanged |
| PII masking server-side | Resolve/QR only return MaskName/MaskPhone (m3); the real phone never leaves the server in the QR flow |
| State transition guarded | Remind does not change status; the 24h guard is atomic inside the UPDATE |
| Error contract ERR_* | 3 new codes registered in errmap + openapi in the same commit |

---

m4-api-support · solution architecture v1 · 18/07/2026
