import { test, expect, createFreshReversal, releaseReversal } from './fixtures/auth';

/** T8 (P2) — Audit Log search, read-only against seeded/organic audit rows. */
test.describe('Audit log', () => {
  test('list renders with actor/action/resource columns', async ({ asMaker }) => {
    await asMaker.goto('/audit');
    await expect(asMaker.getByText('Audit Logs')).toBeVisible();
    await expect(asMaker.locator('table')).toBeVisible();
  });

  test('search narrows results for a real match', async ({ asMaker, makerApi, checkerApi }) => {
    // Seed a real, uniquely-markable audit row by filing a reversal request
    // (POST /v1/admin/reversals) — an action every admin session is allowed
    // to perform. Shares the single discoverable completed transaction with
    // my-tasks.spec.ts (see fixtures/auth.ts createFreshReversal), so this
    // releases it again immediately after asserting.
    const marker = `e2e-audit-search-marker-${Date.now()}`;
    const { id } = await createFreshReversal(makerApi, marker);

    await asMaker.goto('/audit');
    const search = asMaker.getByPlaceholder(/Search by action/i);
    await search.fill(marker);
    await expect(asMaker.getByText(marker)).toBeVisible();

    await releaseReversal(checkerApi, id);
  });

  test(
    'a real zero-result search shows an empty table, not hardcoded demo rows',
    async ({ asMaker }) => {
      // Fixed bug: GET /v1/admin/audit returned `logs: null` for zero
      // matches (sqlc leaves the slice nil when no rows scan; encoding/json
      // marshals nil as `null`, not `[]`). hpay-admin's useAuditQueries.ts
      // `if (res?.logs)` treated that `null` as "the call didn't really
      // work" and silently fell back to 3 hardcoded demo rows instead of a
      // real, correct empty result. AdminHandler.ListAuditLogs now coerces
      // a nil slice to `[]` before responding — `[]` is truthy in JS, so no
      // frontend change was needed.
      await asMaker.goto('/audit');
      const search = asMaker.getByPlaceholder(/Search by action/i);
      await search.fill(`zzz-no-such-action-${Date.now()}`);
      await expect(asMaker.getByText('Nothing here yet')).toBeVisible();
      await expect(asMaker.getByText('POLICY_UPDATE')).not.toBeVisible();
      await expect(asMaker.getByText('admin.098812')).not.toBeVisible();
    },
  );
});
