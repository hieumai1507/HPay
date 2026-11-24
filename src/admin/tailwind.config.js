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
        xs: '2px', // rounded-t-xs (charts)
      },
      // The pages were written with Tailwind v4 class names; this project builds with v3, which
      // silently emits no CSS for names it does not know (it broke the Offer Factory layout:
      // md:w-70 gave the left column no width). Define the names the code uses so they render.
      spacing: {
        '4.5': '1.125rem', // p-4.5
        '5.5': '1.375rem', // w-5.5, h-5.5
        '9.5': '2.375rem', // h-9.5
        '10.5': '2.625rem', // h-10.5
        '70': '17.5rem', // md:w-70 (offer services column)
      },
      borderWidth: {
        '1.5': '1.5px', // border-1.5
      },
      boxShadow: {
        xs: '0 1px 2px 0 rgb(0 0 0 / 0.05)', // shadow-xs
      },
      backdropBlur: {
        xs: '2px', // backdrop-blur-xs
      },
      fontSize: {
        '2.5xl': ['1.75rem', { lineHeight: '2.125rem' }], // text-2.5xl (dashboard KPI)
      },
    },
  },
  plugins: [],
};
