import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react-native';
import { Linking } from 'react-native';

import { apiRequest } from '@/src/api/client';
import { ApiError } from '@/src/lib/errors';

import TotpEnrolScreen from '../totp-enroll';
import TotpStatusScreen from '../totp-status';

const mockBack = jest.fn();
jest.mock('expo-router', () => ({ router: { back: () => mockBack(), push: jest.fn() } }));
jest.mock('@/src/api/client', () => ({ apiRequest: jest.fn() }));
jest.mock('expo-clipboard', () => ({ setStringAsync: jest.fn().mockResolvedValue(true) }));
jest.mock('expo-screen-capture', () => ({
  preventScreenCaptureAsync: jest.fn().mockResolvedValue(undefined),
  allowScreenCaptureAsync: jest.fn().mockResolvedValue(undefined),
}));
jest.mock('react-native-safe-area-context', () => ({
  SafeAreaView: ({ children }: { children: React.ReactNode }) => children,
}));

const api = apiRequest as jest.Mock;
const CODES = Array.from({ length: 10 }, (_, i) => `rc-${i}aaaa-bbbb`);

function wrap(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false }, queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe('Test_TotpEnrol', () => {
  beforeEach(() => {
    api.mockReset();
    mockBack.mockReset();
    jest.spyOn(Linking, 'openURL').mockResolvedValue(true);
    api.mockImplementation(async (p: string) => {
      if (p === '/v1/me/totp/enroll')
        return { status: 'pending', secret: 'JBSWY3DPEHPK3PXP', otpauth_uri: 'otpauth://totp/HPay?secret=JBSWY3DPEHPK3PXP' };
      if (p === '/v1/me/totp/confirm') return { status: 'totp_enabled', recovery_codes: CODES };
      throw new Error('unexpected ' + p);
    });
  });

  async function toRecovery() {
    await wrap(<TotpEnrolScreen />);
    await fireEvent.changeText(screen.getByLabelText('Account password'), 'pw-secret');
    await fireEvent.press(screen.getByLabelText('Continue'));
    await screen.findByText('JBSW Y3DP EHPK 3PXP');
    await fireEvent.changeText(screen.getAllByLabelText('OTP digit 1')[0], '123456');
    await fireEvent.press(screen.getByLabelText('Turn on'));
    await screen.findByText('Save your recovery codes');
  }

  it('happy path: password, secret in groups of 4, link, code, recovery codes, gated Done', async () => {
    await wrap(<TotpEnrolScreen />);
    await fireEvent.changeText(screen.getByLabelText('Account password'), 'pw-secret');
    await fireEvent.press(screen.getByLabelText('Continue'));
    await screen.findByText('JBSW Y3DP EHPK 3PXP');
    expect(api.mock.calls[0][1].body).toStrictEqual({ password: 'pw-secret' });
    await fireEvent.press(screen.getByLabelText('Open in authenticator app'));
    expect(Linking.openURL).toHaveBeenCalledWith('otpauth://totp/HPay?secret=JBSWY3DPEHPK3PXP');
    await fireEvent.changeText(screen.getAllByLabelText('OTP digit 1')[0], '123456');
    await fireEvent.press(screen.getByLabelText('Turn on'));
    await screen.findByText('Save your recovery codes');
    expect(api.mock.calls[1][1].body).toStrictEqual({ code: '123456' });
    for (const c of CODES) expect(screen.getByText(c)).toBeTruthy();
    expect(screen.queryByText('JBSW Y3DP EHPK 3PXP')).toBeNull(); // secret dropped from state
  });

  it('Done stays disabled until "I saved these codes" is on', async () => {
    await toRecovery();
    expect(screen.getByLabelText('Done').props.accessibilityState.disabled).toBe(true);
    await fireEvent.press(screen.getByLabelText('Done'));
    expect(mockBack).not.toHaveBeenCalled();
    await fireEvent(screen.getByLabelText('I saved these codes'), 'valueChange', true);
    await waitFor(() => expect(screen.getByLabelText('Done').props.accessibilityState.disabled).toBe(false));
    await fireEvent.press(screen.getByLabelText('Done'));
    expect(mockBack).toHaveBeenCalledTimes(1);
  });

  it('double tap on Continue sends the enrol request once', async () => {
    await wrap(<TotpEnrolScreen />);
    await fireEvent.changeText(screen.getByLabelText('Account password'), 'pw-secret');
    fireEvent.press(screen.getByLabelText('Continue'));
    fireEvent.press(screen.getByLabelText('Continue'));
    await screen.findByText('JBSW Y3DP EHPK 3PXP');
    expect(api.mock.calls.filter((c) => c[0] === '/v1/me/totp/enroll')).toHaveLength(1);
  });

  it('a wrong first code shows the server copy and stays on the code step', async () => {
    await wrap(<TotpEnrolScreen />);
    await fireEvent.changeText(screen.getByLabelText('Account password'), 'pw-secret');
    await fireEvent.press(screen.getByLabelText('Continue'));
    await screen.findByText('JBSW Y3DP EHPK 3PXP');
    api.mockImplementation(async () => {
      throw new ApiError('ERR_TOTP_INVALID', 400);
    });
    await fireEvent.changeText(screen.getAllByLabelText('OTP digit 1')[0], '000000');
    await fireEvent.press(screen.getByLabelText('Turn on'));
    await screen.findByText('The authenticator code is incorrect or has already been used.');
  });

  it('no secret, password or recovery code reaches the console', async () => {
    const spies = (['log', 'info', 'warn', 'error'] as const).map((m) => jest.spyOn(console, m));
    await toRecovery();
    for (const s of spies) {
      const out = JSON.stringify(s.mock.calls);
      expect(out).not.toMatch(/pw-secret|JBSWY3DP|rc-0aaaa/);
    }
  });
});

describe('Test_TotpStatus', () => {
  it('shows active and codes left, warning at 3 or fewer', async () => {
    api.mockReset();
    api.mockResolvedValue({ enabled: true, recovery_codes_left: 3 });
    await wrap(<TotpStatusScreen />);
    await screen.findByText('On');
    expect(screen.getByText('Recovery codes left: 3')).toBeTruthy();
    expect(screen.getByText(/running low on recovery codes/)).toBeTruthy();
  });
  it('no warning with 4 left; Off offers enrolment', async () => {
    api.mockReset();
    api.mockResolvedValue({ enabled: true, recovery_codes_left: 4 });
    const a = await wrap(<TotpStatusScreen />);
    await screen.findByText('Recovery codes left: 4');
    expect(screen.queryByText(/running low/)).toBeNull();
    a.unmount();
    api.mockResolvedValue({ enabled: false, recovery_codes_left: 0 });
    await wrap(<TotpStatusScreen />);
    await screen.findByText('Off');
    expect(screen.getByLabelText('Turn on two-step verification')).toBeTruthy();
  });
});