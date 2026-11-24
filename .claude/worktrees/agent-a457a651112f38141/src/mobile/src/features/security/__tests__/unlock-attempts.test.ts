import { MAX_UNLOCK_ATTEMPTS, runUnlockAttempt } from '../unlockAttempts';

describe('TestEdge_C07_AppLock_NoClientPin_LockoutAfter5', () => {
  it('unlocks on success and resets the counter', async () => {
    const res = await runUnlockAttempt(async () => ({ success: true }), 3);
    expect(res).toEqual({ outcome: 'unlocked', failedAttempts: 0 });
  });

  it('counts a failed attempt', async () => {
    const res = await runUnlockAttempt(async () => ({ success: false, error: 'authentication_failed' }), 0);
    expect(res).toEqual({ outcome: 'failed', failedAttempts: 1 });
  });

  it('does not count a user cancel as a wrong attempt', async () => {
    const res = await runUnlockAttempt(async () => ({ success: false, error: 'user_cancel' }), 2);
    expect(res).toEqual({ outcome: 'cancelled', failedAttempts: 2 });
  });

  it('signs out on the 5th failed attempt', async () => {
    const res = await runUnlockAttempt(
      async () => ({ success: false, error: 'authentication_failed' }),
      MAX_UNLOCK_ATTEMPTS - 1,
    );
    expect(res).toEqual({ outcome: 'signed_out', failedAttempts: MAX_UNLOCK_ATTEMPTS });
  });

  it('treats a thrown hardware error as a failed attempt, never as success', async () => {
    const res = await runUnlockAttempt(async () => {
      throw new Error('hardware');
    }, 0);
    expect(res.outcome).toBe('failed');
  });

  it('allows exactly 5 attempts', () => {
    expect(MAX_UNLOCK_ATTEMPTS).toBe(5);
  });
});
