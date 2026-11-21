/**
 * Design tokens — names mirror ui-design-spec.html §2 exactly (Figma variable
 * collections `color`/`space`/`radius`/`type`) so a future Figma Code Connect map
 * lines up 1:1. Do not rename without updating the spec doc first.
 *
 * Font: the spec mandates Inter app-wide. Bundling the Inter/JetBrains Mono font
 * assets via expo-font is not part of M1 scope (no real screens render text yet) —
 * `type.*` below carries the intended weights/sizes; wire up expo-font + the actual
 * typeface in M2 when the first real screen ships.
 */

export const color = {
  // Rebrand M4: chủ đạo #1ECBE1 (accent 400) — bậc 600/700 đậm hơn cho nút/link
  // để chữ trắng đạt contrast AA (#1ECBE1 nền trắng chỉ ~1.9:1, không dùng làm nền nút).
  brand: {
    50: '#E6FAFD',
    100: '#C9F4FA',
    300: '#64DDEC',
    400: '#1ECBE1',
    600: '#0E90A3',
    700: '#0B7181',
    900: '#073F48',
  },
  // 50-tints match the ones already used for chips/notes across the docs' own HTML/CSS
  // (e.g. ui-design-spec .tag, master-plan .t-adm/.bad-note) — kept consistent, not invented.
  success: { 50: '#ECFDF3', 600: '#16A34A' },
  warning: { 50: '#FFFBEB', 600: '#D97706' },
  danger: { 50: '#FEF2F2', 600: '#DC2626' },
  info: { 50: '#ECFEFF', 600: '#0891B2' },
  neutral: {
    50: '#F9FAFB',
    100: '#F3F4F6',
    200: '#E5E7EB',
    300: '#D1D5DB',
    400: '#9CA3AF',
    500: '#6B7280',
    600: '#4B5563',
    700: '#374151',
    800: '#1F2937',
    900: '#111827',
  },
} as const;

export const space = { 4: 4, 8: 8, 12: 12, 16: 16, 20: 20, 24: 24, 32: 32, 40: 40 } as const;

export const radius = { sm: 8, md: 12, lg: 16, full: 999 } as const;

export const type = {
  display: { fontSize: 32, lineHeight: 40, fontWeight: '700' as const },
  h1: { fontSize: 24, lineHeight: 32, fontWeight: '600' as const },
  h2: { fontSize: 18, lineHeight: 26, fontWeight: '600' as const },
  body: { fontSize: 15, lineHeight: 22, fontWeight: '400' as const },
  caption: { fontSize: 13, lineHeight: 18, fontWeight: '400' as const },
  mono: { fontSize: 13, lineHeight: 18, fontWeight: '500' as const },
} as const;

/** Apply to every Text node that renders a money amount — stops digits from jumping columns. */
export const tabularNums: { fontVariant: ('tabular-nums' | 'lining-nums')[] } = {
  fontVariant: ['tabular-nums'],
};
