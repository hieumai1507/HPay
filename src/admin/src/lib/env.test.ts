import { describe, expect, it } from 'vitest';

import { isMockMode } from './env';
import { idempotencyHeaders, newIdempotencyKey } from './idempotency';

describe('isMockMode', () => {
  it('is on only for a dev server with the explicit flag', () => {
    expect(isMockMode(true, 'true')).toBe(true);
    expect(isMockMode(true, undefined)).toBe(false);
    expect(isMockMode(true, '1')).toBe(false);
    expect(isMockMode(false, 'true')).toBe(false); // production build can never mock
    expect(isMockMode(undefined, 'true')).toBe(false);
  });
});

describe('idempotency key', () => {
  it('is fresh for every call and sent under the Idempotency-Key header', () => {
    const a = newIdempotencyKey();
    const b = newIdempotencyKey();
    expect(a).not.toBe(b);
    expect(a.length).toBeGreaterThanOrEqual(16);
    expect(Object.keys(idempotencyHeaders())).toEqual(['Idempotency-Key']);
  });
});
