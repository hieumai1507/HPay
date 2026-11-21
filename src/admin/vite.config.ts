import path from 'node:path';

import react from '@vitejs/plugin-react';
import { defineConfig, loadEnv, type Plugin } from 'vite';

/**
 * Content-Security-Policy for the PRODUCTION build only. The dev server needs
 * inline scripts (React Fast Refresh preamble) and a websocket (HMR), so a
 * strict policy would break it. The built index.html has one external module
 * script and one external stylesheet, no inline script, so script-src 'self'
 * holds. style-src needs 'unsafe-inline' because React renders style="..."
 * attributes. connect-src adds the origin of VITE_API_URL when it is set.
 * Note: frame-ancestors is ignored in a <meta> CSP; set it as a response header
 * on the static host.
 */
function cspPlugin(apiUrl: string | undefined): Plugin {
  let apiOrigin = '';
  try {
    if (apiUrl) apiOrigin = new URL(apiUrl).origin;
  } catch {
    apiOrigin = '';
  }
  const policy = [
    "default-src 'self'",
    "script-src 'self'",
    "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
    "font-src 'self' https://fonts.gstatic.com data:",
    "img-src 'self' data: blob: https:",
    `connect-src 'self'${apiOrigin ? ' ' + apiOrigin : ''}`,
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
  ].join('; ');
  return {
    name: 'hpay-admin-csp',
    apply: 'build',
    transformIndexHtml() {
      return [
        { tag: 'meta', attrs: { 'http-equiv': 'Content-Security-Policy', content: policy }, injectTo: 'head-prepend' },
      ];
    },
  };
}

export default defineConfig(({ mode }) => ({
  plugins: [react(), cspPlugin(loadEnv(mode, process.cwd(), '').VITE_API_URL)],
  resolve: {
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  server: {
    port: 5173,
    proxy: {
      // Dev only: backend runs on :8080 (go run ./cmd/server --role=all)
      '/v1': { target: 'http://localhost:8080', changeOrigin: true },
      '/healthz': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
  preview: {
    port: 4173,
    proxy: {
      // Same proxy as dev, but for `vite preview` — used by the Playwright
      // E2E suite (e2e/) so the built app can reach the real backend on :8080
      // without baking a base URL into the production bundle.
      '/v1': { target: 'http://localhost:8080', changeOrigin: true },
      '/healthz': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
}));
