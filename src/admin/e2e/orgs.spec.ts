import { test, expect } from './fixtures/auth';

/**
 * T6 (P2) — Organizations & Policy.
 *
 * Fixed bug (found while first writing this spec, verified against a real
 * backend + real admin token): GET /v1/org requires a *user*-audience JWT,
 * so an *admin*-audience token got a real 401, and src/api/client.ts's
 * `api()` wrapper treats ANY 401 as "session expired" and calls
 * clearSession() unconditionally before useOrgsQueries.ts's own
 * try/catch-and-fall-back-to-mock logic ever saw the error — RequireAdmin
 * redirected to /login before the mock-data fallback could even paint. Net
 * effect: any admin who opened the Organizations page (UI label "Tổ chức & Policy") was immediately logged
 * out. Fix: a real admin-ops endpoint, GET /v1/admin/orgs (hpay-backend),
 * which useOrgs() now calls instead — not scoped to caller membership like
 * /v1/org, so it works under the admin audience and returns every org with
 * real member/policy/pending-approval counts.
 */
test.describe('Organizations & Policy', () => {
  test('opening /orgs as an admin shows the org list without logging out', async ({ asMaker }) => {
    await asMaker.goto('/orgs');
    await expect(asMaker).toHaveURL(/\/orgs$/);
    // DEH Corp is seeded by cmd/seed (real org, real member/policy counts) —
    // proves the page rendered real data from GET /v1/admin/orgs, not just
    // that it didn't redirect. exact:true because "DEH Corp — Chính sách phê
    // duyệt" (the detail-panel heading, "DEH Corp - Approval Policy") also contains this text.
    await expect(asMaker.getByText('DEH Corp', { exact: true })).toBeVisible();
  });
});
