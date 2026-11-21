[← Implementation Plan](implementation-plan.md) · f2-auth

# F2 Auth & Users — Solution Architecture

## 1. Context

```mermaid
flowchart LR
  cl["📱 Client"] -->|"/v1/auth/*"| rl["RateLimit\nRedis fixed window"]
  rl --> h["auth handlers"]
  h --> svc["auth.Service"]
  svc --> pw["password\nargon2id"]
  svc --> jw["jwt\nHS256 access 15'"]
  svc --> rt["refresh tokens\nopaque + SHA-256, family rotation"]
  svc --> otp["OTP core\nhash · TTL 120s · 3 attempts"]
  svc --> st["store (sqlc)"]
  st --> pg[("PostgreSQL\nusers · refresh_tokens\notp_challenges · audit_logs")]
  ra["RequireAuth middleware"] -.->|"every protected route from F3+"| jw
```

## 2. Main flow — Refresh rotation + theft detection

```mermaid
sequenceDiagram
  autonumber
  actor C as Client
  participant API as /v1/auth/refresh
  participant S as auth.Service
  participant DB as PostgreSQL
  C->>API: refresh_token (opaque)
  API->>S: Refresh(sha256(token))
  S->>DB: SELECT by token_hash
  alt does not exist / revoked
    S-->>C: 401 ERR_INVALID_TOKEN
  else already used before (REUSE = token was stolen)
    S->>DB: UPDATE revoke the whole family_id
    S->>DB: INSERT audit_logs (auth.token_reuse)
    S-->>C: 401 ERR_TOKEN_REUSED — client forced to log in again
  else expired
    S-->>C: 401 ERR_TOKEN_EXPIRED
  else valid
    S->>DB: tx: mark used_at (WHERE used_at IS NULL — race protection)\n+ INSERT new token in the same family
    S-->>C: 200 new access + new refresh
  end
```

## 3. State — OTP challenge

```mermaid
stateDiagram-v2
  [*] --> active : create (hash code, TTL 120s)\ncooldown 30s per creation
  active --> consumed : correct code & not expired
  active --> exhausted : 3rd wrong attempt (E11)\npermanently invalidated
  active --> expired : TTL exceeded
  consumed --> [*]
  exhausted --> [*]
  expired --> [*]
  note right of exhausted : 5 consecutive exhausted challenges\n→ purpose locked for 15 minutes (used by F6 for approve)
```

## 4. Data model delta

```mermaid
erDiagram
  users ||--o{ refresh_tokens : "logs in"
  users ||--o{ audit_logs : "actor (or SYSTEM_ACTOR)"
  refresh_tokens {
    uuid id PK
    uuid user_id FK
    uuid family_id "rotation chain - revoke the whole chain on reuse"
    text token_hash UK "sha256, raw token is not stored"
    varchar device_id
    timestamptz expires_at
    timestamptz used_at "NULL = not used yet"
    timestamptz revoked_at
  }
  admin_users {
    uuid id PK
    varchar email UK
    admin_role role "viewer|kyc_reviewer|risk_officer|ops|superadmin"
  }
  audit_logs {
    bigint id PK
    uuid actor_id "NOT NULL - system uses a fixed uuid"
    varchar actor_type "user|admin|system"
    varchar action
    varchar resource
    jsonb before
    jsonb after
  }
```

## 5. Decisions & trade-offs

| Decision | Alternative | Why |
| --- | --- | --- |
| Opaque refresh token (random 256-bit) stored as SHA-256, not a JWT | Refresh token is also a JWT | Opaque tokens can be revoked individually/per family; a JWT refresh token cannot be revoked before expiry |
| Theft detection by family | Only reject the old token | If the thief uses the token before the victim, rejecting a single token does not help; revoking the family forces both sides to re-login — OWASP standard |
| Single HS256 secret | RS256 key pair | One service issues and verifies its own tokens — asymmetric keys are not needed; switch to RS256 when services are split (the interface already hides it) |
| Login failures return a generic ERR_INVALID_CREDENTIALS | Distinguish non-existent user | Prevents user enumeration |
| Fixed-window rate limit with Redis INCR+EXPIRE | Precise sliding window | Sufficient for the purpose; simple = fewer bugs; upgrade later if needed |

## 6. Invariants touched

| Invariant | How it is guaranteed |
| --- | --- |
| OTP/password/token never appear in logs | Only id/purpose are logged; OTP dev-mode prints the code through the single APP_ENV=dev branch; fintech-review #7 scans for it |
| OTP is stored hashed | SHA-256 of code + challenge-id salt; constant-time comparison |
| Every sensitive action is audited | token_reuse, login_failed (counted); account lock is not in F2 but the schema is ready |
