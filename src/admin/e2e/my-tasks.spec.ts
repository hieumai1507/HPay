import { expect, test, createFreshReversal, releaseReversal, loginViaApi, seedSession, apiAs, CHECKER } from './fixtures/auth';

/**
 * T7 (P0) — My Tasks: the core maker-checker flow.
 * Each test creates its own reversal-approval request via a direct API call
 * (makerApi, POST /v1/admin/reversals) rather than reusing the single seeded
 * "0900000004 reversal pending" row, per the plan's fixture-isolation
 * strategy (§4b). See fixtures/auth.ts createFreshReversal for why this
 * still runs serially and cleans up after itself — seed data only exposes
 * one discoverable completed transaction to reverse.
 */
test.describe.configure({ mode: 'serial' });

test.describe('My Tasks (maker-checker)', () => {
  test('a maker-created reversal request appears in the checker inbox', async ({ makerApi, checkerApi, browser }) => {
    const { id } = await createFreshReversal(makerApi, 'visibility-check');

    const session = await loginViaApi(CHECKER.email, CHECKER.password);
    const context = await browser.newContext();
    await seedSession(context, session);
    const page = await context.newPage();
    await page.goto('/tasks');
    await expect(page.getByText('reversal').first()).toBeVisible();
    await context.close();

    await releaseReversal(checkerApi, id);
  });

  test('checker decide dialog enforces a 10-char reason before approve/reject', async ({ makerApi, checkerApi, browser }) => {
    const { id } = await createFreshReversal(makerApi, 'reason-validation');

    const session = await loginViaApi(CHECKER.email, CHECKER.password);
    const context = await browser.newContext();
    await seedSession(context, session);
    const page = await context.newPage();
    await page.goto('/tasks');
    await page.getByRole('button', { name: 'Xử lý' }).first().click();

    const approveBtn = page.getByRole('button', { name: 'Phê duyệt' });
    await approveBtn.click(); // empty reason -> toast, dialog stays open
    await expect(page.getByText(/ít nhất 10 ký tự/)).toBeVisible();
    await context.close();

    await releaseReversal(checkerApi, id);
  });

  test('checker approves a task: row updates in place (no full table reload)', async ({ makerApi, browser }) => {
    const { refCode } = await createFreshReversal(makerApi, 'approve-flow');

    const session = await loginViaApi(CHECKER.email, CHECKER.password);
    const context = await browser.newContext();
    await seedSession(context, session);
    const page = await context.newPage();
    await page.goto('/tasks');

    const row = page.locator('table tbody tr', { hasText: refCode });
    await expect(row).toBeVisible();
    await row.getByRole('button', { name: 'Xử lý' }).click();

    await page.locator('textarea').fill('Đã xác minh với người dùng qua điện thoại, đồng ý đảo giao dịch.');
    const [decideResp] = await Promise.all([
      page.waitForResponse((r) => r.url().includes('/decide') && r.request().method() === 'POST'),
      page.getByRole('button', { name: 'Phê duyệt' }).click(),
    ]);
    expect(decideResp.status()).toBe(200);
    await expect(page.getByText('Đã phê duyệt lệnh.')).toBeVisible();
    await expect(row).toBeHidden();
    await context.close();
    // Finalized (approved) — the underlying transaction is now 'reversed' and
    // no longer eligible for a new reversal, so nothing to release here.
  });

  test('A01 — two checker contexts race to decide the same reversal: exactly one wins', async ({ makerApi, browser }) => {
    const { id } = await createFreshReversal(makerApi, 'race-check');

    const session = await loginViaApi(CHECKER.email, CHECKER.password);
    const ctx1 = await browser.newContext();
    const ctx2 = await browser.newContext();
    await seedSession(ctx1, session);
    await seedSession(ctx2, session);
    const { ctx: apiCtx1 } = await apiAs(CHECKER.email, CHECKER.password);
    const { ctx: apiCtx2 } = await apiAs(CHECKER.email, CHECKER.password);

    const [r1, r2] = await Promise.all([
      apiCtx1.post(`/v1/admin/approvals/${id}/decide`, {
        data: { action: 'approve', reason: 'Checker context #1 — automated E2E race (A01)' },
      }),
      apiCtx2.post(`/v1/admin/approvals/${id}/decide`, {
        data: { action: 'approve', reason: 'Checker context #2 — automated E2E race (A01)' },
      }),
    ]);

    const statuses = [r1.status(), r2.status()].sort();
    expect(statuses).toEqual([200, 409]);

    await apiCtx1.dispose();
    await apiCtx2.dispose();
    await ctx1.close();
    await ctx2.close();
  });
});
