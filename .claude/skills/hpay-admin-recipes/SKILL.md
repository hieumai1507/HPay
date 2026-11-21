---
name: hpay-admin-recipes
description: Recipes for src/admin (React + Vite + TanStack) - scaffold a page (queue/table + detail) and create a TanStack Query hook wired to the typed api client. Use inside hpay-feature stage 4-5 when a task adds an admin page or data hook.
---

# hpay-admin-recipes

| Need | Read |
|------|------|
| New admin page (queue/table, optional detail) | [references/page.md](references/page.md) |
| Query/mutation hook for an endpoint | [references/hook.md](references/hook.md) |

Rules that override the recipes when they conflict: mockups live in `docs/design/mockups/`; no mock fallbacks in production builds (only under `import.meta.env.DEV`); amounts are integer minor units, never `Number()` on money; every write sends an `Idempotency-Key` and disables its button while pending; routes with privileged actions need a permission guard. Check with `cd src/admin && npx tsc --noEmit && npm run lint`.