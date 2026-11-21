import { ActivityIndicator, Pressable, Text, View, type PressableProps } from 'react-native';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';

export interface ButtonProps extends Omit<PressableProps, 'children' | 'disabled'> {
  variant?: ButtonVariant;
  label: string;
  loading?: boolean;
  disabled?: boolean;
}

const VARIANT_CLASSES: Record<ButtonVariant, { bg: string; text: string; border?: string }> = {
  primary: { bg: 'bg-brand active:bg-brand-600', text: 'text-white' },
  secondary: { bg: 'bg-slate-100 active:bg-slate-200 border border-slate-200', text: 'text-slate-900' },
  ghost: { bg: 'bg-transparent active:bg-cyan-50', text: 'text-brand' },
  danger: { bg: 'bg-rose-600 active:bg-rose-700', text: 'text-white' },
};

export function Button({ variant = 'primary', label, loading = false, disabled = false, style, ...pressableProps }: ButtonProps) {
  const v = VARIANT_CLASSES[variant];
  const isDisabled = disabled || loading;

  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ disabled: isDisabled, busy: loading }}
      disabled={isDisabled}
      className={`rounded-xl py-3 px-5 items-center justify-center min-h-[44px] ${v.bg} ${isDisabled && !loading ? 'opacity-40' : 'opacity-100'}`}
      {...pressableProps}
    >
      <View className="items-center justify-center">
        <Text className={`text-base font-semibold ${v.text} ${loading ? 'opacity-0' : 'opacity-100'}`} numberOfLines={1}>
          {label}
        </Text>
        {loading && (
          <View className="absolute inset-0 items-center justify-center">
            <ActivityIndicator color={variant === 'secondary' || variant === 'ghost' ? '#0E90A3' : '#FFFFFF'} />
          </View>
        )}
      </View>
    </Pressable>
  );
}
