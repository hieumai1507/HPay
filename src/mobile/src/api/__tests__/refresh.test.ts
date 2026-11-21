import * as SecureStore from 'expo-secure-store';

import { apiRequest } from '../client';
import { refreshAccessToken } from '../refresh';
import { getStoredRefreshToken, storeRefreshToken, useSessionStore } from '@/src/stores/session';

function jsonResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response;
}

// __reset is exported by the manual mock (__mocks__/expo-secure-store.ts) but not by
// the real package's types — cast through unknown since Jest substitutes the mock at runtime.
const resetSecureStore = () => (SecureStore as unknown as { __reset: () => void }).__reset();

beforeEach(() => {
  resetSecureStore();
  useSessionStore.setState({ accessToken: 'stale-token', user: null, status: 'authenticated', logoutReason: null });
  global.fetch = jest.fn();
});

describe('TestEdge_M04_Refresh_SingleFlight_RetryOnce', () => {
  it('collapses 3 concurrent 401s into exactly one refresh call, and retries each request once with the new token', async () => {
    await storeRefreshToken('valid-refresh-token');

    let refreshCalls = 0;
    let protectedCalls = 0;

    (global.fetch as jest.Mock).mockImplementation(async (url: string, init: RequestInit) => {
      if (url.includes('/v1/auth/refresh')) {
        refreshCalls += 1;
        return jsonResponse(200, {
          tokens: { access_token: 'fresh-token', refresh_token: 'rotated-refresh', expires_in: 900 },
        });
      }
      if (url.includes('/v1/protected')) {
        protectedCalls += 1;
        const headers = init.headers as Record<string, string>;
        if (headers.Authorization === 'Bearer fresh-token') {
          return jsonResponse(200, { ok: true });
        }
        return jsonResponse(401, { code: 'ERR_TOKEN_EXPIRED', message: 'expired' });
      }
      throw new Error(`unexpected url ${url}`);
    });

    const results = await Promise.all([
      apiRequest('/v1/protected'),
      apiRequest('/v1/protected'),
      apiRequest('/v1/protected'),
    ]);

    expect(results).toEqual([{ ok: true }, { ok: true }, { ok: true }]);
    expect(refreshCalls).toBe(1); // single-flight — not 3
    expect(protectedCalls).toBe(6); // 3 initial 401s + 3 retries
    expect(useSessionStore.getState().accessToken).toBe('fresh-token');
  });

  it('does not retry a second time if the retried request also comes back 401', async () => {
    await storeRefreshToken('valid-refresh-token');

    let protectedCalls = 0;
    (global.fetch as jest.Mock).mockImplementation(async (url: string) => {
      if (url.includes('/v1/auth/refresh')) {
        return jsonResponse(200, {
          tokens: { access_token: 'fresh-token', refresh_token: 'rotated-refresh', expires_in: 900 },
        });
      }
      if (url.includes('/v1/protected')) {
        protectedCalls += 1;
        return jsonResponse(401, { code: 'ERR_INVALID_TOKEN', message: 'still invalid' });
      }
      throw new Error(`unexpected url ${url}`);
    });

    await expect(apiRequest('/v1/protected')).rejects.toMatchObject({ code: 'ERR_INVALID_TOKEN' });
    expect(protectedCalls).toBe(2); // original + exactly one retry, no loop
  });
});

describe('TestEdge_M05_RefreshRevoked_ForceLogout', () => {
  it('clears the session and secure-store refresh token when the server reports ERR_TOKEN_REUSED', async () => {
    await storeRefreshToken('stolen-refresh-token');

    (global.fetch as jest.Mock).mockImplementation(async (url: string) => {
      if (url.includes('/v1/auth/refresh')) {
        return jsonResponse(401, { code: 'ERR_TOKEN_REUSED', message: 'refresh token reused' });
      }
      throw new Error(`unexpected url ${url}`);
    });

    await expect(refreshAccessToken()).rejects.toMatchObject({ code: 'ERR_TOKEN_REUSED' });

    const state = useSessionStore.getState();
    expect(state.status).toBe('unauthenticated');
    expect(state.accessToken).toBeNull();
    expect(state.logoutReason).toBe('refresh_revoked');
    await expect(getStoredRefreshToken()).resolves.toBeNull();
  });

  it('reports refresh_expired (not refresh_revoked) when there is simply no stored refresh token', async () => {
    await expect(refreshAccessToken()).rejects.toMatchObject({ code: 'ERR_TOKEN_EXPIRED' });
    expect(useSessionStore.getState().logoutReason).toBe('refresh_expired');
  });
});
