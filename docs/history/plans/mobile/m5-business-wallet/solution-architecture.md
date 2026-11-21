[← implementation-plan](implementation-plan.md) · m5-business-wallet

# M5 Business Wallet — Solution Architecture

## 1. Context & Screen Flow

```mermaid
flowchart TD
  subgraph ACC["👤 Account screen"]
    LIST["Organization list"]
    CRE["Button: Create Organization"]
  end
  subgraph HOME["🏠 Home screen"]
    PERS["Personal Wallet (default)\nBalance + personal shortcuts"]
    BIZ["Business Wallet\nOrg wallet balance + role-based shortcuts"]
  end
  subgraph FLOWS["💼 Maker/Checker flows"]
    MK["Maker: Create disbursement order\n(org/disburse.tsx)"]
    CK["Checker: Approve orders (My Tasks)\n(org/approvals.tsx)"]
    ADM["Admin: Policy administration\n(org/admin.tsx)"]
  end

  LIST -->|Tap to select organization| BIZ
  CRE -->|POST /v1/org| LIST
  BIZ -->|Role: Maker| MK
  BIZ -->|Role: Checker| CK
  BIZ -->|Role: Admin| ADM
  BIZ -->|Button: Back to personal wallet| PERS
```

## 2. State Management — Zustand store

To store the active organization state independently of the login session, we set up `useOrgStore`:

```
interface ActiveOrg {
  orgId: string;
  name: string;
  role: 'admin' | 'maker' | 'checker';
  checkerLevel: number;
  walletId: string;
  balance: bigint;
}

interface OrgState {
  activeOrg: ActiveOrg | null;
  switchOrg: (org: ActiveOrg) => void;
  exitOrg: () => void;
  updateBalance: (balance: bigint) => void;
}

```

## 3. Sequence Diagram — Checker Approval execution

```mermaid
sequenceDiagram
  autonumber
  actor Checker
  participant Detail as Approval detail screen
  participant API as Backend Gateway
  participant OTP as OTP entry modal

  Checker->>Detail: Tap the "Approve" button
  Detail->>API: POST /v1/org/approvals/{id}/challenge (OTP request)
  API-->>Detail: 200 OK (returns status: "otp_sent" + otp_dev in dev)
  Detail->>OTP: Open the modal requesting the OTP
  Checker->>OTP: Enter 6 digits
  OTP->>API: POST /v1/org/approvals/{id}/decide {action: "approve", otp}
  API-->>OTP: 200 OK (approved successfully, workflow resumes)
  OTP-->>Detail: Close the modal, show success message
  Detail->>Detail: Navigate back to the approvals list screen
```

## 4. Dynamic Quick Actions on Home

| Role | Displayed shortcuts | Corresponding navigation |
|---|---|---|
| **Maker** | Create disbursement order · History · Personal wallet | `/(main)/org/disburse` · `/(main)/history` · Return to personal wallet |
| **Checker** | Approve orders (count) · History · Personal wallet | `/(main)/org/approvals` · `/(main)/history` · Return to personal wallet |
| **Admin** | Top up · Administration · History · Personal wallet | `/(main)/org/topup` · `/(main)/org/admin` · `/(main)/history` · Return to personal wallet |
