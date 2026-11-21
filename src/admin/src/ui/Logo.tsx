/**
 * HPay logo — vector recreation of the brand mark (wallet + HP monogram +
 * growth-arrow swoosh + circuit dots), single brand color so it works on both
 * the dark sidebar and light surfaces.
 */

const BRAND = '#1ECBE1';

export function LogoMark({ size = 28, color = BRAND }: { size?: number; color?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none" aria-hidden="true">
      {/* wallet body */}
      <rect x="10" y="14" width="42" height="38" rx="9" stroke={color} strokeWidth="3.5" />
      {/* clasp */}
      <rect x="41" y="29" width="13" height="11" rx="3.5" fill={color} />
      <circle cx="46" cy="34.5" r="2" fill="#fff" />
      {/* HP monogram */}
      <path
        d="M19 24v20M28 24v20M19 34h9"
        stroke={color}
        strokeWidth="4"
        strokeLinecap="round"
      />
      <path
        d="M33 44V24h4.5a5.5 5.5 0 0 1 0 11H33"
        stroke={color}
        strokeWidth="4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {/* growth arrow swoosh */}
      <path
        d="M4 51C16 46 40 32 53 13"
        stroke={color}
        strokeWidth="4"
        strokeLinecap="round"
      />
      <path d="M44.5 12.5 56 8l-2.5 12z" fill={color} />
      {/* circuit dots */}
      <circle cx="8" cy="24" r="2.2" fill={color} />
      <path d="M8 26v5" stroke={color} strokeWidth="1.8" strokeLinecap="round" />
      <circle cx="30" cy="58" r="2.2" fill={color} />
      <path d="M30 56v-4" stroke={color} strokeWidth="1.8" strokeLinecap="round" />
      <circle cx="58" cy="40" r="2.2" fill={color} />
      <path d="M56 40h-4" stroke={color} strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  );
}

interface LogoProps {
  size?: number;
  /** Mark color (defaults to brand accent). */
  color?: string;
  /** Wordmark color; omit to match `color`. */
  textColor?: string;
  showText?: boolean;
}

export function Logo({ size = 28, color = BRAND, textColor, showText = true }: LogoProps) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: size * 0.32 }}>
      <LogoMark size={size} color={color} />
      {showText && (
        <span
          style={{
            fontWeight: 800,
            fontSize: size * 0.68,
            letterSpacing: '-0.02em',
            color: textColor ?? color,
            lineHeight: 1,
          }}
        >
          HPay
        </span>
      )}
    </span>
  );
}
