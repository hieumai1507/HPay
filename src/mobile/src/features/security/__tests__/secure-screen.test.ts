import { renderHook } from '@testing-library/react-native';
import * as ScreenCapture from 'expo-screen-capture';

import { useSecureScreen } from '../useSecureScreen';

jest.mock('expo-screen-capture', () => ({
  preventScreenCaptureAsync: jest.fn(() => Promise.resolve()),
  allowScreenCaptureAsync: jest.fn(() => Promise.resolve()),
}));

const prevent = ScreenCapture.preventScreenCaptureAsync as jest.Mock;
const allow = ScreenCapture.allowScreenCaptureAsync as jest.Mock;

beforeEach(() => {
  prevent.mockClear();
  allow.mockClear();
});

describe('useSecureScreen (M07)', () => {
  it('blocks capture while mounted and releases it on unmount', () => {
    const { unmount } = renderHook(() => useSecureScreen());
    expect(prevent).toHaveBeenCalledTimes(1);
    expect(allow).not.toHaveBeenCalled();

    unmount();
    expect(allow).toHaveBeenCalledTimes(1);
  });

  it('does nothing when disabled', () => {
    renderHook(() => useSecureScreen(false));
    expect(prevent).not.toHaveBeenCalled();
  });

  // Expo Go has no native module: a rejection must not escape and take the
  // screen down with it.
  it('survives an unsupported platform', () => {
    prevent.mockRejectedValueOnce(new Error('not available in Expo Go'));
    expect(() => renderHook(() => useSecureScreen())).not.toThrow();
  });
});
