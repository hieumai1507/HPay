---
title: ADR 0001 Monorepo with a docs knowledge base
type: decision
status: verified-against-code
last_verified: 2026-11-21
---

# ADR 0001: Monorepo with a docs knowledge base

Status: Accepted (2026-11-21)

## Context
HPay was spread over several repositories (backend, admin, mobile, docs, reference docs). Contracts drifted between them (audit section 2: OpenAPI vs frontend mismatches, docs contradicting code) and agents had no single place for context. The old directory was not under version control, so `.gitignore` protected nothing (audit section 6).

## Decision
- One repository at `D:\hpay`. Code lives under `src/` (`src/backend`, `src/admin`, `src/mobile`).
- The knowledge base lives under `docs/` (English Markdown, front matter with status and last_verified). Material from other systems stays outside the repo (`docs/reference/external/` is git-ignored).
- Shared agent assets live at the repository root: `.claude/skills`, `.claude/agents`, `CLAUDE.md`, `AGENTS.md`. Each app may add its own `CLAUDE.md`.
- CI is path-filtered per app in `.github/workflows/` (the admin e2e job builds the backend from the same checkout, no separate PAT).
- Git history was reset on 2026-11-21: the repository starts with `chore: init hpay monorepo`. Earlier history is not carried over.

## Consequences
- One checkout builds and tests the whole product; API and frontend changes can land in one commit.
- Old history and blame are gone; historical context is kept in `docs/history/` instead.
- Secrets must be kept out of the tree from the first commit (`.gitignore` covers `.env*`, `data/`, `e2e/.auth/`).
- Go module path remains `github.com/hieumai1507/hpay-backend`, so imports did not change.