import { test as base, expect, request, type APIRequestContext, type BrowserContext, type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { SESSIONS_FILE } from '../global-setup';

/**
 * Auth strategy (implementation-plan.html §5): most specs bootstrap a session
 * via a REAL HTTP call to POST /v1/admin/auth/login (not through the login
 * form), then inject the resulting {token, admin} into sessionStorage under
 * the exact key useAdminStore reads (src/stores/admin.ts STORAGE_KEY =
 * 'hpay-admin-session', shape: {token, admin}). Only e2e/login.spec.ts goes
 * through the real form.
 *
 * Credentials come from hpay-backend `go run ./cmd/seed --e2e-admins`:
 *   e2e-maker@HPay.dev    role=ops         (maker in My Tasks specs)
 *   e2e-checker@HPay.dev  role=superadmin  (checker in My Tasks specs)
 */

// The seed (src/backend cmd/seed --e2e-admins) creates both admins with this default.
// Override with E2E_ADMIN_PASSWORD when the seed was run with another password.
const E2E_ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? 'E2ETest@123';

export const MAKER = { email: 'e2e-maker@HPay.dev', password: E2E_ADMIN_PASSWORD };
export const CHECKER = { email: 'e2e-checker@HPay.dev', password: E2E_ADMIN_PASSWORD };

export interface AdminProfile {
  id: string;
  email: string;
  full_name: string;
  role: string;
}

export interface Session {
  token: string;
  admin: AdminProfile;
}

const STORAGE_KEY = 'hpay-admin-session';

function apiBaseURL(): string {
  // The backend is reachable directly on :8080 for API bootstrap calls (the
  // vite preview proxy only matters for calls made *from inside the page*).
  return process.env.E2E_API_URL ?? 'http://localhost:8080';
}

let sharedApiContext: APIRequestContext | undefined;

async function apiContext(): Promise<APIRequestContext> {
  if (!sharedApiContext) {
    sharedApiContext = await request.newContext({ baseURL: apiBaseURL() });
  }
  return sharedApiContext;
}

// POST /v1/admin/auth/login is rate-limited to 5 requests/minute per IP
// (hpay-backend internal/api/router.go: RateLimit(d.Redis, 5, time.Minute)).
// A full-stack suite that mints a fresh token per test/worker would blow
// through that in seconds, so e2e/global-setup.ts logs each fixed E2E admin
// in EXACTLY ONCE for the whole `playwright test` run and persists the
// tokens to disk (e2e/.auth/sessions.json) — every worker reads them back
// here instead of hitting /login again. Falls back to a real (cached, one
// per process) login if the cache file is missing, so fixtures still work if
// a spec is run directly against a running dev server without globalSetup
// having executed in this process' invocation.
const sessionCache = new Map<string, Promise<Session>>();
let diskCache: Record<string, { token: string; admin: AdminProfile }> | undefined;

function readDiskCache(): Record<string, { token: string; admin: AdminProfile }> | undefined {
  if (diskCache !== undefined) return diskCache;
  try {
    diskCache = JSON.parse(readFileSync(SESSIONS_FILE, 'utf-8'));
  } catch {
    diskCache = {};
  }
  return diskCache;
}

async function doLogin(email: string, password: string): Promise<Session> {
  const fromDisk = readDiskCache()?.[email];
  if (fromDisk) return fromDisk;

  const ctx = await apiContext();
  const res = await ctx.post('/v1/admin/auth/login', { data: { email, password } });
  if (!res.ok()) {
    throw new Error(`login failed for ${email}: ${res.status()} ${await res.text()}`);
  }
  const body = (await res.json()) as { access_token: string; admin: AdminProfile };
  return { token: body.access_token, admin: body.admin };
}

/** Returns the session for a fixed E2E admin — from the global-setup disk cache when available, otherwise a real (per-process cached) login. */
export async function loginViaApi(email: string, password: string): Promise<Session> {
  const key = `${email}:${password}`;
  let pending = sessionCache.get(key);
  if (!pending) {
    pending = doLogin(email, password);
    sessionCache.set(key, pending);
  }
  try {
    return await pending;
  } catch (err) {
    sessionCache.delete(key); // don't poison the cache with a failed attempt
    throw err;
  }
}

/** Writes the session into sessionStorage before any app script runs (so useAdminStore's lazy init picks it up). */
export async function seedSession(target: BrowserContext | Page, session: Session): Promise<void> {
  await target.addInitScript(
    ({ key, value }) => {
      window.sessionStorage.setItem(key, value);
    },
    { key: STORAGE_KEY, value: JSON.stringify(session) },
  );
}

/** Directly authenticated APIRequestContext for a role — for beforeEach data setup that must not go through the UI. */
export async function apiAs(email: string, password: string): Promise<{ ctx: APIRequestContext; session: Session }> {
  const session = await loginViaApi(email, password);
  const ctx = await request.newContext({
    baseURL: apiBaseURL(),
    extraHTTPHeaders: { Authorization: `Bearer ${session.token}` },
  });
  return { ctx, session };
}

/**
 * Files a fresh reversal request (POST /v1/admin/reversals) against a real completed
 * transaction and returns {id, refCode}.
 *
 * A completed reversal is final: one reversal per transaction (ADR-0013), so a transaction that
 * a test approved is gone for good. The seed (`go run ./cmd/seed --e2e-admins`) therefore creates
 * a dedicated pool of completed p2p transactions (key prefix `e2e-reversible-`, each with a
 * completed p2p_transfer workflow instance, which is how the admin API lists them). Every call
 * takes the next transaction of that pool that has not been reversed yet. Two Playwright
 * workers may race for the same one; the loser gets 409 and simply moves on to the next.
 *
 * A request that a test does not decide stays pending and keeps its transaction locked; tests
 * that do not need that call `releaseReversal`, which rejects it and frees the transaction.
 */
export async function createFreshReversal(
  api: APIRequestContext,
  label: string,
): Promise<{ id: string; refCode: string }> {
  const candidates = await listReversiblePool(api);
  if (candidates.length === 0) {
    throw new Error('the E2E reversible pool is empty: run `go run ./cmd/seed --e2e-admins`');
  }

  // Start at a different offset per call so concurrent workers rarely pick the same one.
  const start = Math.floor(Math.random() * candidates.length);
  let lastErr = '';
  for (let n = 0; n < candidates.length; n++) {
    const refCode = candidates[(start + n) % candidates.length];
    const res = await api.post('/v1/admin/reversals', {
      data: { ref_code: refCode, reason: `${label} — automated E2E fixture` },
    });
    if (res.ok()) {
      const body = (await res.json()) as { request_id: string };
      return { id: body.request_id, refCode };
    }
    lastErr = `${refCode}: ${res.status()} ${await res.text()}`;
    // 409 = already reversed or a sibling holds a pending request on it: try the next one.
    if (res.status() !== 409) throw new Error(`reversal request failed: ${lastErr}`);
  }
  throw new Error(`the E2E reversible pool is used up (${candidates.length} transactions) — last attempt: ${lastErr}`);
}

/** Ref codes of the completed workflow instances of the E2E pool, newest listing first. */
async function listReversiblePool(api: APIRequestContext): Promise<string[]> {
  const refs = new Set<string>();
  for (let page = 1; page <= 5; page++) {
    const res = await api.get(`/v1/admin/workflows?status=completed&definition=p2p_transfer&limit=100&page=${page}`);
    if (!res.ok()) throw new Error(`GET /v1/admin/workflows failed: ${res.status()} ${await res.text()}`);
    const body = (await res.json()) as { workflows?: Array<{ txn_ref?: string }>; total_pages?: number };
    for (const w of body.workflows ?? []) if (w.txn_ref) refs.add(w.txn_ref);
    if (page >= (body.total_pages ?? 1)) break;
  }
  return [...refs];
}
/** Rejects a reversal request so its underlying transaction becomes reversible again for the next caller (see createFreshReversal). */
export async function releaseReversal(checkerApi: APIRequestContext, id: string): Promise<void> {
  await checkerApi.post(`/v1/admin/approvals/${id}/decide`, {
    data: { action: 'reject', reason: 'cleanup — freeing shared fixture transaction for the next E2E case' },
  });
}

interface AuthFixtures {
  /** Page pre-authenticated as the fixed E2E maker (role ops). */
  asMaker: Page;
  /** Page pre-authenticated as the fixed E2E checker (role superadmin). */
  asChecker: Page;
  /** Authenticated API client for the maker — for beforeEach data setup via direct API calls. */
  makerApi: APIRequestContext;
  /** Authenticated API client for the checker. */
  checkerApi: APIRequestContext;
}

export const test = base.extend<AuthFixtures>({
  asMaker: async ({ browser }, use) => {
    const session = await loginViaApi(MAKER.email, MAKER.password);
    const context = await browser.newContext();
    await seedSession(context, session);
    const page = await context.newPage();
    await use(page);
    await context.close();
  },
  asChecker: async ({ browser }, use) => {
    const session = await loginViaApi(CHECKER.email, CHECKER.password);
    const context = await browser.newContext();
    await seedSession(context, session);
    const page = await context.newPage();
    await use(page);
    await context.close();
  },
  makerApi: async ({}, use) => {
    const { ctx } = await apiAs(MAKER.email, MAKER.password);
    await use(ctx);
    await ctx.dispose();
  },
  checkerApi: async ({}, use) => {
    const { ctx } = await apiAs(CHECKER.email, CHECKER.password);
    await use(ctx);
    await ctx.dispose();
  },
});

export { expect };
