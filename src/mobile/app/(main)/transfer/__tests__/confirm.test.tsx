import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react-native';
import { Alert } from 'react-native';

import { apiRequest } from '@/src/api/client';
import { ApiError } from '@/src/lib/errors';

import TransferConfirmScreen from '../confirm';

const mockReplace = jest.fn();
const mockPush = jest.fn();
jest.mock('expo-router', () => ({
  router: { replace: (...a: unknown[]) => mockReplace(...a), push: (...a: unknown[]) => mockPush(...a), back: jest.fn() },
  useLocalSearchParams: () => ({ phone: '0912345678', maskedName: 'A B', amount: '100000', note: '' }),
}));
jest.mock('@/src/api/client', () => ({ apiRequest: jest.fn() }));
jest.mock('@/src/features/wallet/recent-contacts', () => ({
  useRecentContacts: () => ({ contacts: [{ phone: '0912345678' }], add: jest.fn() }),
}));
jest.mock('@/src/lib/intentKey', () => ({
  withIntentKey: (_i: string, run: (k: string) => Promise<unknown>) => run('key-1'),
}));
jest.mock('react-native-safe-area-context', () => ({
  SafeAreaView: ({ children }: { children: React.ReactNode }) => children,
}));

jest.setTimeout(15000);
// onPress returns the submit promise, which stays pending while a prompt is open; never await it.
const pressTransfer = () => {
  void fireEvent.press(screen.getByText('Transfer'));
};
const api = apiRequest as jest.Mock;
let confirmBody: Record<string, unknown> | null;
let requestAuth: string;
let confirmError: ApiError | null;

function setup() {
  confirmBody = null;
  confirmError = null;
  api.mockReset();
  api.mockImplementation(async (path: string, opts?: { body?: Record<string, unknown> }) => {
    if (path.startsWith('/v1/fees')) return { fee: 1000, total: 101000 };
    if (path === '/v1/transactions/request')
      return { transaction_id: 'tx-1', ref_code: 'R1', auth_required: requestAuth };
    if (path === '/v1/transactions/challenge') return { status: 'otp_sent' };
    if (path === '/v1/transactions/confirm') {
      confirmBody = opts?.body ?? null;
      if (confirmError) throw confirmError;
      return { ref_code: 'R1' };
    }
    throw new Error('unexpected ' + path);
  });
}

