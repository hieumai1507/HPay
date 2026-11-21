import type { ExpoConfig } from 'expo/config';

// APP_ENV switches the API base URL. dev (default) uses EXPO_PUBLIC_API_URL when set and
// otherwise the local backend on this machine; staging/demo are placeholders until those
// environments exist. A physical device cannot reach "localhost": set EXPO_PUBLIC_API_URL to
// the LAN address of the dev machine, e.g. EXPO_PUBLIC_API_URL=http://192.168.1.10:8080.
const APP_ENV = process.env.APP_ENV ?? 'dev';

const API_BASE_URL: Record<string, string> = {
  dev: process.env.EXPO_PUBLIC_API_URL || 'http://localhost:8080',
  staging: 'https://staging-api.HPay.example.com',
  demo: 'https://demo-api.HPay.example.com',
};

const config: ExpoConfig = {
  name: 'HPay',
  slug: 'hpay-mobile',
  scheme: 'hpay',
  version: '1.0.0',
  orientation: 'portrait',
  icon: './assets/icon.png',
  userInterfaceStyle: 'light',
  ios: {
    bundleIdentifier: 'com.hieumai1507.hpay',
    supportsTablet: true,
  },
  android: {
    package: 'com.hieumai1507.hpay',
    adaptiveIcon: {
      backgroundColor: '#E6F4FE',
      foregroundImage: './assets/android-icon-foreground.png',
      backgroundImage: './assets/android-icon-background.png',
      monochromeImage: './assets/android-icon-monochrome.png',
    },
    predictiveBackGestureEnabled: false,
  },
  plugins: ['expo-router', 'expo-secure-store', 'expo-splash-screen'],
  extra: {
    appEnv: APP_ENV,
    apiBaseUrl: API_BASE_URL[APP_ENV] ?? API_BASE_URL.dev,
  },
};

export default config;
