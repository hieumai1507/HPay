import { useCallback } from 'react';
import { Text, TextInput, View } from 'react-native';

import { formatAmountWhileTyping, sanitizeAmountInput } from '@/src/lib/money';

export interface AmountInputProps {
  value: string;
  onChangeValue: (formatted: string) => void;
  error?: string;
  autoFocus?: boolean;
}

export function AmountInput({ value, onChangeValue, error, autoFocus }: AmountInputProps) {
  const handleChange = useCallback(
    (raw: string) => {
      onChangeValue(formatAmountWhileTyping(sanitizeAmountInput(raw)));
    },
    [onChangeValue],
  );

  return (
    <View className="items-center gap-2">
      <View className="flex-row items-center gap-2">
        <TextInput
          className={`text-4xl font-bold ${error ? 'color-rose-600' : 'text-slate-900 dark:text-white'} min-w-[80px] text-center p-0`}
          style={{ fontVariant: ['tabular-nums'] }}
          value={value}
          onChangeText={handleChange}
          keyboardType="number-pad"
          placeholder="0"
          placeholderTextColor="#cbd5e1"
          autoFocus={autoFocus}
          accessibilityLabel="Amount"
          maxLength={19}
        />
        <Text className="text-2xl font-semibold text-slate-400">₫</Text>
      </View>
      {error ? <Text className="text-xs color-rose-600">{error}</Text> : null}
    </View>
  );
}
