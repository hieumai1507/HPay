import { act, fireEvent, render, screen, waitFor } from '@testing-library/react-native';
import { Pressable, Text } from 'react-native';

import { apiRequest } from '@/src/api/client';

import { useTransactionAuth, type AuthFactors, type AuthRequired } from '../useTransactionAuth';
import { apiError, wrapper } from '../testUtils';

jest.mock('@/src/api/client', () => ({ apiRequest: jest.fn() }));
const api = apiRequest as jest.Mock;

let result: Promise<AuthFactors | null> | null = null;
let band: AuthRequired = 'none';

function Harness() {
  const { collect, prompt } = useTransactionAuth();
  return (
    <>
      <Pressable
        accessibilityLabel="ask"
        onPress={() => {
          result = collect(band, { transactionId: 'tx-1' });
        }}
      >
        <Text>ask</Text>
      </Pressable>
      {prompt}
    </>
  );
}

async function ask(b: AuthRequired) {
  band = b;
  await render(<Harness />, { wrapper: wrapper() });
  await fireEvent.press(screen.getByLabelText('ask'));
}

// OTPInput renders 6 boxes per instance; the paste path fills all of them from the first box.
async function tick(seconds: number) {
  for (let i = 0; i < seconds; i++) {
    await act(async () => {
      jest.advanceTimersByTime(1000);
    });
  }
}

const typeInto = (instance: number, value: string) =>
  fireEvent.changeText(screen.getAllByLabelText('OTP digit 1')[instance], value);

