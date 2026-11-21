import config from '../app.config';

describe('Test_AppConfig_ApiUrlAndIdentifiers', () => {
  const original = { ...process.env };
  afterEach(() => {
    process.env = { ...original };
    jest.resetModules();
  });

  function load(env: Record<string, string | undefined>) {
    process.env = { ...original, ...env };
    delete process.env.EXPO_PUBLIC_API_URL;
    if (env.EXPO_PUBLIC_API_URL) process.env.EXPO_PUBLIC_API_URL = env.EXPO_PUBLIC_API_URL;
    let loaded: typeof config = config;
    jest.isolateModules(() => {
      // eslint-disable-next-line @typescript-eslint/no-require-imports
      loaded = require('../app.config').default;
    });
    return loaded as unknown as { extra: { apiBaseUrl: string }; ios: { bundleIdentifier?: string }; android: { package?: string } };
  }

  it('dev defaults to localhost, never a remote host', () => {
    const c = load({ APP_ENV: 'dev' });
    expect(c.extra.apiBaseUrl).toBe('http://localhost:8080');
  });

  it('dev honours EXPO_PUBLIC_API_URL', () => {
    const c = load({ APP_ENV: 'dev', EXPO_PUBLIC_API_URL: 'http://192.168.1.10:8080' });
    expect(c.extra.apiBaseUrl).toBe('http://192.168.1.10:8080');
  });

  it('declares ios bundleIdentifier and android package', () => {
    const c = load({ APP_ENV: 'dev' });
    expect(c.ios.bundleIdentifier).toBe('com.hieumai1507.hpay');
    expect(c.android.package).toBe('com.hieumai1507.hpay');
  });
});