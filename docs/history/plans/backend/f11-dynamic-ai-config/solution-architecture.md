[← implementation-plan](implementation-plan.md) · f11-dynamic-ai-config

# F11 Dynamic AI Config — Solution Architecture

## 1. Context — Structure of dynamic AI configuration loading

The Workflow Engine loads configuration from the DB to route AI requests to the selected provider and model.

```mermaid
flowchart TD
  subgraph Admin API
    adm["🖥 Super Admin"] -->|"PUT /admin/ai/settings\nConfigure model+key"| handler["internal/api\nadmin_ai.go"]
  end

  subgraph Workflow Execution
    eng["workflow.Engine"] -->|"Step 1: SCORE"| kyc_h["kyc_scorer.go\nhandler"]
    eng -->|"Step 2: FRAUD_DETECT"| fraud_h["fraud_advisor.go\nhandler"]
  end

  subgraph Configuration Database
    pg[("PostgreSQL\nai_keys\nai_function_settings")]
  end

  subgraph Dynamic AI Clients
    factory["AI Client Factory"]
    claude["🤖 Anthropic Claude Client\n(net/http)"]
    gpt["🤖 OpenAI GPT Client\n(net/http)"]
    mock["⚙️ Mock Fallback Client"]
  end

  handler -->|Write configuration| pg
  kyc_h -->|Query active settings| pg
  fraud_h -->|Query active settings| pg

  kyc_h & fraud_h -->|Load config| factory
  factory -->|Create| claude
  factory -->|Create| gpt
  factory -->|Create| mock
```

## 2. Main flow — Loading configuration & execution in the KYC Scorer step

```mermaid
sequenceDiagram
  autonumber
  participant ENG as Workflow Engine
  participant H as kyc_scorer handler
  participant DB as PostgreSQL (DB Pool)
  participant FC as Client Factory
  participant AI as AI API (Claude / GPT)

  ENG->>H: Execute step kyc_scorer (ctx, request_id)
  H->>DB: Query active setting for "kyc_scoring" (JOIN ai_keys)
  alt No configuration found or provider = 'mock'
    DB-->>H: Return provider='mock', key=empty
    H->>FC: Get MockScorer
    FC-->>H: Return MockScorer
    H->>H: Score automatically based on the image hash
  else Configuration found and key is active
    DB-->>H: Return provider='openai', model='gpt-4o', api_key='sk-...'
    H->>FC: Create Client(provider, model, key)
    FC-->>H: Return OpenAIClient
    H->>AI: POST /v1/chat/completions (with API key + 2 base64 images)
    AI-->>H: 200 OK {"choices": [{"message": {"content": "..."}}]}
  else Configuration found but key is disabled (is_active = false)
    DB-->>H: Return provider='openai', key.is_active=false
    H-->>ENG: Return error ERR_AI_PROVIDER_UNAVAILABLE (Alert, stop the automatic flow)
  end
  H->>ENG: Done, return result {score, reasons}
```

## 3. Data model — New data tables

```mermaid
erDiagram
  ai_keys ||--o{ ai_function_settings : "key_id (1 key can be used for multiple features)"
  ai_keys {
    uuid id PK
    varchar name "Descriptive name of the Key"
    varchar provider "anthropic | openai"
    varchar api_key "API Key value"
    boolean is_active "Active status"
    timestamp created_at
    timestamp updated_at
  }
  ai_function_settings {
    varchar function_name PK "kyc_scoring | fraud_advising"
    varchar provider "anthropic | openai | mock"
    varchar model_name "Name of the model to call"
    uuid key_id FK "Key link"
    timestamp updated_at
  }
```

## 4. Decisions & trade-offs

| Decision | Alternative | Reason & Benefits |
| --- | --- | --- |
| Split into **ai_keys** and **ai_function_settings** tables | Store api_key directly in a column of the settings table. | - **Easy management and reuse:** Lets users declare a single API Key (for example the company's OpenAI Key) and map that key to be shared by both KYC and Fraud. When the key needs changing, only one record in the `ai_keys` table has to be edited. - **Security:** Makes it easy to hide/mask the secret field in management tables. |
| **Query the DB directly** on every workflow step run | Store the configuration in a cache (Redis or in-memory). | - **Immediacy:** When an admin discovers a faulty key and clicks "deactivate", the system immediately blocks subsequent AI calls without waiting for cache expiry or cluster sync.<br> - **Low frequency:** AI tasks such as KYC image submission or Fraud alerts occur at a modest rate (a few per second at medium scale), so querying PostgreSQL directly (with optimized indexes) does not affect system performance. |
