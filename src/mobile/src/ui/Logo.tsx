import React from 'react';
import { Text, View } from 'react-native';
import Svg, { Circle, Path, Rect } from 'react-native-svg';

import { color } from './tokens';

/**
 * HPay logo — vector recreation of the brand mark (wallet + HP monogram +
 * growth-arrow swoosh + circuit dots). Same geometry as the admin web version
 * (src/admin/src/ui/Logo.tsx) — keep both in sync if the mark changes.
 */

export function LogoMark({ size = 32, tint = color.brand[600] }: { size?: number; tint?: string }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      {/* wallet body */}
      <Rect x={10} y={14} width={42} height={38} rx={9} stroke={tint} strokeWidth={3.5} />
      {/* clasp */}
      <Rect x={41} y={29} width={13} height={11} rx={3.5} fill={tint} />
      <Circle cx={46} cy={34.5} r={2} fill="#fff" />
      {/* HP monogram */}
      <Path d="M19 24v20M28 24v20M19 34h9" stroke={tint} strokeWidth={4} strokeLinecap="round" />
      <Path
        d="M33 44V24h4.5a5.5 5.5 0 0 1 0 11H33"
        stroke={tint}
        strokeWidth={4}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {/* growth arrow swoosh */}
      <Path d="M4 51C16 46 40 32 53 13" stroke={tint} strokeWidth={4} strokeLinecap="round" />
      <Path d="M44.5 12.5 56 8l-2.5 12z" fill={tint} />
      {/* circuit dots */}
      <Circle cx={8} cy={24} r={2.2} fill={tint} />
      <Path d="M8 26v5" stroke={tint} strokeWidth={1.8} strokeLinecap="round" />
      <Circle cx={30} cy={58} r={2.2} fill={tint} />
      <Path d="M30 56v-4" stroke={tint} strokeWidth={1.8} strokeLinecap="round" />
      <Circle cx={58} cy={40} r={2.2} fill={tint} />
      <Path d="M56 40h-4" stroke={tint} strokeWidth={1.8} strokeLinecap="round" />
    </Svg>
  );
}

interface LogoProps {
  size?: number;
  tint?: string;
  textColor?: string;
  showText?: boolean;
}

export function Logo({ size = 32, tint = color.brand[600], textColor, showText = true }: LogoProps) {
  return (
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: size * 0.3 }}>
      <LogoMark size={size} tint={tint} />
      {showText && (
        <Text
          style={{
            fontWeight: '800',
            fontSize: size * 0.68,
            letterSpacing: -0.5,
            color: textColor ?? tint,
          }}
        >
          HPay
        </Text>
      )}
    </View>
  );
}