describe('Test_useTransactionAuth', () => {
  beforeEach(() => {
    api.mockReset();
    result = null;
  });

  it('none: resolves {} with no prompt and no request', async () => {
    await ask('none');
    await expect(result).resolves.toEqual({});
    expect(screen.queryByLabelText('stepup-confirm')).toBeNull();
    expect(screen.queryByLabelText('pin-confirm')).toBeNull();
    expect(api).not.toHaveBeenCalled();
  });

  it('pin: carries exactly the pin, no challenge call', async () => {
    await ask('pin');
    await typeInto(0, '482913');
    await fireEvent.press(screen.getByLabelText('pin-confirm'));
    await expect(result).resolves.toStrictEqual({ pin: '482913' });
    expect(api).not.toHaveBeenCalled();
  });

  it('otp: calls challenge first, then carries exactly pin and otp', async () => {
    api.mockResolvedValue({ status: 'otp_sent' });
    await ask('otp');
    expect(api).toHaveBeenCalledWith('/v1/transactions/challenge', {
      method: 'POST',
      body: { transaction_id: 'tx-1' },
    });
    await screen.findByLabelText('stepup-confirm');
    await typeInto(0, '482913');
    await typeInto(1, '123456');
    await fireEvent.press(screen.getByLabelText('stepup-confirm'));
    const factors = await result;
    expect(factors).toStrictEqual({ pin: '482913', otp: '123456' });
    expect(factors).not.toHaveProperty('totp');
  });

  it('otp: shows the dev code only when the API returns it', async () => {
    api.mockResolvedValue({ status: 'otp_sent', otp_dev: '654321' });
    await ask('otp');
    await screen.findByText('Dev code: 654321');
  });

  it('otp: does not show a dev code when the API does not return one', async () => {
    api.mockResolvedValue({ status: 'otp_sent' });
    await ask('otp');
    await screen.findByLabelText('stepup-confirm');
    expect(screen.queryByText(/Dev code/)).toBeNull();
  });

  it('otp: resend is disabled during the cooldown and shows the countdown', async () => {
    jest.useFakeTimers();
    try {
      api.mockResolvedValue({ status: 'otp_sent' });
      await ask('otp');
      await screen.findByText('Resend code in 30s');
      expect(screen.getByLabelText('resend-code').props.accessibilityState.disabled).toBe(true);
      await tick(30);
      await screen.findByText('Resend code');
      expect(screen.getByLabelText('resend-code').props.accessibilityState.disabled).toBe(false);
    } finally {
      jest.useRealTimers();
    }
  });

  it('otp: a refused resend (ERR_OTP_COOLDOWN) keeps the button locked and shows the copy', async () => {
    jest.useFakeTimers();
    try {
      api.mockResolvedValueOnce({ status: 'otp_sent' });
      await ask('otp');
      await tick(30);
      api.mockRejectedValueOnce(apiError('ERR_OTP_COOLDOWN', 429));
      await screen.findByText('Resend code');
      await fireEvent.press(screen.getByLabelText('resend-code'));
      // react-query schedules mutation work on timers; flush them under fake timers.
      await tick(3);
      await waitFor(() => expect(api).toHaveBeenCalledTimes(2));
      await screen.findByText('Please wait before requesting another OTP code.');
      await screen.findByText(/Resend code in \d+s/);
    } finally {
      jest.useRealTimers();
    }
  });

  it('otp: a failed first challenge rejects, opens no prompt and sends no confirm', async () => {
    api.mockRejectedValue(apiError('ERR_RATE_LIMITED', 429));
    await ask('otp');
    await expect(result).rejects.toMatchObject({ code: 'ERR_RATE_LIMITED' });
    expect(screen.queryByLabelText('stepup-confirm')).toBeNull();
    expect(api).toHaveBeenCalledTimes(1);
  });
  it('2fa: carries exactly pin and totp, never an otp, no challenge call', async () => {
    await ask('2fa');
    await typeInto(0, '482913');
    await typeInto(1, '123456');
    await fireEvent.press(screen.getByLabelText('stepup-confirm'));
    const factors = await result;
    expect(factors).toStrictEqual({ pin: '482913', totp: '123456' });
    expect(factors).not.toHaveProperty('otp');
    expect(api).not.toHaveBeenCalled();
  });

  it('2fa: the recovery-code toggle accepts a recovery code in the same field', async () => {
    await ask('2fa');
    expect(screen.getByText('Authenticator code')).toBeTruthy();
    await fireEvent.press(screen.getByLabelText('toggle-recovery'));
    expect(screen.getByText('Recovery code')).toBeTruthy();
    await typeInto(0, '482913');
    await fireEvent.changeText(screen.getByLabelText('recovery-code-input'), 'abcd-efgh-ijkl');
    await fireEvent.press(screen.getByLabelText('stepup-confirm'));
    await expect(result).resolves.toStrictEqual({ pin: '482913', totp: 'abcd-efgh-ijkl' });
  });

  it('2fa: confirm stays disabled until PIN and code are complete', async () => {
    await ask('2fa');
    await typeInto(0, '4829');
    expect(screen.getByLabelText('stepup-confirm').props.accessibilityState.disabled).toBe(true);
    await typeInto(0, '482913');
    expect(screen.getByLabelText('stepup-confirm').props.accessibilityState.disabled).toBe(true);
    await typeInto(1, '123456');
    expect(screen.getByLabelText('stepup-confirm').props.accessibilityState.disabled).toBe(false);
  });

  it.each(['pin', 'otp', '2fa'] as const)('%s: cancel resolves null and no confirm is possible', async (b) => {
    api.mockResolvedValue({ status: 'otp_sent' });
    await ask(b);
    await fireEvent.press(await screen.findByLabelText(b === 'pin' ? 'pin-cancel' : 'stepup-cancel'));
    await expect(result).resolves.toBeNull();
    expect(api).not.toHaveBeenCalledWith('/v1/transactions/confirm', expect.anything());
  });

  it('never writes a credential to the console', async () => {
    const spies = (['log', 'info', 'warn', 'error', 'debug'] as const).map((m) => jest.spyOn(console, m));
    await ask('2fa');
    await typeInto(0, '482913');
    await typeInto(1, '123456');
    await fireEvent.press(screen.getByLabelText('stepup-confirm'));
    await result;
    for (const s of spies) {
      expect(JSON.stringify(s.mock.calls)).not.toMatch(/482913|123456/);
      s.mockRestore();
    }
  });
});