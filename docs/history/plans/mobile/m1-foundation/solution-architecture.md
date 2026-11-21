# 🏗 Solution Architecture — M1 Foundation

## 1. Context — where M1 sits in the system

```mermaid
flowchart LR
  subgraph APP["hpay-mobile (M1)"]
    rt["expo-router\napp/_layout + splash gate"] --> qp["QueryClientProvider\nTanStack Query"]
    qp --> api["src/api/client.ts\nfetch wrapper + interceptors"]
    api --> gen["src/api/schema.d.ts\nGENERATED from openapi.yaml"]
    api --> ss["stores/session (Zustand)\naccess token IN-MEMORY"]
    api --> ds["stores/device\nX-Device-Id"]
    ss --> sec["expo-secure-store\nrefresh token · device id"]
    lib["src/lib\nmoney(BigInt) · errors · idempotency · polling"] --> api
    ui["src/ui\ntokens.ts + Button/AmountDisplay/StatusBadge"] --> rt
  end
  api -->|"HTTP localhost:8080"| be["backend F2 Auth\n/v1/auth/*  /healthz"]
  spec["hpay-backend\napi/openapi.yaml v0.8.0"] -.->|"npm run gen:api\nopenapi-typescript"| gen
```

## 2. Main flow — authenticated request + 401 single-flight refresh (M04/M05)

```mermaid
sequenceDiagram
  participant C as Caller (hook/screen)
  participant W as api/client (wrapper)
  participant R as refresh.ts (single-flight)
  participant S as session store
  participant K as secure-store
  participant BE as backend
  C->>W: request(path, opts)
  W->>W: attach Authorization + X-Device-Id + X-Request-Id (+Idempotency-Key for money POSTs)
  W->>BE: fetch
  alt 200/2xx
    BE-->>C: data (typed from schema)
  else first 401
    W->>R: ensureFreshToken()
    Note over R: if a refresh is already in flight →<br/>await the SAME promise (single-flight)
    R->>K: read refresh_token
    R->>BE: POST /v1/auth/refresh
    alt rotate ok
      BE-->>R: new token pair
      R->>S: set access (memory)
      R->>K: store new refresh
      W->>BE: retry EXACTLY 1 time with the new token
      BE-->>C: data
    else ERR_TOKEN_REUSED / ERR_INVALID_TOKEN (M05)
      R->>S: clear session
      R->>K: delete refresh
      R-->>C: throw SessionRevokedError(reason) → router goes to Login, KEEP the draft
    end
  else 401 after retry
    W-->>C: throw (no further retry)
  end
```

## 3. State — client-side session lifecycle

```mermaid
stateDiagram-v2
  [*] --> Booting : open app
  Booting --> SignedOut : no refresh token
  Booting --> SignedIn : has refresh → refresh succeeds
  Booting --> SignedOut : refresh fails
  SignedOut --> SignedIn : login/register + OTP (M2)
  SignedIn --> Refreshing : access expired (401)
  Refreshing --> SignedIn : rotate ok (retry 1 time)
  Refreshing --> Revoked : ERR_TOKEN_REUSED / revoke
  Revoked --> SignedOut : force logout + reason notice (M05)
  SignedIn --> SignedOut : user logout (POST /v1/auth/logout)
```

## 4. Data model delta

No DB. Durable client-side state:

```mermaid
erDiagram
  SECURE_STORE ||--o| REFRESH_TOKEN : "key mp.refresh_token"
  SECURE_STORE ||--o| DEVICE_ID : "key mp.device_id (uuid, generated once)"
  MEMORY_SESSION ||--o| ACCESS_TOKEN : "Zustand — lost when the app is killed"
  MEMORY_SESSION ||--o| USER_PROFILE : "id, phone, kyc_status"
```

## 5. Decisions & trade-offs

| Decision | Alternatives | Why |
|---|---|---|
| Thin hand-written fetch wrapper on top of the openapi-typescript types | openapi-fetch, axios, orval | Full control over the interceptors (single-flight, idempotency, retry-once) — this logic is a hard requirement of M01/M04; fewer dependencies |
| Access token in Zustand memory, NOT persisted | secure-store for both tokens | Rule doc 02 §5; access token lasts 15 minutes — losing it when the app is killed is acceptable, refresh handles the rest |
| Money = BigInt end-to-end, formatted with string ops | Number + Intl.NumberFormat | Index invariant §8: no float; Intl takes a number → write our own thousands formatting on strings |
| Polling helper + TransactionWatcher interface | websocket/SSE | Doc 02 §6: v1 has no realtime; leave an interface to swap in later |
| Idempotency-Key generated when ENTERING the confirm screen (hook), not when the button is pressed | generate inside onPress | M01: double-tap/re-render still yields 1 key → backend dedupes and returns the original 200 |

## 6. Invariants touched (index.html §8)

| Invariant | How M1 upholds it |
|---|---|
| Money is BIGINT minor units — no float | `lib/money.ts` exposes only a string/BigInt API; test M06 blocks the Number path |
| A retry must not create a duplicate transaction | Idempotency-Key once per flow (M01) + retry-once after refresh keeps the same key |
| Biometric only unlocks the app — money always needs PIN/OTP | Foundation does not put biometric into the transaction auth flow; noted in the gen-screen skill |

No token/OTP/balance is logged to the console — `lib/log.ts` redacts + strips in production; the balance is HIDDEN by default (UI state in M3).

hpay-mobile · docs/plans/m1-foundation · 17/07/2026
