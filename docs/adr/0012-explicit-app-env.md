---
id: ADR-0012
title: APP_ENV is required and validated; dev-only behavior cannot occur outside dev
status: proposed
date: 2026-11-21
supersedes: []
superseded_by: []
spec: specs/w1-environment-guards/spec.md
affects:
  - architecture/deployment.md
  - reference/configuration.md
  - domain/identity-and-auth.md
  - guides/setup-local-environment.md
  - quality/security-model.md
constitution: [C-09, C-11]
---

# ADR-0012: APP_ENV is required and validated; dev-only behavior cannot occur outside dev

Status: **proposed**. Implemented in the working tree under the lead's instruction to complete the plan before review; the user approves it together with the final change set.

## Context
`APP_ENV` defaults to `dev`, so a deployment that forgets the variable silently enables dev behavior (OTP in responses, mock bank route, weak secrets). The documented values (`dev|ci|demo`) do not include the literal `production` that the code actually compares against, so the real outbound gateway cannot be enabled by a documented value (audit BE-H7, `domain/identity-and-auth.md`, `reference/configuration.md`).

## Options considered
| Option | Pros | Cons | Cost / risk |
|--------|------|------|-------------|
| **A. Required, whitelisted `APP_ENV` (`dev`, `ci`, `demo`, `production`); each dangerous behavior keyed on the validated value** | Fails safe; one place defines the environments; testable | Breaking for deployments that relied on the default | Low; all known launchers are in the repo |
| B. Keep the `dev` default but add warnings | No breaking change | Still unsafe by default; warnings are ignored | Not acceptable |
| C. Default to `production` when unset | Safe by default | Surprising for local runs and tests; hides the missing variable | Medium |
| D. Feature flags per dangerous behavior (OTP in responses, mock route, ...) independent of env | Fine-grained | More configuration, more ways to be wrong | Higher |

## Decision
Adopt **A**. Missing or unknown `APP_ENV` is a startup error. `production` is the only environment with the real outbound gateway and without the mock-bank route; `dev` is the only one that returns `otp_dev` and loads `.env`; secrets must be strong and non-default outside `dev` and `ci`. A single helper on the config type answers each question so no package compares raw strings.

## Constitution check
C-09: now enforced. C-11: no default or placeholder secret outside dev and ci. No deviation.

## Consequences
- Easier: a misconfigured production deploy fails at startup instead of running with dev behavior.
- Harder: every launcher must set `APP_ENV` (compose, scripts, CI, docs updated in this change).
- Risk: an unknown hosted deployment that relied on the default would stop starting. Mitigation: the error message names the allowed values.
- Follow-ups: network-level protection of `/internal/*` at the proxy, a `staging` environment if one appears, key rotation (user task).

## Documentation to update
`architecture/deployment.md`, `reference/configuration.md`, `domain/identity-and-auth.md`, `guides/setup-local-environment.md`, `quality/security-model.md`, `constitution.md` (C-09, C-11 status), `adr/README.md`.