function renderScreen() {
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false }, queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <TransferConfirmScreen />
    </QueryClientProvider>,
  );
}
const typeInto = (i: number, v: string) => fireEvent.changeText(screen.getAllByLabelText('OTP digit 1')[i], v);
const calls = (path: string) => api.mock.calls.filter((c) => c[0] === path).length;
describe('Test_TransferConfirm_StepUp', () => {
  beforeEach(() => {
    mockReplace.mockReset();
    mockPush.mockReset();
    jest.spyOn(Alert, 'alert').mockImplementation(() => {});
    setup();
  });

  async function start() {
    await renderScreen();
    await waitFor(() => expect(screen.queryByText('Calculating…')).toBeNull());
    pressTransfer();
  }
  it('pin band: request then confirm with exactly the pin', async () => {
    requestAuth = 'pin';
    await start();
    await screen.findByLabelText('pin-confirm');
    await typeInto(0, '482913');
    await fireEvent.press(screen.getByLabelText('pin-confirm'));
    await waitFor(() => expect(mockReplace).toHaveBeenCalled());
    expect(confirmBody).toStrictEqual({ transaction_id: 'tx-1', pin: '482913' });
    expect(calls('/v1/transactions/request')).toBe(1);
  });

  it('otp band: challenge, then confirm with pin and otp only', async () => {
    requestAuth = 'otp';
    await start();
    await screen.findByLabelText('stepup-confirm');
    expect(calls('/v1/transactions/challenge')).toBe(1);
    await typeInto(0, '482913');
    await typeInto(1, '123456');
    await fireEvent.press(screen.getByLabelText('stepup-confirm'));
    await waitFor(() => expect(mockReplace).toHaveBeenCalled());
    expect(confirmBody).toStrictEqual({ transaction_id: 'tx-1', pin: '482913', otp: '123456' });
  });

  it('2fa band: confirm with pin and totp only, no challenge', async () => {
    requestAuth = '2fa';
    await start();
    await screen.findByLabelText('stepup-confirm');
    await typeInto(0, '482913');
    await typeInto(1, '123456');
    await fireEvent.press(screen.getByLabelText('stepup-confirm'));
    await waitFor(() => expect(mockReplace).toHaveBeenCalled());
    expect(confirmBody).toStrictEqual({ transaction_id: 'tx-1', pin: '482913', totp: '123456' });
    expect(calls('/v1/transactions/challenge')).toBe(0);
  });

  it('cancelling the prompt sends no confirm', async () => {
    requestAuth = '2fa';
    await start();
    await fireEvent.press(await screen.findByLabelText('stepup-cancel'));
    await new Promise((r) => setTimeout(r, 20));
    expect(confirmBody).toBeNull();
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it('a wrong code re-asks for the same held transaction: one request only', async () => {
    requestAuth = '2fa';
    await start();
    confirmError = new ApiError('ERR_TOTP_INVALID', 400);
    await screen.findByLabelText('stepup-confirm');
    await typeInto(0, '482913');
    await typeInto(1, '000000');
    await fireEvent.press(screen.getByLabelText('stepup-confirm'));
    await screen.findByText('The authenticator code is incorrect or has already been used.');
    expect(calls('/v1/transactions/request')).toBe(1);
    confirmError = null;
    await typeInto(0, '482913');
    await typeInto(1, '123456');
    await fireEvent.press(screen.getByLabelText('stepup-confirm'));
    await waitFor(() => expect(mockReplace).toHaveBeenCalled());
    expect(calls('/v1/transactions/request')).toBe(1);
    expect(calls('/v1/transactions/confirm')).toBe(2);
  });

  it('ERR_AUTH_EXPIRED goes back to the amount screen', async () => {
    requestAuth = 'pin';
    await start();
    confirmError = new ApiError('ERR_AUTH_EXPIRED', 410);
    await screen.findByLabelText('pin-confirm');
    await typeInto(0, '482913');
    await fireEvent.press(screen.getByLabelText('pin-confirm'));
    await waitFor(() => expect(mockReplace).toHaveBeenCalledWith('/(main)/transfer/amount'));
    expect(Alert.alert).toHaveBeenCalledWith('Payment expired', 'This payment expired. Start again.');
  });

  it.each([
    ['ERR_LIMIT_DAILY', 'You have reached your daily transaction limit.'],
    ['ERR_LIMIT_MONTHLY', 'You have reached your monthly transaction limit.'],
    ['ERR_LIMIT_PER_TXN', 'The amount exceeds the limit for a single transaction.'],
    ['ERR_TOO_FREQUENT', 'You are doing that too quickly. Please try again in a few minutes.'],
    ['ERR_PIN_LOCKED', 'Your PIN is locked for 15 minutes after too many incorrect attempts.'],
  ])('%s stays on the screen, shows the message and blocks retry', async (code, msg) => {
    requestAuth = 'pin';
    await start();
    confirmError = new ApiError(code, 429);
    await screen.findByLabelText('pin-confirm');
    await typeInto(0, '482913');
    await fireEvent.press(screen.getByLabelText('pin-confirm'));
    await screen.findByText(msg);
    expect(mockReplace).not.toHaveBeenCalled();
    await waitFor(() => expect(screen.getByRole('button',{name:'Transfer'}).props.accessibilityState.disabled).toBe(true));
  });

  it('ERR_PIN_NOT_SET offers Set PIN', async () => {
    requestAuth = 'pin';
    await start();
    confirmError = new ApiError('ERR_PIN_NOT_SET', 403);
    await screen.findByLabelText('pin-confirm');
    await typeInto(0, '482913');
    await fireEvent.press(screen.getByLabelText('pin-confirm'));
    await fireEvent.press(await screen.findByLabelText('Set PIN'));
    expect(mockPush).toHaveBeenCalledWith('/(main)/security/set-pin');
  });

  it('ERR_TOTP_NOT_ENROLLED offers two-step verification', async () => {
    requestAuth = '2fa';
    await start();
    confirmError = new ApiError('ERR_TOTP_NOT_ENROLLED', 403);
    await screen.findByLabelText('stepup-confirm');
    await typeInto(0, '482913');
    await typeInto(1, '123456');
    await fireEvent.press(screen.getByLabelText('stepup-confirm'));
    await fireEvent.press(await screen.findByLabelText('Turn on two-step verification'));
    expect(mockPush).toHaveBeenCalledWith('/(main)/security/totp-enroll');
  });

  it('double tap sends one request', async () => {
    requestAuth = 'pin';
    await start();
    pressTransfer();
    pressTransfer();
    await new Promise((r) => setTimeout(r, 20));
    expect(calls('/v1/transactions/request')).toBe(1);
  });

  it('no credential reaches the console', async () => {
    const spies = (['log', 'info', 'warn', 'error'] as const).map((m) => jest.spyOn(console, m));
    requestAuth = '2fa';
    await start();
    await screen.findByLabelText('stepup-confirm');
    await typeInto(0, '482913');
    await typeInto(1, '123456');
    await fireEvent.press(screen.getByLabelText('stepup-confirm'));
    await waitFor(() => expect(mockReplace).toHaveBeenCalled());
    for (const s of spies) expect(JSON.stringify(s.mock.calls)).not.toMatch(/482913|123456/);
    expect(JSON.stringify(mockReplace.mock.calls)).not.toMatch(/482913|123456/);
  });
});