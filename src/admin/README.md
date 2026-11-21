# hpay-admin

React + Vite + TypeScript Admin Operations SPA for HPay. Includes the transaction design manager and the offer factory.

## Features

- **Transaction Design Manager (5 Tabs)**:
  - Tab 0: Overview & General Config
  - Tab 1: Trans Definition (GL Posting Debit/Credit Rules)
  - Tab 2: Trans Validations & Input Field Rules
  - Tab 3: Action Pipeline Builder (Directional 3rd Party HTTP Gateways)
  - Tab 4: Offer Factory & Auth Matrix (`pin` / `otp` / `2fa`)
- **Offer Factory & Authorization Matrix Manager**: Manage limits and Auth levels per customer segment tier.
- **Business Wallet & Maker-Checker Approvals**: Multi-tier organizational approval inbox.
- **KYC & Fraud Operations**: AI-assisted KYC review & fraud flag management.
- **Reconciliation Runs**: Automated Bank & Ledger reconciliation summaries.

## Quick Start

```bash
npm install
npm run dev      # Starts Vite dev server at http://localhost:5173
npm run build    # Typechecks and builds production bundle in dist/
```
