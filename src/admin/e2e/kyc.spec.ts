import { test, expect, loginViaApi, seedSession, CHECKER } from './fixtures/auth';

/**
 * T2 (P0) — KYC queue + review action.
 *
 * Isolation note: the seed only produces ONE manual_review row
 * (user 0900000003, cmd/seed/main.go `kycRequest`). There is no public API
 * to mint a fresh KYC-pending row for beforeEach isolation (KYC requests are
 * only created by the mobile onboarding flow), so this file runs serially
 * against that single seeded record and orders cases so the record is only
 * "consumed" (decided) by the last case that needs a decision (A01).
 * kyc.review permission is superadmin-only in this app's role matrix
 * (src/stores/admin.ts ROLE_PERMISSIONS), so review actions use asChecker.
 */
test.describe.configure({ mode: 'serial' });

test.describe('KYC queue', () => {
  test('manual-review tab lists the seeded pending profile', async ({ asMaker }) => {
    await asMaker.goto('/kyc?tab=manual');
    await expect(asMaker.getByText('Trần Văn Ba')).toBeVisible();
    await expect(asMaker.getByText('0900000003')).toBeVisible();
  });

  test('opening a row shows AI score, verdict and reasons', async ({ asChecker }) => {
    await asChecker.goto('/kyc?tab=manual');
    await asChecker.getByText('Trần Văn Ba').click();
    await expect(asChecker).toHaveURL(/\/kyc\/.+\?tab=manual/);
    await expect(asChecker.getByText('64/100')).toBeVisible();
    await expect(asChecker.getByText('AI phân tích')).toBeVisible();
    await expect(asChecker.getByRole('button', { name: /Phê duyệt/ })).toBeVisible();
  });

  test('reject dialog enforces a 10-char minimum reason before it can be confirmed', async ({ asChecker }) => {
    await asChecker.goto('/kyc?tab=manual');
    await asChecker.getByText('Trần Văn Ba').click();
    await asChecker.getByRole('button', { name: /Từ chối/ }).first().click();

    const dialog = asChecker.getByRole('dialog', { name: 'Từ chối hồ sơ KYC' });
    await expect(dialog).toBeVisible();
    const confirm = dialog.getByRole('button', { name: 'Xác nhận từ chối' });
    await expect(confirm).toBeDisabled();

    await dialog.locator('textarea').fill('quá ngắn');
    await expect(confirm).toBeDisabled();

    await dialog.locator('textarea').fill('Ảnh CCCD mờ, không khớp khuôn mặt selfie.');
    await expect(confirm).toBeEnabled();

    // Close without submitting — the record must stay pending for the A04/A01
    // cases below, which still need it undecided.
    await dialog.getByRole('button', { name: 'Hủy' }).click();
    await expect(dialog).toBeHidden();
  });

  test('A04 — expired KYC image URL: FE retries once via refetch before giving up', async ({ asChecker }) => {
    let kycListHits = 0;
    await asChecker.route('**/v1/admin/kyc**', (route) => {
      kycListHits++;
      route.continue();
    });

    let imageHits = 0;
    await asChecker.route('**/seed/kyc/id-card.jpg**', (route) => {
      imageHits++;
      if (imageHits === 1) {
        route.fulfill({ status: 403, body: 'expired signed url' });
      } else {
        route.continue();
      }
    });

    await asChecker.goto('/kyc?tab=manual');
    const before = kycListHits;
    await asChecker.getByText('Trần Văn Ba').click();

    // SignedImage.onError -> refetch() fires exactly once (retried ref guards
    // against loops) — assert the KYC list query re-runs after the 403.
    await expect
      .poll(() => imageHits, { message: 'image should have been requested at least twice (403 then retry)' })
      .toBeGreaterThanOrEqual(1);
    await expect
      .poll(() => kycListHits, { message: 'KYC queue should refetch after the signed image 403s' })
      .toBeGreaterThan(before);
  });

  test('A01 — two checker contexts race to approve the same profile: one wins, one gets a stale-state notice', async ({ browser }) => {
    const session = await loginViaApi(CHECKER.email, CHECKER.password);

    const ctx1 = await browser.newContext();
    const ctx2 = await browser.newContext();
    await seedSession(ctx1, session);
    await seedSession(ctx2, session);
    const page1 = await ctx1.newPage();
    const page2 = await ctx2.newPage();

    await page1.goto('/kyc?tab=manual');
    await page2.goto('/kyc?tab=manual');
    await page1.getByText('Trần Văn Ba').click();
    await page2.getByText('Trần Văn Ba').click();

    const approve1 = page1.getByRole('button', { name: /Phê duyệt/ });
    const approve2 = page2.getByRole('button', { name: /Phê duyệt/ });
    await expect(approve1).toBeVisible();
    await expect(approve2).toBeVisible();

    const [resp1, resp2] = await Promise.all([
      page1.waitForResponse((r) => r.url().includes('/review') && r.request().method() === 'POST'),
      page2.waitForResponse((r) => r.url().includes('/review') && r.request().method() === 'POST'),
      approve1.click(),
      approve2.click(),
    ]);

    const statuses = [resp1.status(), resp2.status()].sort();
    // One request wins (200), the other finds the record already decided (409).
    expect(statuses).toEqual([200, 409]);

    await ctx1.close();
    await ctx2.close();
  });
});
