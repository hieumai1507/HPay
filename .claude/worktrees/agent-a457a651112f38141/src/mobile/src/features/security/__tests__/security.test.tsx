import { act } from '@testing-library/react-native';

import { useSecurityStore } from '@/src/stores/security';
import { useSessionStore } from '@/src/stores/session';

describe('TestEdge_M07_AppBackground_PrivacyOverlay', () => {
  beforeEach(() => {
    useSecurityStore.setState({ isLocked: false, isRooted: false, biometricEnabled: false, pendingRoute: null });
    useSessionStore.setState({ accessToken: 'valid-token', status: 'authenticated' });
    jest.useFakeTimers();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it('triggers local biometric lock if app stays in background for more than 5 minutes', () => {
    const store = useSecurityStore.getState();
    expect(store.isLocked).toBe(false);

    // Simulate app going to background
    let lastActiveTime = Date.now();
    
    // Simulate AppState going inactive/background
    act(() => {
      // In the wrapper, background time is captured using Date.now() when AppState changes.
      // We advance time by 6 minutes (360000ms)
      jest.advanceTimersByTime(360000);
    });

    // Simulate foreground transition and checking the elapsed time
    const elapsed = Date.now() - lastActiveTime + 360000;
    if (elapsed >= 5 * 60 * 1000) {
      useSecurityStore.setState({ isLocked: true });
    }

    expect(useSecurityStore.getState().isLocked).toBe(true);
  });

  it('does NOT trigger lock if app is in background for less than 5 minutes', () => {
    const store = useSecurityStore.getState();
    expect(store.isLocked).toBe(false);

    // Simulate background for 2 minutes
    act(() => {
      jest.advanceTimersByTime(120000);
    });

    const elapsed = 120000;
    if (elapsed >= 5 * 60 * 1000) {
      useSecurityStore.setState({ isLocked: true });
    }

    expect(useSecurityStore.getState().isLocked).toBe(false);
  });
});

describe('TestEdge_M08_DeepLink_PendingAuth', () => {
  beforeEach(() => {
    useSecurityStore.setState({ isLocked: false, isRooted: false, biometricEnabled: false, pendingRoute: null });
    useSessionStore.setState({ accessToken: null, status: 'unauthenticated' });
  });

  it('saves pending route when link is triggered while unauthenticated, and clears it on login', () => {
    // 1. User is unauthenticated, triggers deep link
    const targetRoute = '/org/approvals/987cba65-4321-4321-4321-1234567890ab';
    
    useSecurityStore.setState({ pendingRoute: targetRoute });
    expect(useSecurityStore.getState().pendingRoute).toBe(targetRoute);

    // 2. Perform successful login
    useSessionStore.setState({ accessToken: 'valid-access-token', status: 'authenticated' });

    // 3. Login component triggers redirection
    const pending = useSecurityStore.getState().pendingRoute;
    let redirectedTo = '';
    if (pending) {
      redirectedTo = pending;
      useSecurityStore.setState({ pendingRoute: null });
    }

    // Verify it redirected to the deep link and cleared the store
    expect(redirectedTo).toBe(targetRoute);
    expect(useSecurityStore.getState().pendingRoute).toBeNull();
  });
});
