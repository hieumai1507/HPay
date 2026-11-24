import { test, expect, CHECKER } from './fixtures/auth';

/** T9/T10 (P1/P2) — cross-cutting edge cases A03 and A07. */
test.describe('Cross-cutting edge cases', () => {
  test('A03 — re-login after a mid-session 401 returns to the original URL + filters', async ({ asChecker }) => {
    // Fixed bug (found while first writing this test, confirmed step-by-step
    // against a real login flow): src/features/auth/RequireAdmin.tsx
    // correctly redirects to /login with state:{from: pathname+search} on a
    // 401, and LoginPage.tsx's onSubmit calls `navigate(from,
    // {replace:true})` after a successful login — but LoginPage.tsx's
    // top-level guard `if (token) return <Navigate to="/dashboard"
    // replace/>` (meant for "already logged in, don't show the form") ALSO
    // fired on the re-render triggered by that same successful setSession(),
    // ignoring `from` entirely. The two navigations raced and the hardcoded
    // "/dashboard" one won in practice. Fix: the guard now targets `from`
    // too, so both navigations agree regardless of which wins the race.
    // useAdminStore only reads sessionStorage once at module init
    // (src/stores/admin.ts loadInitial()) — mutating sessionStorage in place
    // post-hoc does NOT change the in-memory token the app actually sends,
    // so that can't be used to simulate a mid-session token going bad.
    // Instead, intercept every KYC request and swap in a bad bearer token on
    // the wire from the very first load — this reproduces a real "token
    // rejected server-side" 401 (e.g. revoked/rotated secret) deterministically
    // (no race with an earlier, still-valid request) and exercises the exact
    // same clearSession() path api/client.ts uses for any genuine 401.
    // The KYC queue needs kyc.review (superadmin, ADR-0015), so this test uses the checker.
    await asChecker.route('**/v1/admin/kyc**', async (route) => {
      const headers = { ...route.request().headers(), authorization: 'Bearer corrupted.invalid.token' };
      await route.continue({ headers });
    });

    await asChecker.goto('/kyc?tab=manual&q=Ba');

    // api/client.ts clearSession()s on 401 -> RequireAdmin redirects to
    // /login, preserving pathname+search in location.state.from.
    await expect(asChecker).toHaveURL(/\/login$/);

    // Stop corrupting requests (the token is gone now anyway; re-login and
    // the resulting real navigation back to /kyc must succeed normally).
    await asChecker.unroute('**/v1/admin/kyc**');

    // Re-login through the real form.
    await asChecker.getByLabel('Email').fill(CHECKER.email);
    await asChecker.getByLabel('Password').fill(CHECKER.password);
    await asChecker.getByRole('button', { name: 'Log in' }).click();

    await expect(asChecker).toHaveURL(/\/kyc\?tab=manual&q=Ba$/);
  });

  test.skip(
    'A07 — PII masked in workflow step JSON responses',
    () => {
      // Not implemented in hpay-backend: grep for mask/"***" logic across
      // internal/api and internal/workflow found nothing, and
      // cmd/seed/main.go's `instance()` helper writes literal '{}' for every
      // step_executions.input/output — there is no PII-bearing step context
      // in seed data to assert against, and no masking code path to exercise.
      // Skipping honestly rather than asserting a no-op ("*** not present in
      // an empty object" would pass without testing anything real).
    },
  );
});
