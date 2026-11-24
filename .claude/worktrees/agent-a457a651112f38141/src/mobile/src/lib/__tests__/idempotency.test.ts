import { renderHook } from '@testing-library/react-native';

import { generateIdempotencyKey, useIdempotencyKey } from '../idempotency';

describe('TestEdge_M01_IdempotencyKey_StablePerConfirmFlow', () => {
  it('generates a well-formed UUID', () => {
    const key = generateIdempotencyKey();
    expect(key).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i);
  });

  it('keeps the same key across re-renders of the same confirm flow (double-tap safe)', async () => {
    const { result, rerender } = await renderHook(
      (props: { resetKey: string }) => useIdempotencyKey(props.resetKey),
      { initialProps: { resetKey: '0912345678:50000' } },
    );

    const firstKey = result.current;
    expect(firstKey).toBeTruthy();

    // Simulate re-renders from double-tapping "Chuyển tiền" (state updates, no navigation).
    await rerender({ resetKey: '0912345678:50000' });
    await rerender({ resetKey: '0912345678:50000' });

    expect(result.current).toBe(firstKey);
  });

  it('mints a fresh key when the flow changes (different receiver/amount)', async () => {
    const { result, rerender } = await renderHook(
      (props: { resetKey: string }) => useIdempotencyKey(props.resetKey),
      { initialProps: { resetKey: '0912345678:50000' } },
    );

    const firstKey = result.current;

    await rerender({ resetKey: '0987654321:100000' });

    expect(result.current).not.toBe(firstKey);
  });
});
