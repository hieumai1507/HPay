import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { api } from './client';
import { ApiError } from './errors';
import { useAdminStore } from '@/stores/admin';

function stubFetch(body: string, init: { status: number; contentType?: string }) {
  const fn = vi.fn(async () =>
    new Response(body, {
      status: init.status,
      headers: init.contentType ? { 'content-type': init.contentType } : {},
    }),
  );
  vi.stubGlobal('fetch', fn);
  return fn;
}

describe('api client', () => {
  beforeEach(() => {
    useAdminStore.getState().clearSession();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('rejects a 200 HTML body (proxy error page) instead of returning the text as T', async () => {
    stubFetch('<html>502 Bad Gateway</html>', { status: 200, contentType: 'text/html' });
    const err = await api('/v1/admin/kyc').catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).code).toBe('ERR_NON_JSON_RESPONSE');
    expect((err as ApiError).message).toMatch(/non-JSON/i);
  });

  it('turns an HTML 502 into an ApiError with a clear message and no HTML payload', async () => {
    stubFetch('<html>bad gateway</html>', { status: 502, contentType: 'text/html' });
    const err = (await api('/v1/admin/kyc').catch((e: unknown) => e)) as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(502);
    expect(err.message).toMatch(/502/);
    expect(err.payload).toBeUndefined();
  });

  it('rejects a malformed JSON body', async () => {
    stubFetch('{oops', { status: 200, contentType: 'application/json' });
    const err = (await api('/x').catch((e: unknown) => e)) as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe('ERR_NON_JSON_RESPONSE');
  });

  it('still returns parsed JSON and parses the {code,message} error envelope', async () => {
    stubFetch('{"a":1}', { status: 200, contentType: 'application/json' });
    await expect(api<{ a: number }>('/x')).resolves.toEqual({ a: 1 });
    stubFetch('{"code":"ERR_CONFLICT","message":"x"}', { status: 409, contentType: 'application/json' });
    const err = (await api('/x').catch((e: unknown) => e)) as ApiError;
    expect(err.code).toBe('ERR_CONFLICT');
  });

  it('sends the caller-provided Idempotency-Key header', async () => {
    const fn = stubFetch('{}', { status: 200, contentType: 'application/json' });
    await api('/x', { method: 'POST', body: {}, headers: { 'Idempotency-Key': 'k1' } });
    const init = (fn.mock.calls[0] as unknown as [string, RequestInit])[1];
    expect((init.headers as Record<string, string>)['Idempotency-Key']).toBe('k1');
  });
});