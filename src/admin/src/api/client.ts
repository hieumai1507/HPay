import { useAdminStore } from '@/stores/admin';
import { ApiError, messageFor } from './errors';

const BASE_URL = import.meta.env.VITE_API_URL ?? '';

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: unknown;
  /** Extra headers, e.g. Idempotency-Key for money-moving POSTs. */
  headers?: Record<string, string>;
  signal?: AbortSignal;
}

/**
 * Single fetch wrapper for every /v1 call. Injects the admin JWT, parses the
 * backend `{code, message}` error envelope into ApiError, and clears the
 * session on 401 so the router can show the re-login modal (edge A03).
 */
export async function api<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { token, clearSession } = useAdminStore.getState();

  const headers: Record<string, string> = { ...options.headers };
  if (options.body !== undefined) headers['Content-Type'] = 'application/json';
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const requestBody =
    options.body !== undefined
      ? typeof options.body === 'string'
        ? options.body
        : JSON.stringify(options.body)
      : undefined;

  const res = await fetch(BASE_URL + path, {
    method: options.method ?? 'GET',
    headers,
    body: requestBody,
    signal: options.signal,
  });

  if (res.status === 204) return undefined as T;

  const isJson = res.headers.get('content-type')?.includes('application/json') ?? false;
  const data: unknown = isJson ? await res.json() : await res.text();

  if (!res.ok) {
    const { code, message } = extractError(data, res.status);
    if (res.status === 401) clearSession();
    throw new ApiError(res.status, code, messageFor(code, message), data);
  }

  return data as T;
}

export const apiRequest = api;

function extractError(data: unknown, status: number): { code: string; message?: string } {
  if (typeof data === 'object' && data !== null) {
    const obj = data as Record<string, unknown>;
    const code = typeof obj.code === 'string' ? obj.code : undefined;
    const message = typeof obj.message === 'string' ? obj.message : undefined;
    if (code) return { code, message };
  }
  return { code: status === 401 ? 'ERR_UNAUTHORIZED' : `HTTP_${status}` };
}
