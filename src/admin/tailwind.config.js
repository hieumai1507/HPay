/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          DEFAULT: '#0e90a3',
          600: '#0b7181',
          50: '#e6fafd',
        },
        accent: '#1ecbe1',
        sidebar: {
          bg: '#0b1220',
          text: '#94a3b8',
          active: '#1ecbe1',
          hover: '#1e293b',
        },
        main: {
          bg: '#f9fafb',
        },
        card: {
          bg: '#ffffff',
        },
        bad: {
          DEFAULT: '#dc2626',
          50: '#fef2f2',
        },
        ok: {
          DEFAULT: '#16a34a',
          50: '#ecfdf3',
        },
        warn: {
          DEFAULT: '#d97706',
          50: '#fffbeb',
        },
        info: {
          DEFAULT: '#0891b2',
          50: '#e0f7fa',
        },
      },
      fontFamily: {
        sans: ['Inter', '-apple-system', 'Segoe UI', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'monospace'],
      },
      borderRadius: {
        card: '12px',
        sm: '8px',
      },
    },
  },
  plugins: [],
};
