import { act, fireEvent, screen, waitFor } from '@testing-library/react-native';

import { renderWithClient, typeDigits, deferred, pressAsync } from '@/src/test-utils/security';
import { ApiError } from '@/src/lib/errors';

import SetPinScreen from '../set-pin';

const mockBack = jest.fn();
jest.mock('expo-router', () => ({
  useRouter: () => ({ back: mockBack, push: jest.fn(), replace: jest.fn() }),
}));
const mockApi = jest.fn();
jest.mock('@/src/api/client', () => ({ apiRequest: (...a: unknown[]) => mockApi(...a) }));

const STRONG = '482913';

function fill(pin = STRONG, repeat = pin, password = 'Passw0rd!') {
  fireEvent.changeText(screen.getByLabelText('Password'), password);
  typeDigits('New PIN digit', pin);
  typeDigits('Repeat PIN digit', repeat);
}
const submit = () => screen.getByLabelText('Set PIN');

describe('Set PIN screen', () => {
  beforeEach(() => {
    mockApi.mockReset();
    mockBack.mockReset();
  });

  it('happy path: sends PUT /v1/me/pin once with password and new_pin, then confirms', async () => {
    mockApi.mockResolvedValue({ status: 'pin_set' });
    renderWithClient(<SetPinScreen />);
    fill();
    await pressAsync(submit());
    await waitFor(() => expect(screen.getByText('PIN set')).toBeTruthy());
    expect(mockApi).toHaveBeenCalledTimes(1);
    expect(mockApi).toHaveBeenCalledWith('/v1/me/pin', {
      method: 'PUT',
      body: { password: 'Passw0rd!', new_pin: STRONG },
    });
    fireEvent.press(screen.getByLabelText('Done'));
    expect(mockBack).toHaveBeenCalled();
  });

  it('blocks a weak PIN on the client and shows a hint as soon as 6 digits are typed', () => {
    renderWithClient(<SetPinScreen />);
    fill('123456');
    expect(screen.getByText(/run of digits/)).toBeTruthy();
    expect(submit().props.accessibilityState?.disabled).toBe(true);
    fireEvent.press(submit());
    expect(mockApi).not.toHaveBeenCalled();
  });

  it('blocks all-same-digit PIN', () => {
    renderWithClient(<SetPinScreen />);
    fill('777777');
    expect(screen.getByText(/same digit/)).toBeTruthy();
    expect(mockApi).not.toHaveBeenCalled();
  });

  it('blocks when the repeat differs', () => {
    renderWithClient(<SetPinScreen />);
    fill(STRONG, '482914');
    expect(screen.getByText('The two PINs do not match.')).toBeTruthy();
    fireEvent.press(submit());
    expect(mockApi).not.toHaveBeenCalled();
  });

  it('needs a password', () => {
    renderWithClient(<SetPinScreen />);
    fill(STRONG, STRONG, '');
    expect(submit().props.accessibilityState?.disabled).toBe(true);
  });

  it.each([
    ['ERR_PIN_WEAK', /too easy to guess/],
    ['ERR_INVALID_CREDENTIALS', /Incorrect phone number or password/],
    ['ERR_RATE_LIMITED', /too quickly/],
    ['ERR_VALIDATION', /Invalid data/],
  ])('shows the server error %s through describeError', async (code, pattern) => {
    mockApi.mockRejectedValue(new ApiError(code, 400));
    renderWithClient(<SetPinScreen />);
    fill();
    await pressAsync(submit());
    await waitFor(() => expect(screen.getByText(pattern)).toBeTruthy());
  });

  it('shows ERR_PIN_WEAK from the server even for a PIN the client accepted', async () => {
    mockApi.mockRejectedValue(new ApiError('ERR_PIN_WEAK', 400));
    renderWithClient(<SetPinScreen />);
    fill('135791');
    await pressAsync(submit());
    await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());
    expect(screen.getByText(/too easy to guess/)).toBeTruthy();
  });

  it('a double tap sends one request', async () => {
    const d = deferred<{ status: string }>();
    mockApi.mockReturnValue(d.promise);
    renderWithClient(<SetPinScreen />);
    fill();
    fireEvent.press(submit());
    fireEvent.press(submit());
    await waitFor(() => expect(mockApi).toHaveBeenCalledTimes(1));
    await act(async () => {
      d.resolve({ status: 'pin_set' });
    });
    expect(mockApi).toHaveBeenCalledTimes(1);
  });

  it('cancel goes back and sends nothing', () => {
    renderWithClient(<SetPinScreen />);
    fill();
    fireEvent.press(screen.getByLabelText('Cancel'));
    expect(mockBack).toHaveBeenCalled();
    expect(mockApi).not.toHaveBeenCalled();
  });

  it('never leaks the PIN or password into logs, storage or the screen after success', async () => {
    const spies = [
      jest.spyOn(console, 'log'),
      jest.spyOn(console, 'warn'),
      jest.spyOn(console, 'error'),
      jest.spyOn(console, 'info'),
    ];
    mockApi.mockResolvedValue({ status: 'pin_set' });
    renderWithClient(<SetPinScreen />);
    fill();
    await pressAsync(submit());
    await waitFor(() => expect(screen.getByText('PIN set')).toBeTruthy());
    const logged = JSON.stringify(spies.flatMap((s) => s.mock.calls));
    expect(logged).not.toContain(STRONG);
    expect(logged).not.toContain('Passw0rd!');
    spies.forEach((s) => s.mockRestore());
  });
});