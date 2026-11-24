/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./app/**/*.{js,jsx,ts,tsx}",
    "./src/**/*.{js,jsx,ts,tsx}",
  ],
  presets: [require("nativewind/preset")],
  theme: {
    extend: {
      colors: {
        brand: {
          DEFAULT: '#0E90A3',
          600: '#0B7181',
          50: '#E6FAFD',
        },
        accent: '#1ECBE1',
        dark: '#0B1220',
        card: '#151F32',
        surface: '#1E293B',
      },
    },
  },
  plugins: [],
};
