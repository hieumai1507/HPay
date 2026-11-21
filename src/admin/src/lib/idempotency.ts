/**
 * One Idempotency-Key per user action (constitution C-03). Call
 * idempotencyHeaders() once per mutate() invocation, i.e. once per click.
 * NOTE: the admin routes in src/backend do not read this header yet (backend
 * gap); sending it is harmless and makes the client ready.
 */
export function newIdempotencyKey(): string {
  const c = globalThis.crypto;
  if (c && typeof c.randomUUID === 'function') return c.randomUUID();
  const bytes = new Uint8Array(16);
  c.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

export function idempotencyHeaders(): Record<string, string> {
  return { 'Idempotency-Key': newIdempotencyKey() };
}