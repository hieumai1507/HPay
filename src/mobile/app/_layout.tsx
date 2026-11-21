import '../global.css';

import { QueryClientProvider } from '@tanstack/react-query';
import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { StatusBar } from 'expo-status-bar';
import { useEffect } from 'react';

import { useSessionBootstrap } from '@/src/api/refresh';
import { queryClient } from '@/src/api/query-client';
import { SecurityWrapper } from '@/src/features/security/components/SecurityWrapper';

SplashScreen.preventAutoHideAsync().catch(() => {
  // no-op — already hidden or unsupported (web)
});

export default function RootLayout() {
  const isBootstrapping = useSessionBootstrap();

  useEffect(() => {
    if (!isBootstrapping) {
      SplashScreen.hideAsync().catch(() => {});
    }
  }, [isBootstrapping]);

  if (isBootstrapping) {
    return null;
  }

  return (
    <QueryClientProvider client={queryClient}>
      <StatusBar style="auto" />
      <SecurityWrapper>
        <Stack screenOptions={{ headerShown: false }} />
      </SecurityWrapper>
    </QueryClientProvider>
  );
}
