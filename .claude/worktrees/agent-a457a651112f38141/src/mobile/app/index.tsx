import { Redirect } from 'expo-router';

import { useSessionStore } from '@/src/stores/session';

/**
 * Splash gate: by the time this mounts, RootLayout has already resolved
 * useSessionBootstrap() (silent refresh attempted), so `status` is settled.
 */
export default function Index() {
  const status = useSessionStore((s) => s.status);

  if (status === 'authenticated') {
    const user = useSessionStore.getState().user;
    const kycSkipped = useSessionStore.getState().kycSkippedInSession;

    if (user?.kyc_status === 'none' && !kycSkipped) {
      return <Redirect href="/(kyc)/intro" />;
    }
    return <Redirect href="/(main)/home" />;
  }
  return <Redirect href="/(auth)/onboarding" />;
}

