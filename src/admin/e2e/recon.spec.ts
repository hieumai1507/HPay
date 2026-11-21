import { test, expect } from './fixtures/auth';

function yesterday(): string {
  const d = new Date();
  d.setDate(d.getDate() - 1);
  return d.toISOString().slice(0, 10);
}

/**
 * T5 (P1) — Reconciliation (run + view + export).
 * The page's default date input is a hardcoded demo value that does not
 * match the seed's data (cmd/seed/main.go seeds "yesterday" relative to when
 * the seed ran), so every case sets the date input to the real seeded date.
 */
test.describe('Reconciliation', () => {
  test('viewing the seeded recon date shows KPI cards and the AI summary', async ({ asMaker }) => {
    await asMaker.goto('/recon');
    await asMaker.locator('input[type="date"]').fill(yesterday());
    await expect(asMaker.getByText('Tổng đối chiếu')).toBeVisible();
    await expect(asMaker.getByText('AI TÓM TẮT NGUYÊN NHÂN')).toBeVisible();
  });

  test('running recon for an already-processed date does not crash the page', async ({ asMaker }) => {
    asMaker.once('dialog', (d) => d.accept());
    await asMaker.goto('/recon');
    await asMaker.locator('input[type="date"]').fill(yesterday());
    await asMaker.getByRole('button', { name: /Chạy đối soát/ }).click();
    // The mutation swallows both success and ERR_RECON_ALREADY_RAN into an
    // alert() (useReconQueries.ts useRunRecon) — assert the page survives.
    await expect(asMaker.getByText('Tổng đối chiếu')).toBeVisible();
  });

  test('A06 — export stays non-blocking: the date picker remains interactive right after clicking export', async ({ asMaker }) => {
    asMaker.once('dialog', (d) => d.accept());
    await asMaker.goto('/recon');
    await asMaker.locator('input[type="date"]').fill(yesterday());
    await asMaker.getByRole('button', { name: /Xuất Excel/ }).click();
    // No full-page blocking overlay exists in this page at all — the date
    // input (and everything else) must stay clickable immediately after.
    const dateInput = asMaker.locator('input[type="date"]');
    await expect(dateInput).toBeEditable();
    await expect(dateInput).toBeEnabled();
  });
});
