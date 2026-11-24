import { test, expect } from '@playwright/test';
import { MAKER } from './fixtures/auth';

/**
 * T0 (P0) — the ONLY spec that goes through the real LoginPage form.
 * Every other spec bootstraps its session via a direct API call
 * (e2e/fixtures/auth.ts) so the login UI is not re-tested dozens of times.
 */
test.describe('Login', () => {
  test('correct credentials redirect to /dashboard', async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Email').fill(MAKER.email);
    await page.getByLabel('Password').fill(MAKER.password);
    await page.getByRole('button', { name: 'Log in' }).click();

    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(page.getByText('E2E Maker').or(page.getByText(MAKER.email))).toBeVisible();
  });

  test('wrong password shows inline error and does not redirect', async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Email').fill(MAKER.email);
    await page.getByLabel('Password').fill('WrongPassword123!');
    await page.getByRole('button', { name: 'Log in' }).click();

    await expect(page.getByText(/password|account|information/i)).toBeVisible();
    await expect(page).toHaveURL(/\/login$/);
  });
});
