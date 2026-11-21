[← Cloudinary Storage](../cloudinary-storage/implementation-plan.md) · f11-dynamic-ai-config · [solution-architecture →](solution-architecture.md)

# F11 Dynamic AI Config — Implementation Plan

## 1. Scope & non-goals

> **Key point:** Move the AI configuration (LLM Provider, Models, API Keys) from static `.env` files to dynamic management in the Database. This allows replacing/changing Keys via the Admin API, emergency activation/deactivation of Keys when errors occur, and mapping different AI features (KYC scoring, Fraud advising) to different models/providers (Anthropic Claude, OpenAI GPT) to optimize cost/performance. The **admin UI** for this feature belongs to phase A4 on the admin side — see `src/admin/docs/plans/a4-recon-orgs-audit/` (task T6/T6b + sections 3–4 of the solution-architecture).

| Ships | Non-goals (out of scope here) |
| --- | --- |
| - **New database tables**: `ai_keys` (key pool management, active/inactive status) and `ai_function_settings` (mapping of function -> model + key).<br> - **Multi-provider support**: Integrate plain API clients for both **Anthropic Claude** (Messages API) and **OpenAI GPT** (Chat Completions API) in the KYC Scorer and Fraud Advisor.<br> - **Fallback mechanism**: Automatically fall back to Mock or return an appropriate error when the provider or API key of that function is disabled (inactive).<br> - **Admin API system**: Provide secured Admin APIs to manage Keys (Add, Update status, Update) and feature settings (Configure Model & Provider per function). | - A complete Admin SPA UI (focus only on providing the end-to-end API on the Backend side).<br> - Integrating providers beyond Anthropic and OpenAI (for example Gemini, DeepSeek) in this phase. |

## 2. Task breakdown

| # | Task | Files | Size | Depends |
| --- | --- | --- | --- | --- |
| T1 | Migration 000015: Create the `ai_keys` and `ai_function_settings` tables. Seed the default configuration as the `mock` provider for the `kyc_scoring` and `fraud_advising` functions. | db/migrations/000015_* | S | — |
| T2 | Queries: Write SQL queries for Key management (list, insert, update status) and Settings (Get the active configuration of one function together with the corresponding API Key value, Update settings). | internal/store/queries/ai.sql | M | T1 |
| T3 | Restructure Scorer & Advisor: Change the kyc Scorer and fraud Advisor constructor parameters from static to dynamic loading by passing in the DB Pool. Write a client for OpenAI GPT to call when the configured provider is `openai`. | internal/kyc/scorer.go, internal/fraud/advisor.go | L | T2 |
| T4 | Admin API Handlers: Build the admin routes in AdminHandler to manage keys and settings, protected by the `RequireAdmin` middleware. Record change actions in `audit_logs`. | internal/api/admin_ai.go [NEW] | M | T3 |
| T5 | Testing: Write integration tests that run the KYC flow against an OpenAI mock HTTP server and an Anthropic mock server; tests verify that disabling a key (inactive) immediately stops or falls back the corresponding feature. | test/integration/ai_config_test.go [NEW] | L | T4 |

## 3. Schema changes

Add two new tables in Migration `000015_create_ai_configs.up.sql`:

```sql
CREATE TABLE ai_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    provider VARCHAR(50) NOT NULL, -- 'anthropic' | 'openai'
    api_key VARCHAR(255) NOT NULL, -- Encrypted/encrypted-at-rest in the future
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE ai_function_settings (
    function_name VARCHAR(100) PRIMARY KEY, -- 'kyc_scoring' | 'fraud_advising'
    provider VARCHAR(50) NOT NULL,          -- 'anthropic' | 'openai' | 'mock'
    model_name VARCHAR(100) NOT NULL,       -- e.g. 'claude-3-5-sonnet', 'gpt-4o'
    key_id UUID REFERENCES ai_keys(id) ON DELETE SET NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

## 4. API changes (into openapi.yaml)

| Endpoint | Description | Errors |
| --- | --- | --- |
| `GET /v1/admin/ai/keys` | Get the list of API keys currently in the system (hide the api_key field or show it masked only). | 401 Unauthorized |
| `POST /v1/admin/ai/keys` | Add a new API Key {name, provider, api_key}. | ERR_VALIDATION |
| `PUT /v1/admin/ai/keys/{id}` | Update a Key or change its active status {name, api_key?, is_active}. | ERR_NOT_FOUND |
| `GET /v1/admin/ai/settings` | Get the current configuration of all AI functions. | — |
| `PUT /v1/admin/ai/settings` | Set the model/provider configuration for a function {function_name, provider, model_name, key_id?}. | ERR_VALIDATION, ERR_KEY_INACTIVE (if key_id points to a disabled key) |

## 5. Test plan

| Group | Test description |
| --- | --- |
| Inactive Key test | Set `is_active = false` for a key -> Call the KYC function -> Returns the error `ERR_AI_PROVIDER_UNAVAILABLE` (because the key is disabled), without crashing the system. |
| Model switch test | Update the `kyc_scoring` setting from `anthropic` to `openai` -> Call KYC submit -> The HTTP request is sent in the correct OpenAI API format. |

## 6. Rollout & risk

- **Sudden deactivation:** If an admin disables all active keys of `kyc_scoring`, the system falls into a state where automatic review is no longer supported (fallback to manual review or return a review error). A warning needs to be displayed on the admin UI.

- **API Key security:** The API Key must be displayed masked, `sk-...xxxx`, when queried to avoid data leakage.

## 7. Execution checklist

- [ ] gen-feature-plan · [ ] T1 (migration) → T2 (sqlc) → T3 (refactor services) → T4 (api handlers) → T5 (integration tests) · [ ] fintech-review · [ ] commit `feat: dynamic database-driven AI model configurations and administration`
