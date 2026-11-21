import { request } from '@playwright/test';
import { mkdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

/**
 * Runs ONCE for the whole `playwright test` invocation (regardless of worker
 * count), before any test file starts. POST /v1/admin/auth/login is
 * rate-limited to 5 req/min/IP (hpay-backend router.go RateLimit(...)) —
 * with `fullyParallel` + multiple workers, letting every worker/test log in
 * independently blows through that limit in seconds. So the two fixed E2E
 * admins log in exactly once here; every worker reads the resulting tokens
 * back from disk (e2e/fixtures/auth.ts loginViaApi) instead of hitting
 * /login again. The 15-minute access-token TTL comfortably covers a full
 * suite run.
 */

const CACHE_DIR = path.join(__dirname, '.auth');
export const SESSIONS_FILE = path.join(CACHE_DIR, 'sessions.json');

// Default matches the backend e2e seed; override with E2E_ADMIN_PASSWORD.
const E2E_ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? 'E2ETest@123';

const CREDENTIALS = [
  { email: 'e2e-maker@HPay.dev', password: E2E_ADMIN_PASSWORD },
  { email: 'e2e-checker@HPay.dev', password: E2E_ADMIN_PASSWORD },
];

export default async function globalSetup() {
  const apiBaseURL = process.env.E2E_API_URL ?? 'http://localhost:8080';
  const ctx = await request.newContext({ baseURL: apiBaseURL });

  const sessions: Record<string, { token: string; admin: unknown }> = {};
  for (const { email, password } of CREDENTIALS) {
    const res = await ctx.post('/v1/admin/auth/login', { data: { email, password } });
    if (!res.ok()) {
      throw new Error(`global-setup: login failed for ${email}: ${res.status()} ${await res.text()}`);
    }
    const body = (await res.json()) as { access_token: string; admin: unknown };
    sessions[email] = { token: body.access_token, admin: body.admin };
  }
  await ctx.dispose();

  mkdirSync(CACHE_DIR, { recursive: true });
  writeFileSync(SESSIONS_FILE, JSON.stringify(sessions, null, 2));
}
