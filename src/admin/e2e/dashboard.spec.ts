import { test, expect } from './fixtures/auth';

/** T1 (P1) — Dashboard overview, read-only, uses seeded data directly. */
test.describe('Dashboard', () => {
  test('KPI cards and volume chart render', async ({ asMaker }) => {
    await asMaker.goto('/dashboard');
    await expect(asMaker.getByText('Người dùng mới (hôm nay)')).toBeVisible();
    await expect(asMaker.getByText('Volume theo giờ')).toBeVisible();
    await expect(asMaker.getByText('Cần chú ý')).toBeVisible();
  });

  test('clicking the KYC attention card navigates to /kyc', async ({ asMaker }) => {
    await asMaker.goto('/dashboard');
    await asMaker.getByText('KYC chờ duyệt').click();
    await expect(asMaker).toHaveURL(/\/kyc$/);
  });
});
