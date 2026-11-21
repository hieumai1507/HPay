---
name: hpay-mobile-recipes
description: Recipes for src/mobile (Expo / React Native, expo-router) - scaffold a screen and create a TanStack Query hook wired to the generated API client. Use inside hpay-feature stage 4-5 when a task adds a mobile screen or data hook.
---

# hpay-mobile-recipes

| Need | Read |
|------|------|
| New expo-router screen | [references/screen.md](references/screen.md) |
| Query/mutation hook for an endpoint | [references/hook.md](references/hook.md) |

Rules that override the recipes when they conflict: money uses BigInt helpers in `src/lib/money.ts` (no `Number()`/`parseInt()` on amounts); every money-moving mutation uses a per-attempt idempotency key kept stable across retries; never hard-code a PIN or secret; list screens need Skeleton/Empty/Error states. Check with `cd src/mobile && npx tsc --noEmit && npm run lint && npm test`.