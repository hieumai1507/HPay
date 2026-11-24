import Constants from 'expo-constants';
import * as Crypto from 'expo-crypto';

import { ApiError, NetworkError } from '@/src/lib/errors';
import { getDeviceId } from '@/src/stores/device';
import { getAccessToken } from '@/src/stores/session';

import type { components } from './schema';

type ErrorBody = components['schemas']['Error'];

export const API_BASE_URL =
  (Constants.expoConfig?.extra?.apiBaseUrl as string | undefined) ?? 'http://localhost:8080';

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: unknown;
  /** POST endpoints that create money movement require this (doc 02 §6). */
  idempotencyKey?: string;
  /** Public endpoints (register/login/refresh) — no Authorization header, no 401-refresh dance. */
  skipAuth?: boolean;
  headers?: Record<string, string>;
  signal?: AbortSignal;
}

/**
 * Fires one HTTP request with the standard headers. Does NOT handle 401-refresh —
 * that orchestration lives in `apiRequest` (below) and in `refreshAccessToken`
 * (../api/refresh.ts), which calls this directly to avoid recursing through the
 * retry logic when refreshing the token itself.
 */
export async function rawRequest(
  path: string,
  options: RequestOptions,
  accessTokenOverride?: string | null,
): Promise<Response> {
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'X-Request-Id': Crypto.randomUUID(),
    ...options.headers,
  };

  const deviceId = getDeviceId();
  if (deviceId) headers['X-Device-Id'] = deviceId;
  if (options.idempotencyKey) headers['Idempotency-Key'] = options.idempotencyKey;

  let body: string | undefined;
  if (options.body !== undefined) {
    headers['Content-Type'] = 'application/json';
    body = stringifyWithBigInt(options.body);
  }

  if (!options.skipAuth) {
    const token = accessTokenOverride !== undefined ? accessTokenOverride : getAccessToken();
    if (token) headers.Authorization = `Bearer ${token}`;
  }

  try {
    return await fetch(`${API_BASE_URL}${path}`, {
      method: options.method ?? 'GET',
      headers,
      body,
      signal: options.signal,
    });
  } catch {
    throw new NetworkError();
  }
}

/**
 * JSON.stringify that emits BigInt values as RAW JSON integers (M06: money
 * never passes through Number/float on its way to the wire). Uses a
 * placeholder-substitution pass because JSON.stringify cannot natively emit
 * a number token from a BigInt.
 */
export function stringifyWithBigInt(value: unknown): string {
  const bigints: string[] = [];
  const withPlaceholders = JSON.stringify(value, (_key, v) => {
    if (typeof v === 'bigint') {
      bigints.push(v.toString());
      return `__bigint_${bigints.length - 1}__`;
    }
    return v;
  });
  return withPlaceholders.replace(/"__bigint_(\d+)__"/g, (_m, i: string) => bigints[Number(i)]);
}

export async function parseErrorBody(res: Response): Promise<ErrorBody | null> {
  try {
    return (await res.json()) as ErrorBody;
  } catch {
    return null;
  }
}

async function toApiError(res: Response): Promise<ApiError> {
  const body = await parseErrorBody(res);
  return new ApiError(body?.code ?? 'ERR_UNKNOWN', res.status, body?.request_id, body?.message);
}

type UnauthorizedHandler = () => Promise<string>;

// Set by ./refresh at module load (registerUnauthorizedHandler) — dependency-inverted
// so client.ts never has to import refresh.ts (which itself needs rawRequest from here).
let unauthorizedHandler: UnauthorizedHandler | null = null;

export function registerUnauthorizedHandler(handler: UnauthorizedHandler): void {
  unauthorizedHandler = handler;
}

/**
 * The request function screens/hooks should use. On a 401 (and only when the request
 * itself required auth), it triggers a single-flight refresh and retries EXACTLY once
 * with the new token (M04). If refresh fails, the original 401 is surfaced as an
 * ApiError — the session store will already have moved to `unauthenticated` by then.
 */
export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  let res = await rawRequest(path, options);

  if (res.status === 401 && !options.skipAuth && unauthorizedHandler) {
    try {
      const newToken = await unauthorizedHandler();
      res = await rawRequest(path, options, newToken);
    } catch {
      // refreshAccessToken already cleared the session; fall through and surface
      // the original 401 below.
    }
  }

  if (!res.ok) {
    throw await toApiError(res);
  }

  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}
