import { test, expect, loginViaApi, seedSession, CHECKER } from './fixtures/auth';

/**
 * T3 (P0) — Fraud monitor + resolve action.
 *
 * Fixed bug (found while first writing this spec, verified via a real
 * Playwright browser trace): GET /v1/admin/fraud-flags serialized the
 * fraud_flags.reasons jsonb column through a sqlc-generated `Reasons []byte`
 * field straight into JSON, which Go's encoding/json auto-base64-encodes —
 * every real flag's `reasons` came back as an opaque base64 string, and
 * hpay-admin's FraudPage.tsx crashed calling `.map` on it. Fixed in
 * hpay-backend (json.RawMessage) and hpay-admin (parses the real
 * {signals, advisory} shape). cmd/seed/main.go's fraudFlag rows were also
 * updated to that same real shape (they previously wrote a plain string
 * array, which never matched what internal/fraud/handler.go actually
 * produces).
 *
 * Isolation note: cmd/seed produces exactly ONE 'open' flag (VELOCITY_5MIN
 * on u1's traffic) — there's no public API to mint a fresh fraud flag for
 * beforeEach isolation (flags are only created by the real rule engine
 * reacting to a real transaction). Runs serially, consuming that one open
 * flag only in the last case that needs a decision (A01), same convention
 * as kyc.spec.ts.
 */
test.describe.configure({ mode: 'serial' });

test.describe('Fraud monitor', () => {
  test('open-flags queue lists the seeded VELOCITY_5MIN flag', async ({ asChecker }) => {
    await asChecker.goto('/fraud');
    // Scoped to the table — 'VELOCITY_5MIN'/'AMOUNT_SPIKE' also appear as
    // <option> text in the (hidden-until-opened) rule filter <select>.
    const table = asChecker.getByRole('table');
    await expect(table.getByText('VELOCITY_5MIN').first()).toBeVisible();
    await expect(table.getByText('87', { exact: true })).toBeVisible();
  });

  test('detail panel shows the real per-signal evidence and AI advisory, not a base64 blob', async ({ asChecker }) => {
    await asChecker.goto('/fraud');
    // The one open flag is auto-selected as activeDetailFlag by default
    // (FraudPage.tsx: selectedFlag || filteredFlags.find(f => f.status === 'open')).
    await expect(asChecker.getByText('5 giao dịch trong 5 phút')).toBeVisible();
    await expect(asChecker.getByText(/AI nhận định:/)).toBeVisible();
    await expect(asChecker.getByText(/tần suất bất thường/)).toBeVisible();
  });

  test('rule filter narrows the table (status=all, rule=AMOUNT_SPIKE isolates the cleared flag)', async ({ asChecker }) => {
    await asChecker.goto('/fraud');
    await asChecker.getByRole('combobox').filter({ hasText: 'Status:' }).selectOption('all');
    await asChecker.getByRole('combobox').filter({ hasText: 'Rule:' }).selectOption('AMOUNT_SPIKE');

    const table = asChecker.getByRole('table');
    await expect(table.getByText('AMOUNT_SPIKE').first()).toBeVisible();
    await expect(table.getByText('VELOCITY_5MIN')).not.toBeVisible();
  });

  test('confirm-fraud dialog enforces a 10-char minimum reason before it can be confirmed', async ({ asChecker }) => {
    await asChecker.goto('/fraud');
    await asChecker.getByRole('button', { name: /Xác nhận gian lận/ }).click();

    const dialog = asChecker.getByRole('dialog');
    await expect(dialog).toBeVisible();
    const confirm = dialog.getByRole('button', { name: /Xác nhận xử lý gian lận|Đánh dấu an toàn/ });
    await expect(confirm).toBeDisabled();

    await dialog.locator('textarea').fill('ngắn quá');
    await expect(confirm).toBeDisabled();

    await dialog.locator('textarea').fill('Mẫu hình smurfing rõ, nhiều giao dịch liên tiếp tới người nhận mới.');
    await expect(confirm).toBeEnabled();

    // Close without submitting — the flag must stay open for A01 below.
    await dialog.getByRole('button', { name: 'Hủy' }).click();
    await expect(dialog).toBeHidden();
  });

  test('A01 — two checker contexts race to resolve the same flag: exactly one wins', async ({ browser }) => {
    const session = await loginViaApi(CHECKER.email, CHECKER.password);

    const ctx1 = await browser.newContext();
    const ctx2 = await browser.newContext();
    await seedSession(ctx1, session);
    await seedSession(ctx2, session);
    const page1 = await ctx1.newPage();
    const page2 = await ctx2.newPage();

    await page1.goto('/fraud');
    await page2.goto('/fraud');

    await page1.getByRole('button', { name: /Xác nhận gian lận/ }).click();
    await page2.getByRole('button', { name: /Xác nhận gian lận/ }).click();

    const reason = 'Đối chiếu 2 checker cùng xử lý một flag trong bài kiểm thử A01.';
    await page1.getByRole('dialog').locator('textarea').fill(reason);
    await page2.getByRole('dialog').locator('textarea').fill(reason);

    const [resp1, resp2] = await Promise.all([
      page1.waitForResponse((r) => r.url().includes('/resolve') && r.request().method() === 'POST'),
      page2.waitForResponse((r) => r.url().includes('/resolve') && r.request().method() === 'POST'),
      page1.getByRole('dialog').getByRole('button', { name: /Xác nhận xử lý gian lận/ }).click(),
      page2.getByRole('dialog').getByRole('button', { name: /Xác nhận xử lý gian lận/ }).click(),
    ]);

    const statuses = [resp1.status(), resp2.status()].sort();
    // One request wins (200), the other finds the flag already resolved (409).
    expect(statuses).toEqual([200, 409]);

    await ctx1.close();
    await ctx2.close();
  });
});
