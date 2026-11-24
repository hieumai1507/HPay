# hpay-mobile

React Native / Expo Mobile Wallet App for HPay — Integrated with 3-Step Low-Code Service Engine.

## Features

- **Unified Transaction Engine Hook (`useTransactionEngine`)**:
  - P2P Transfer (`P2P_TRANSFER`)
  - Money Request & Split Bill (`MONEY_REQUEST`)
  - Bank Topup (`TOPUP_BANK`)
  - Business Disbursement (`ORG_DISBURSEMENT`)
- **Security & Reliability**:
  - Client-side Idempotency Key generation
  - SecureStore token management
  - Native Blur on App Backgrounding (VCB Digibank style)

## Quick Start

```bash
npm install
npx expo start    # Starts Expo Metro bundler
npx tsc --noEmit # Runs TypeScript typecheck
```
