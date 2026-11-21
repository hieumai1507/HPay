<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **hpay** (3098 symbols, 7737 relationships, 113 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> Index stale? Run `node .gitnexus/run.cjs analyze` from the project root — it auto-selects an available runner. No `.gitnexus/run.cjs` yet? `npx gitnexus analyze` (npm 11 crash → `npm i -g gitnexus`; #1939).

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows. For regression review, compare against the default branch: `detect_changes({scope: "compare", base_ref: "main"})`.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `query({search_query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `context({name: "symbolName"})`.
- For security review, `explain({target: "fileOrSymbol"})` lists taint findings (source→sink flows; needs `analyze --pdg`).

## Never Do

- NEVER edit a function, class, or method without first running `impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit changes without running `detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/hpay/context` | Codebase overview, check index freshness |
| `gitnexus://repo/hpay/clusters` | All functional areas |
| `gitnexus://repo/hpay/processes` | All execution flows |
| `gitnexus://repo/hpay/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |
| Work in the Workflow area (102 symbols) | `.claude/skills/generated/workflow/SKILL.md` |
| Work in the Api area (88 symbols) | `.claude/skills/generated/api/SKILL.md` |
| Work in the Gen area (73 symbols) | `.claude/skills/generated/gen/SKILL.md` |
| Work in the Integration area (48 symbols) | `.claude/skills/generated/integration/SKILL.md` |
| Work in the Auth area (33 symbols) | `.claude/skills/generated/auth/SKILL.md` |
| Work in the Approval area (26 symbols) | `.claude/skills/generated/approval/SKILL.md` |
| Work in the Org area (17 symbols) | `.claude/skills/generated/org/SKILL.md` |
| Work in the Moneyrequest area (16 symbols) | `.claude/skills/generated/moneyrequest/SKILL.md` |
| Work in the Fraud area (12 symbols) | `.claude/skills/generated/fraud/SKILL.md` |
| Work in the Kyc area (12 symbols) | `.claude/skills/generated/kyc/SKILL.md` |
| Work in the Recon area (9 symbols) | `.claude/skills/generated/recon/SKILL.md` |
| Work in the Ledger area (7 symbols) | `.claude/skills/generated/ledger/SKILL.md` |
| Work in the Stores area (7 symbols) | `.claude/skills/generated/stores/SKILL.md` |
| Work in the Ui area (7 symbols) | `.claude/skills/generated/ui/SKILL.md` |
| Work in the Outbox area (6 symbols) | `.claude/skills/generated/outbox/SKILL.md` |
| Work in the Storage area (6 symbols) | `.claude/skills/generated/storage/SKILL.md` |
| Work in the Config area (4 symbols) | `.claude/skills/generated/config/SKILL.md` |
| Work in the Fee area (3 symbols) | `.claude/skills/generated/fee/SKILL.md` |
| Work in the Testutil area (3 symbols) | `.claude/skills/generated/testutil/SKILL.md` |
| Work in the Cluster_81 area (3 symbols) | `.claude/skills/generated/cluster-81/SKILL.md` |

<!-- gitnexus:end -->

# HPay workflow

Layout: code in `src/{backend,admin,mobile}`, knowledge-base in `docs/` (start at `docs/README.md`), skills only in `.claude/skills/`, subagents in `.claude/agents/`.

| Situation | Use |
|-----------|-----|
| New feature / change / vague request | skill `hpay-feature` (clarify -> spec -> plan -> red tests -> implement -> verify -> review -> update knowledge base) |
| Bug, failing test, audit finding | skill `hpay-bugfix` (searches `docs/bugs` first; records the bug and fix there when done) |
| Before commit / PR | skill hpay-review |
| QA test-case workbooks | skill hpay-qa-testcases |
| Building blocks (module, migration, step, endpoint test, page, hook, screen) | `hpay-backend-recipes`, `hpay-admin-recipes`, `hpay-mobile-recipes` |

Knowledge base (`docs/`): `domain/` business rules per bounded context, `architecture/`, `reference/` (data model, API, errors, config, glossary), `product/`, `guides/`, `adr/`, `bugs/` (ledger of every bug fixed), `quality/`, `history/` (old plans, not truth). Read the relevant `docs/domain/*.md` before changing a module; update it when you change behavior.`n`nRules: docs-first (change docs before code); code is the fact when docs disagree (see `docs/audit/AUDIT-REPORT.md`); never report a command as passing unless it ran in this session; 3 failed fix attempts means stop and report; never commit secrets, `.env`, `data/`; do not push or publish without the user's explicit go-ahead for that push.