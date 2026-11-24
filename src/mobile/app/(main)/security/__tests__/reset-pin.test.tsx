import { act, fireEvent, screen, waitFor } from '@testing-library/react-native';

import { renderWithClient, typeDigits, deferred, pressAsync } from '@/src/test-utils/security';
import { ApiError } from '@/src/lib/errors';

import ResetPinScreen from '../reset-pin';

const mockBack = jest.fn();
const mockReplace = jest.fn();
jest.mock('expo-router', () => ({
  useRouter: () => ({ back: mockBack, push: jest.fn(), replace: mockReplace }),
}));
const mockApi = jest.fn();
jest.mock('@/src/api/client', () => ({ apiRequest: (...a: unknown[]) => mockApi(...a) }));

const NEW_PIN = '482913';
const CODE = '654987';

async function requestCode(result: unknown = { status: 'code_sent', expires_in: 300 }) {
  mockApi.mockResolvedValueOnce(result);
  fireEvent.changeText(screen.getByLabelText('Password'), 'Passw0rd!');
  await pressAsync(screen.getByLabelText('Send code'));
  await waitFor(() => expect(screen.getByText('Enter the code')).toBeTruthy());
}
function fillStep2(code = CODE, pin = NEW_PIN, repeat = pin) {
  typeDigits('Code digit', code);
  typeDigits('New PIN digit', pin);
  typeDigits('Repeat PIN digit', repeat);
}

