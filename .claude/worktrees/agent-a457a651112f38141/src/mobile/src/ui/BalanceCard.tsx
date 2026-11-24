import { Ionicons } from '@expo/vector-icons';
import { LinearGradient } from 'expo-linear-gradient';
import { useEffect } from 'react';
import { Pressable, Text, View } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue, withTiming } from 'react-native-reanimated';

import { AmountDisplay } from './AmountDisplay';
import { color } from './tokens';

export interface BalanceCardProps {
  balanceMinorUnits: bigint | null;
  visible: boolean;
  onToggleVisible: () => void;
}

export function BalanceCard({ balanceMinorUnits, visible, onToggleVisible }: BalanceCardProps) {
  const visibleVal = useSharedValue(visible ? 1 : 0);

  useEffect(() => {
    visibleVal.value = withTiming(visible ? 1 : 0, { duration: 250 });
  }, [visible, visibleVal]);

  const amountStyle = useAnimatedStyle(() => ({
    opacity: visibleVal.value,
    transform: [{ translateY: (1 - visibleVal.value) * 6 }],
  }));

  const dotsStyle = useAnimatedStyle(() => ({
    opacity: 1 - visibleVal.value,
    transform: [{ translateY: visibleVal.value * -6 }],
  }));

  return (
    <LinearGradient
      colors={[color.brand[400], color.brand[700]]}
      start={{ x: 0, y: 0 }}
      end={{ x: 1, y: 1 }}
      className="rounded-2xl p-5 gap-3 shadow-md"
    >
      <View className="flex-row items-center justify-between">
        <Text className="text-xs text-white/90 font-semibold">Số dư khả dụng</Text>
        <Pressable
          onPress={onToggleVisible}
          hitSlop={12}
          accessibilityRole="button"
          accessibilityLabel={visible ? 'Ẩn số dư' : 'Hiện số dư'}
        >
          <Ionicons
            name={visible ? 'eye-off-outline' : 'eye-outline'}
            size={22}
            color="#fff"
            className="opacity-90"
          />
        </Pressable>
      </View>

      <View className="h-10 justify-center relative">
        {balanceMinorUnits !== null ? (
          <>
            <Animated.View className="absolute left-0 right-0" style={amountStyle} pointerEvents={visible ? 'auto' : 'none'}>
              <AmountDisplay valueMinorUnits={balanceMinorUnits} size="display" colorOverride="#fff" />
            </Animated.View>
            <Animated.View className="absolute left-0 right-0" style={dotsStyle} pointerEvents={!visible ? 'auto' : 'none'}>
              <Text className="text-2xl font-bold text-white tracking-widest">••••••</Text>
            </Animated.View>
          </>
        ) : (
          <Text className="text-2xl font-bold text-white tracking-widest">—</Text>
        )}
      </View>
    </LinearGradient>
  );
}
