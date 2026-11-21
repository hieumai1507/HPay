import type { ExpoConfig } from 'expo/config';

// APP_ENV switches the API base URL: dev (default) -> local backend on the
// same machine; staging/demo are placeholders until those environments exist.
const APP_ENV = process.env.APP_ENV ?? 'dev';

const API_BASE_URL: Record<string, string> = {
  // dev: 'http://localhost:8080',
  dev: 'https://hpay.demaciahieu.store',
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
    supportsTablet: true,
  },
  android: {
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