describe('Reset PIN screen', () => {
  beforeEach(() => {
    mockApi.mockReset();
    mockBack.mockReset();
    mockReplace.mockReset();
  });
  afterEach(() => {
    jest.useRealTimers();
  });

  it('happy path: request then confirm, then explains the 24 hour rule and goes back', async () => {
    renderWithClient(<ResetPinScreen />);
    await requestCode();
    expect(mockApi).toHaveBeenNthCalledWith(1, '/v1/me/pin/reset/request', {
      method: 'POST',
      body: { password: 'Passw0rd!' },
    });
    mockApi.mockResolvedValueOnce({ status: 'pin_reset' });
    fillStep2();
    await pressAsync(screen.getByLabelText('Reset PIN'));
    await waitFor(() => expect(screen.getByText(/limited to \$100\.00/)).toBeTruthy());
    expect(mockApi).toHaveBeenNthCalledWith(2, '/v1/me/pin/reset/confirm', {
      method: 'POST',
      body: { code: CODE, new_pin: NEW_PIN },
    });
    fireEvent.press(screen.getByLabelText('Done'));
    expect(mockBack).toHaveBeenCalled();
  });

  it('shows the dev code only when the API returns otp_dev', async () => {
    renderWithClient(<ResetPinScreen />);
    await requestCode({ status: 'code_sent', expires_in: 300, otp_dev: '111222' });
    expect(screen.getByText(/Dev code: 111222/)).toBeTruthy();
  });

  it('never shows a dev code when the API does not return one', async () => {
    renderWithClient(<ResetPinScreen />);
    await requestCode();
    expect(screen.queryByText(/Dev code/)).toBeNull();
  });

  it('step 1 errors: wrong password, rate limit, delivery', async () => {
    renderWithClient(<ResetPinScreen />);
    fireEvent.changeText(screen.getByLabelText('Password'), 'x');
    mockApi.mockRejectedValueOnce(new ApiError('ERR_INVALID_CREDENTIALS', 401));
    await pressAsync(screen.getByLabelText('Send code'));
    await waitFor(() => expect(screen.getByText(/Incorrect phone number or password/)).toBeTruthy());
    mockApi.mockRejectedValueOnce(new ApiError('ERR_PIN_RESET_RATE_LIMITED', 429));
    await pressAsync(screen.getByLabelText('Send code'));
    await waitFor(() => expect(screen.getByText(/too many PIN reset codes/)).toBeTruthy());
  });

  it('ERR_PIN_NOT_SET offers Set PIN', async () => {
    renderWithClient(<ResetPinScreen />);
    fireEvent.changeText(screen.getByLabelText('Password'), 'x');
    mockApi.mockRejectedValueOnce(new ApiError('ERR_PIN_NOT_SET', 403));
    await pressAsync(screen.getByLabelText('Send code'));
    await waitFor(() => expect(screen.getByLabelText('Set PIN')).toBeTruthy());
    fireEvent.press(screen.getByLabelText('Set PIN'));
    expect(mockReplace).toHaveBeenCalledWith('/(main)/security/set-pin');
  });

  it('ERR_OTP_COOLDOWN starts a visible countdown that ends after 30 s', async () => {
    jest.useFakeTimers();
    renderWithClient(<ResetPinScreen />);
    fireEvent.changeText(screen.getByLabelText('Password'), 'x');
    mockApi.mockRejectedValueOnce(new ApiError('ERR_OTP_COOLDOWN', 429));
    await pressAsync(screen.getByLabelText('Send code'));
    await waitFor(() => expect(screen.getByText(/Please wait before requesting another/)).toBeTruthy());
    expect(screen.getByText(/another code in 30s/)).toBeTruthy();
    expect(screen.getByLabelText('Send code').props.accessibilityState?.disabled).toBe(true);
    await act(async () => {
      jest.advanceTimersByTime(10_000);
    });
    expect(screen.getByText(/another code in 20s/)).toBeTruthy();
    await act(async () => {
      jest.advanceTimersByTime(20_000);
    });
    expect(screen.queryByText(/another code in/)).toBeNull();
    expect(screen.getByLabelText('Send code').props.accessibilityState?.disabled).toBe(false);
  });

  it('after a code is sent, the resend cooldown counts down from 30', async () => {
    jest.useFakeTimers();
    renderWithClient(<ResetPinScreen />);
    await requestCode();
    expect(screen.getByText(/another code in 30s/)).toBeTruthy();
    await act(async () => {
      jest.advanceTimersByTime(5_000);
    });
    expect(screen.getByText(/another code in 25s/)).toBeTruthy();
  });

  it('wrong code: shows the error, clears the code, lets the user retry', async () => {
    renderWithClient(<ResetPinScreen />);
    await requestCode();
    fillStep2();
    mockApi.mockRejectedValueOnce(new ApiError('ERR_OTP_INVALID', 400));
    await pressAsync(screen.getByLabelText('Reset PIN'));
    await waitFor(() => expect(screen.getByText(/Incorrect OTP code/)).toBeTruthy());
    expect(screen.getByLabelText('Code digit 1').props.value).toBe('');
    expect(screen.queryByLabelText('Request a new code')).toBeTruthy();
  });

  it.each([
    ['ERR_OTP_MAX_ATTEMPTS', /Too many incorrect OTP attempts/],
    ['ERR_OTP_EXPIRED', /has expired/],
  ])('%s ends the attempt and offers a new code', async (code, pattern) => {
    renderWithClient(<ResetPinScreen />);
    await requestCode();
    fillStep2();
    mockApi.mockRejectedValueOnce(new ApiError(code, 400));
    await pressAsync(screen.getByLabelText('Reset PIN'));
    await waitFor(() => expect(screen.getByText(pattern)).toBeTruthy());
    expect(screen.queryByLabelText('Reset PIN')).toBeNull();
    fireEvent.press(screen.getByLabelText('Request a new code'));
    expect(screen.getByText('Forgot PIN?')).toBeTruthy();
  });

  it('weak new PIN is blocked client-side; a server ERR_PIN_WEAK is still shown', async () => {
    renderWithClient(<ResetPinScreen />);
    await requestCode();
    fillStep2(CODE, '123456');
    expect(screen.getByText(/run of digits/)).toBeTruthy();
    fireEvent.press(screen.getByLabelText('Reset PIN'));
    expect(mockApi).toHaveBeenCalledTimes(1); // only the request call

    typeDigits('New PIN digit', '135791');
    typeDigits('Repeat PIN digit', '135791');
    mockApi.mockRejectedValueOnce(new ApiError('ERR_PIN_WEAK', 400));
    await pressAsync(screen.getByLabelText('Reset PIN'));
    await waitFor(() => expect(screen.getByText(/too easy to guess/)).toBeTruthy());
  });

  it('double tap on Send code and on Reset PIN each send once', async () => {
    renderWithClient(<ResetPinScreen />);
    const d1 = deferred<unknown>();
    mockApi.mockReturnValueOnce(d1.promise);
    fireEvent.changeText(screen.getByLabelText('Password'), 'x');
    fireEvent.press(screen.getByLabelText('Send code'));
    fireEvent.press(screen.getByLabelText('Send code'));
    await waitFor(() => expect(mockApi).toHaveBeenCalledTimes(1));
    await act(async () => {
      d1.resolve({ status: 'code_sent', expires_in: 300 });
    });
    await waitFor(() => expect(screen.getByText('Enter the code')).toBeTruthy());

    const d2 = deferred<unknown>();
    mockApi.mockReturnValueOnce(d2.promise);
    fillStep2();
    fireEvent.press(screen.getByLabelText('Reset PIN'));
    fireEvent.press(screen.getByLabelText('Reset PIN'));
    await waitFor(() => expect(mockApi).toHaveBeenCalledTimes(2));
    await act(async () => {
      d2.resolve({ status: 'pin_reset' });
    });
    expect(mockApi).toHaveBeenCalledTimes(2);
  });

  it('cancel sends nothing', () => {
    renderWithClient(<ResetPinScreen />);
    fireEvent.changeText(screen.getByLabelText('Password'), 'x');
    fireEvent.press(screen.getByLabelText('Cancel'));
    expect(mockBack).toHaveBeenCalled();
    expect(mockApi).not.toHaveBeenCalled();
  });

  it('never leaks PIN, code or password into logs', async () => {
    const spies = ['log', 'warn', 'error', 'info'].map((m) =>
      jest.spyOn(console, m as 'log'),
    );
    renderWithClient(<ResetPinScreen />);
    await requestCode();
    mockApi.mockResolvedValueOnce({ status: 'pin_reset' });
    fillStep2();
    await pressAsync(screen.getByLabelText('Reset PIN'));
    await waitFor(() => expect(screen.getByText('PIN reset')).toBeTruthy());
    const logged = JSON.stringify(spies.flatMap((s) => s.mock.calls));
    for (const secret of [NEW_PIN, CODE, 'Passw0rd!']) expect(logged).not.toContain(secret);
    spies.forEach((s) => s.mockRestore());
  });
});