import * as SecureStore from 'expo-secure-store';

import { ApiError, NetworkError } from '../errors';
import {
  INTENT_KEY_TTL_MS,
  __resetIntentKeyMemory,
  getIntentKey,
  releaseIntentKey,
  shouldReleaseIntentKey,
} from '../intentKey';

const resetSecureStore = () => (SecureStore as unknown as { __reset: () => void }).__reset();

beforeEach(() => {
  resetSecureStore();
  __resetIntentKeyMemory();
  jest.restoreAllMocks();
});

describe('TestEdge_C03_IntentKey_StablePerIntent', () => {
  it('returns the same key for the same intent across calls', async () => {
    const a = await getIntentKey('transfer:0912345678:50000:');
    const b = await getIntentKey('transfer:0912345678:50000:');
    expect(a).toBe(b);
  });

  it('survives a remount / process memory loss (persisted in SecureStore)', async () => {
    const a = await getIntentKey('transfer:0912345678:50000:');
    __resetIntentKeyMemory(); // simulates the confirm screen remounting after a crash
    const b = await getIntentKey('transfer:0912345678:50000:');
    expect(b).toBe(a);
  });

  it('gives different intents different keys', async () => {
    const a = await getIntentKey('transfer:0912345678:50000:');
    const b = await getIntentKey('transfer:0912345678:60000:');
    expect(a).not.toBe(b);
  });

  it('mints a fresh key after the intent was released (success or definitive rejection)', async () => {
    const a = await getIntentKey('vault:v1:deposit:1000');
    await releaseIntentKey('vault:v1:deposit:1000');
    const b = await getIntentKey('vault:v1:deposit:1000');
    expect(b).not.toBe(a);
  });

  it('expires a stale key after the TTL', async () => {
    const now = jest.spyOn(Date, 'now').mockReturnValue(1_000_000);
    const a = await getIntentKey('transfer:x:1:');
    now.mockReturnValue(1_000_000 + INTENT_KEY_TTL_MS + 1);
    const b = await getIntentKey('transfer:x:1:');
    expect(b).not.toBe(a);
  });
});

describe('TestEdge_C03_IntentKey_WhenToRelease', () => {
  it('keeps the key when the outcome is ambiguous (network, 5xx, 409, 429)', () => {
    expect(shouldReleaseIntentKey(new NetworkError())).toBe(false);
    expect(shouldReleaseIntentKey(new ApiError('ERR_X', 503))).toBe(false);
    expect(shouldReleaseIntentKey(new ApiError('ERR_X', 409))).toBe(false);
    expect(shouldReleaseIntentKey(new ApiError('ERR_X', 429))).toBe(false);
    expect(shouldReleaseIntentKey(new Error('unknown'))).toBe(false);
  });

  it('releases the key on a definitive 4xx rejection', () => {
    expect(shouldReleaseIntentKey(new ApiError('ERR_INSUFFICIENT_BALANCE', 422))).toBe(true);
    expect(shouldReleaseIntentKey(new ApiError('ERR_AMOUNT_INVALID', 400))).toBe(true);
  });
});
