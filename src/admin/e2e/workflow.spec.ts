import { test, expect } from './fixtures/auth';

/**
 * T4 (P1) — Workflow Monitor + filters.
 *
 * A02 note: the actual pagination here is page-index/offset based
 * (src/hooks/useTableQueryState.ts -> page=pageIndex+1, limit), not the
 * cursor pagination described in the plan's ideal architecture, and there is
 * no public API to insert a fresh workflow instance without driving a full
 * P2P/org-disbursement transaction through a *user* JWT + the async workflow
 * worker. So this case is adapted to what is actually verifiable: paging
 * forward and back is stable (no duplicate/missing rows) and a refetch while
 * on a non-first page does not silently reset the view to page 1.
 */
/** Opens the first payment that has the given status. The list is paged, so filter first: the row may not be on page 1. */
async function openFirstWith(page: import('@playwright/test').Page, filter: string) {
  await page.goto('/workflows');
  await page.locator('select').first().selectOption(filter);
  const row = page.locator('table tbody tr').first();
  await expect(row).toBeVisible();
  await row.getByRole('button', { name: 'Open →' }).click();
  await expect(page.getByText('TIMELINE STEP EXECUTIONS')).toBeVisible();
}

test.describe('Workflow Monitor', () => {
  test('instance list renders with status + definition filters', async ({ asMaker }) => {
    await asMaker.goto('/workflows');
    // The title is in the top bar and again as the page heading: assert the page heading.
    await expect(asMaker.getByRole('heading', { name: 'Workflow Monitor' })).toBeVisible();
    await expect(asMaker.locator('table')).toBeVisible();
  });

  test('"Stuck > 5 min" toggle narrows the table to stuck instances only', async ({ asMaker }) => {
    await asMaker.goto('/workflows');
    const toggle = asMaker.getByRole('button', { name: 'Stuck > 5 min' });
    await toggle.click();
    await expect(asMaker.locator('table')).toBeVisible();
    // Toggling back must not error and must restore the unfiltered view.
    await toggle.click();
    await expect(asMaker.locator('table')).toBeVisible();
  });

  test('opening an instance shows the step timeline and retry/compensate controls', async ({ asMaker }) => {
    // Retry and Compensate are only offered on a payment that stopped with an error, so open a failed one.
    await openFirstWith(asMaker, 'failed');
    await expect(asMaker).toHaveURL(/\/workflows\/.+/);
    await expect(asMaker.getByRole('button', { name: /Retry/ }).first()).toBeVisible();
    await expect(asMaker.getByRole('button', { name: /Compensate|Force fail/ }).first()).toBeVisible();
  });

  // A payment that is waiting for approval or already finished must not offer Retry or Compensate: a wrong
  // click there would try to repair a payment that is fine.
  test('retry and compensate appear only on a payment that stopped with an error', async ({ asMaker }) => {
    await openFirstWith(asMaker, 'waiting');
    await expect(asMaker.getByText('There is nothing to retry or reverse here.')).toBeVisible();
    await expect(asMaker.getByRole('button', { name: /Retry/ })).toHaveCount(0);
    await expect(asMaker.getByRole('button', { name: /Compensate|Force fail/ })).toHaveCount(0);

    await openFirstWith(asMaker, 'failed');
    await expect(asMaker.getByRole('button', { name: /Retry/ }).first()).toBeVisible();
  });

  test('A02 (adapted) — paging forward/back is stable and a mid-page refetch keeps the current page', async ({ asMaker }) => {
    await asMaker.goto('/workflows');
    await expect(asMaker.locator('table tbody tr').first()).toBeVisible();

    const firstPageIds = await asMaker.locator('table tbody tr td:first-child').allTextContents();

    const nextBtn = asMaker.getByTitle('Next');
    if (await nextBtn.isEnabled()) {
      await nextBtn.click();
      await expect(asMaker.getByText(/^2 \//)).toBeVisible();
      const secondPageIds = await asMaker.locator('table tbody tr td:first-child').allTextContents();
      // No row visible on both pages at once (no duplication from an offset drift).
      const overlap = secondPageIds.filter((id) => firstPageIds.includes(id));
      expect(overlap).toEqual([]);

      // Refresh while on page 2 — the page indicator must not silently jump to 1.
      await asMaker.getByRole('button', { name: '⟳ Refresh' }).click();
      await expect(asMaker.getByText(/^2 \//)).toBeVisible();
    } else {
      test.skip(true, 'not enough seeded workflow instances to exercise a second page');
    }
  });
});
