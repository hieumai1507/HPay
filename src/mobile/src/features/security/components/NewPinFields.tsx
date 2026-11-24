import { Text, View } from 'react-native';

import { OTPInput } from '@/src/features/auth/components/OTPInput';

interface Props {
  pin: string;
  onPin: (v: string) => void;
  repeat: string;
  onRepeat: (v: string) => void;
  hint: string | null;
  mismatch: boolean;
  disabled?: boolean;
}

/** New PIN + repeat, with instant client-side hints (the server remains the authority). */
export function NewPinFields({ pin, onPin, repeat, onRepeat, hint, mismatch, disabled }: Props) {
  return (
    <View className="gap-3">
      <Text className="text-sm font-semibold text-slate-700">New PIN</Text>
      <OTPInput value={pin} onChange={onPin} secureTextEntry disabled={disabled} label="New PIN digit" />
      {hint ? <Text accessibilityRole="alert" className="text-xs text-rose-600">{hint}</Text> : null}
      <Text className="text-sm font-semibold text-slate-700">Repeat new PIN</Text>
      <OTPInput value={repeat} onChange={onRepeat} secureTextEntry disabled={disabled} label="Repeat PIN digit" />
      {mismatch ? <Text accessibilityRole="alert" className="text-xs text-rose-600">The two PINs do not match.</Text> : null}
    </View>
  );
}