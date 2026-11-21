# Solution Architecture — A4 Recon, Orgs, Audit & AI Settings hpay-admin

Final phase of hpay-admin: Reconciliation (F9), Organizations & Policy (F6), Audit Log, and **AI Settings — the admin front-end of F11 Dynamic AI Config** (backend: `src/backend/docs/plans/f11-dynamic-ai-config/`).

## 1. Context

```mermaid
flowchart LR
  subgraph FE["hpay-admin"]
    rc["ReconPage\ndate picker · KPI · AI summary\n· export xlsx"]
    og["OrgsPage\norg list + tiers builder\n+ snapshot warning"]
    au["AuditPage\nread-only · filter · export CSV"]
    ai["AISettingsPage (F11)\nkeys CRUD + function routing"]
    rc & og & au & ai --> client["api/client.ts"]
  end
  client -->|"recon/run · recon/runs/:date"| be["hpay-backend"]
  client -->|"org · org/:id/policies"| be
  client -->|"admin/ai/keys · admin/ai/settings"| be
  rc -->|"HMAC-signed URL"| files["GET /v1/files/{key}\nxlsx report"]
  be --> pg[("PostgreSQL\nrecon_runs · org_policies\naudit_logs · ai_keys\nai_function_settings")]
  be -.->|"ai_function_settings routes"| wf["Workflow steps\nkyc_scorer · fraud_advisor (F11)"]
```

## 2. Main flow A — running reconciliation and exporting the report

```mermaid
sequenceDiagram
  autonumber
  participant A as Ops
  participant P as ReconPage
  participant API as /v1/admin/recon
  participant F as /v1/files

  A->>P: Select a date → "Run reconciliation"
  P->>API: POST /v1/admin/recon/run {date}
  alt Date already run
    API-->>P: 409 ERR_RECON_ALREADY_RAN
    P->>API: GET /v1/admin/recon/runs/:date (show the previous run)
  else New run
    API-->>P: 201 {run_id} → poll 2s
    loop until the run finishes
      P->>API: GET /v1/admin/recon/runs/:date
    end
    API-->>P: {run, items, ai_summary, report_url}
  end
  P-->>A: KPI + AI summary + mismatch table
  A->>P: "Export Excel"
  P->>F: GET report_url (signed, 5 minutes)
  F-->>A: Download xlsx
```

## 3. Main flow B — dynamic AI configuration (F11)

```mermaid
sequenceDiagram
  autonumber
  participant SA as Superadmin
  participant P as AISettingsPage
  participant API as /v1/admin/ai
  participant WF as Workflow step (kyc_scorer)

  SA->>P: Open the AI Settings page
  P->>API: GET /v1/admin/ai/keys + GET /v1/admin/ai/settings (in parallel)
  API-->>P: keys (api_key masked) + settings per function
  SA->>P: Add a new key {name, provider, api_key}
  P->>API: POST /v1/admin/ai/keys
  API-->>P: 200 → invalidate ['ai','keys']
  SA->>P: Assign kyc_scoring → provider=openai, model=gpt-4o, the newly created key
  P->>API: PUT /v1/admin/ai/settings {function_name, provider, model_name, key_id}
  alt key_id points to a disabled key
    API-->>P: 400 ERR_KEY_INACTIVE → show the error next to the key select
  else key does not exist
    API-->>P: 404 ERR_KEY_NOT_FOUND
  else OK
    API-->>P: 200 → toast
    Note over WF: The next KYC run queries settings → uses the new provider/model/key immediately (no cache — F11 design)
  end
```

## 4. State — AI key lifecycle (F11, as seen from admin)

```mermaid
stateDiagram-v2
  [*] --> active : POST /ai/keys (is_active=true)
  active --> inactive : PUT is_active=false (kill-switch)
  inactive --> active : PUT is_active=true
  note right of inactive
    A setting pointing at an inactive key →
    the workflow step returns ERR_AI_PROVIDER_UNAVAILABLE;
    the UI must warn immediately when disabling a key that is in use
  end note
```

## 5. Data model delta

**FE does not change the schema.** The related tables already exist from the backend: `recon_runs`, `org_policies` (snapshot — E09), `audit_logs` (immutable), and the F11 pair `ai_keys` / `ai_function_settings` (detailed ERD in the f11 solution-architecture). Client cache keys: `['recon', date]`, `['orgs']`, `['policies', orgId]`, `['ai','keys']`, `['ai','settings']`.

## 6. Decisions & trade-offs

| Decision | Alternative | Reason |
|---|---|---|
| AI Settings lives in A4, not a separate phase | A separate phase A5 for F11 | Only 1 CRUD page + 2 forms; backend F11 already has all the logic — thin FE. But the docs must describe it fully (sections 3–4 here) instead of a one-line task |
| api_key is write-only, never read back raw | Show the full key to superadmin | The backend returns it masked; leaving it blank on update = keep the old key. Reduces leak risk via DevTools/screenshots |
| Disabling a key that settings point to → warning dialog | Block disabling outright | The kill-switch must be fast when a key leaks (F11 design: no cache, takes effect immediately); only warn about the consequence (the AI step will fail) and then allow it |
| Audit export CSV client-side from loaded data | Backend export endpoint | Sufficient for the current scope; if a full export is needed → backend async job (A06) |
| The policy snapshot warning shows the number of in-flight requests | No warning | Matches E09 semantics: pending requests keep the old policy — the admin must see this clearly before saving |

## 7. Invariants & edge cases to uphold

| ID | Situation | How it is upheld |
|---|---|---|
| A05 | 2 tabs editing a policy / AI setting | Backend state-check; 409 handled via the shared interceptor |
| A06 | Heavy export | The xlsx is pre-generated by the backend, the FE only downloads via a signed URL; the audit CSV only serializes data already loaded |
| E09 | Policy snapshot for in-flight requests | Warning card counts pending approval_requests before saving the policy |
| F11 | Inactive key referenced by a setting | UI badge "in use by N functions" on each key; disabling → confirm dialog stating the consequence |
| — | Secrets not leaked | Do not log api_key; input type=password; masked from the backend |
| — | Audit is immutable | The page is strictly read-only — no button other than export |
