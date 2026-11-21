---
name: auth
description: "Skill for the Auth area of hpay. 33 symbols across 11 files."
---

# Auth

33 symbols | 11 files | Cohesion: 88%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how GenerateOTPCode, HashPassword, VerifyPassword work
- Modifying auth-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/auth/service.go` | Register, Login, RequestOTP, RequestOTPCode, issueTokens (+5) |
| `hpay-backend/internal/auth/otp_test.go` | newChallenge, TestEdge_E11_OTPBruteForce, TestOTPHappyPath, TestOTPExpired, TestOTPHashBoundToChallenge |
| `hpay-backend/internal/auth/otp.go` | GenerateOTPCode, HashOTP, CheckOTP |
| `hpay-backend/internal/auth/refresh.go` | NewRefreshToken, HashRefreshToken, DecideRotation |
| `hpay-backend/internal/auth/jwt_test.go` | TestJWTRoundtrip, TestJWTExpired, TestJWTWrongAudience |
| `hpay-backend/internal/auth/password.go` | HashPassword, VerifyPassword |
| `hpay-backend/internal/auth/password_test.go` | TestPasswordRoundtrip, TestPasswordSaltUnique |
| `hpay-backend/internal/store/gen/auth.sql.go` | InsertRefreshToken, MarkRefreshTokenUsed |
| `hpay-backend/internal/auth/admin.go` | Login |
| `hpay-backend/internal/auth/refresh_test.go` | TestDecideRotation |

## Entry Points

Start here when exploring this area:

- **`GenerateOTPCode`** (Function) — `hpay-backend/internal/auth/otp.go:27`
- **`HashPassword`** (Function) — `hpay-backend/internal/auth/password.go:25`
- **`VerifyPassword`** (Function) — `hpay-backend/internal/auth/password.go:38`
- **`TestPasswordRoundtrip`** (Function) — `hpay-backend/internal/auth/password_test.go:4`
- **`TestPasswordSaltUnique`** (Function) — `hpay-backend/internal/auth/password_test.go:17`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `GenerateOTPCode` | Function | `hpay-backend/internal/auth/otp.go` | 27 |
| `HashPassword` | Function | `hpay-backend/internal/auth/password.go` | 25 |
| `VerifyPassword` | Function | `hpay-backend/internal/auth/password.go` | 38 |
| `TestPasswordRoundtrip` | Function | `hpay-backend/internal/auth/password_test.go` | 4 |
| `TestPasswordSaltUnique` | Function | `hpay-backend/internal/auth/password_test.go` | 17 |
| `NewRefreshToken` | Function | `hpay-backend/internal/auth/refresh.go` | 14 |
| `HashRefreshToken` | Function | `hpay-backend/internal/auth/refresh.go` | 23 |
| `DecideRotation` | Function | `hpay-backend/internal/auth/refresh.go` | 41 |
| `TestDecideRotation` | Function | `hpay-backend/internal/auth/refresh_test.go` | 7 |
| `HashOTP` | Function | `hpay-backend/internal/auth/otp.go` | 37 |
| `CheckOTP` | Function | `hpay-backend/internal/auth/otp.go` | 54 |
| `TestEdge_E11_OTPBruteForce` | Function | `hpay-backend/internal/auth/otp_test.go` | 18 |
| `TestOTPHappyPath` | Function | `hpay-backend/internal/auth/otp_test.go` | 37 |
| `TestOTPExpired` | Function | `hpay-backend/internal/auth/otp_test.go` | 44 |
| `TestOTPHashBoundToChallenge` | Function | `hpay-backend/internal/auth/otp_test.go` | 52 |
| `NewJWTIssuer` | Function | `hpay-backend/internal/auth/jwt.go` | 28 |
| `TestJWTRoundtrip` | Function | `hpay-backend/internal/auth/jwt_test.go` | 9 |
| `TestJWTExpired` | Function | `hpay-backend/internal/auth/jwt_test.go` | 25 |
| `TestJWTWrongAudience` | Function | `hpay-backend/internal/auth/jwt_test.go` | 33 |
| `Login` | Method | `hpay-backend/internal/auth/admin.go` | 33 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `Login → HashRefreshToken` | cross_community | 4 |
| `NewRouter → JWTIssuer` | cross_community | 3 |
| `Login → TokenPair` | intra_community | 3 |
| `Login → InsertRefreshTokenParams` | intra_community | 3 |
| `Login → Ts` | intra_community | 3 |
| `VerifyOTP → HashOTP` | intra_community | 3 |
| `RequestOTP → GetActiveOTPParams` | intra_community | 3 |
| `RequestOTP → GenerateOTPCode` | intra_community | 3 |
| `RequestOTP → InsertOTPParams` | intra_community | 3 |
| `RequestOTP → HashOTP` | cross_community | 3 |

## How to Explore

1. `context({name: "GenerateOTPCode"})` — see callers and callees
2. `query({search_query: "auth"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
