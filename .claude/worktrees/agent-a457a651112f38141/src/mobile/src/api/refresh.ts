import { useEffect, useState } from 'react';

import { ApiError } from '@/src/lib/errors';
import { ensureDeviceId } from '@/src/stores/device';
import {
  clearStoredRefreshToken,
  getStoredRefreshToken,
  storeRefreshToken,
  useSessionStore,
} from '@/src/stores/session';

import { parseErrorBody, rawRequest, registerUnauthorizedHandler } from './client';
import type { components } from './schema';

type TokenPair = components['schemas']['TokenPair'];

let refreshInFlight: Promise<string> | null = null;

async function doRefresh(): Promise<string> {
  const refreshToken = await getStoredRefreshToken();
  if (!refreshToken) {
    useSessionStore.getState().clearSession('refresh_expired');
    throw new ApiError('ERR_TOKEN_EXPIRED', 401);
  }

  const res = await rawRequest(
    '/v1/auth/refresh',
    { method: 'POST', body: { refresh_token: refreshToken }, skipAuth: true },
    null,
  );

  if (!res.ok) {
    const errBody = await parseErrorBody(res);
    const code = errBody?.code ?? 'ERR_INVALID_TOKEN';
    // Only a definitive "this refresh token is not valid" (401/403) ends the session.
    // 5xx, 429 and everything else are temporary: keep the stored token and the session
    // and surface a retryable error (audit mobile finding; network failures never get
    // here, rawRequest throws NetworkError before a response exists).
    if (res.status === 401 || res.status === 403) {
      await clearStoredRefreshToken();
      // ERR_TOKEN_REUSED means the refresh family was revoked (theft detection, M05) -
      // everything else (expired/invalid) is a plain "please log in again".
      useSessionStore
        .getState()
        .clearSession(code === 'ERR_TOKEN_REUSED' ? 'refresh_revoked' : 'refresh_expired');
    }
    throw new ApiError(code, res.status, errBody?.request_id, errBody?.message);
  }
  const data = (await res.json()) as { tokens: TokenPair };
  const tokens = data.tokens;
  if (!tokens.access_token || !tokens.refresh_token) {
    throw new ApiError('ERR_INVALID_TOKEN', 500);
  }

  await storeRefreshToken(tokens.refresh_token);
  useSessionStore.getState().setSession(tokens.access_token);
  return tokens.access_token;
}

/**
 * Rotates the refresh token and installs the new access token in the session store.
 * Single-flight: concurrent callers (e.g. 3 requests that all hit 401 at once) share
 * one in-flight POST /v1/auth/refresh instead of racing (M04).
 */
export function refreshAccessToken(): Promise<string> {
  if (!refreshInFlight) {
    refreshInFlight = doRefresh().finally(() => {
      refreshInFlight = null;
    });
  }
  return refreshInFlight;
}

// Wire ourselves into client.ts's 401 handling. Importing this module (which every
// consumer must do to call useSessionBootstrap) is what activates the retry-once path.
registerUnauthorizedHandler(refreshAccessToken);

/**
 * Runs once on app start: if a refresh token is on disk, silently exchange it for a
 * fresh access token so the splash gate can route straight to (main) instead of (auth).
 * Returns `true` while bootstrapping (screens should render nothing / native splash).
 */
export function useSessionBootstrap(): boolean {
  const status = useSessionStore((s) => s.status);
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    if (checked) return;
    (async () => {
      await ensureDeviceId();
      const stored = await getStoredRefreshToken();
      if (!stored) {
        useSessionStore.getState().clearSession(null);
      } else {
        try {
          await refreshAccessToken();
        } catch {
          // 401/403: doRefresh() already cleared the session and the stored token.
          // Temporary failure (offline, 5xx, 429): keep the stored refresh token so the
          // next launch retries, but leave the bootstrapping state so the app can render
          // the login screen instead of a permanent splash.
          if (useSessionStore.getState().status === 'bootstrapping') {
            useSessionStore.getState().clearSession(null);
          }
        }
      }
      setChecked(true);
    })();
  }, [checked]);

  return status === 'bootstrapping' || !checked;
}
