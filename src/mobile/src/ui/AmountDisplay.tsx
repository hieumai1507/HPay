import { Text } from 'react-native';

import { formatAmountDisplay } from '@/src/lib/money';

export type AmountSign = 'in' | 'out' | 'none';
export type AmountDisplaySize = 'sm' | 'md' | 'display';

export interface AmountDisplayProps {
  valueMinorUnits: bigint;
  sign?: AmountSign;
  size?: AmountDisplaySize;
  colorOverride?: string;
}

const SIZE_CLASSES: Record<AmountDisplaySize, string> = {
  sm: 'text-sm',
  md: 'text-2xl',
  display: 'text-4xl',
};

export function AmountDisplay({ valueMinorUnits, sign = 'none', size = 'md', colorOverride }: AmountDisplayProps) {
  const magnitude = valueMinorUnits < 0n ? -valueMinorUnits : valueMinorUnits;
  const formatted = formatAmountDisplay(magnitude);

  const prefix = sign === 'in' ? '+' : sign === 'out' ? '−' : '';
  const colorClass = colorOverride
    ? ''
    : sign === 'in'
      ? 'color-emerald-600 dark:color-emerald-400'
      : 'text-slate-900 dark:text-white';

  return (
    <Text
      className={`font-bold ${SIZE_CLASSES[size]} ${colorClass}`}
      style={colorOverride ? { color: colorOverride, fontVariant: ['tabular-nums'] } : { fontVariant: ['tabular-nums'] }}
    >
      {`${prefix}$${formatted}`}
    </Text>
  );
}
