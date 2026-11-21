import path from 'node:path';

import { defineConfig } from 'vitest/config';

// Unit tests only (src/**/*.test.ts[x]). Playwright specs live in e2e/ and are
// run by `npm run test:e2e`; vitest must never pick them up.
export default defineConfig({
  resolve: { alias: { '@': path.resolve(__dirname, 'src') } },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
    restoreMocks: true,
  },
});