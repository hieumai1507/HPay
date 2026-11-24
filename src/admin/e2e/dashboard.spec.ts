import { test, expect } from './fixtures/auth';

/** T1 (P1) — Dashboard overview, read-only, uses seeded data directly. */
test.describe('Dashboard', () => {
  test('KPI cards and volume chart render', async ({ asMaker }) => {
    await asMaker.goto('/dashboard');
    await expect(asMaker.getByText('New users (today)')).toBeVisible();
    await expect(asMaker.getByText('Hourly volume')).toBeVisible();
    await expect(asMaker.getByText('Needs attention')).toBeVisible();
  });

  test('clicking the KYC attention card navigates to /kyc', async ({ asChecker }) => {
    // /kyc needs kyc.review (superadmin), see ADR-0015.
    await asChecker.goto('/dashboard');
    await asChecker.getByText('KYC pending').click();
    await expect(asChecker).toHaveURL(/\/kyc$/);
  });
});
