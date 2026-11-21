import { Ionicons } from '@expo/vector-icons';
import { useRouter } from 'expo-router';
import React, { useState } from 'react';
import { ActivityIndicator, Alert, FlatList, Pressable, Text, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { formatAmountVND } from '@/src/lib/money';
import { describeError } from '@/src/lib/errors';
import { useCreateVault, useVaults, type Vault } from '@/src/queries';
import { useSecureScreen } from '@/src/features/security/useSecureScreen';
import { EmptyState } from '@/src/ui';
import { color } from '@/src/ui/tokens';

function progressPct(v: Vault): number | null {
  const target = v.target_amount ?? 0;
  if (!target) return null; // no goal set — show the amount, not a bar
  return Math.min(100, Math.round(((v.balance ?? 0) / target) * 100));
}

export default function VaultsScreen() {
  // Shows savings balances.
  useSecureScreen();

  const router = useRouter();
  const { data: vaults, isLoading, refetch, isRefetching } = useVaults();
  const createVault = useCreateVault();

  const [creating, setCreating] = useState(false);
  const [label, setLabel] = useState('');
  const [target, setTarget] = useState('');

  const total = (vaults ?? []).reduce((sum, v) => sum + (v.balance ?? 0), 0);

  const submitCreate = async () => {
    const name = label.trim();
    if (!name) {
      Alert.alert('Thiếu tên hũ', 'Đặt tên cho hũ tiết kiệm của bạn.');
      return;
    }
    try {
      await createVault.mutateAsync({
        label: name,
        targetAmount: target ? Number(target.replace(/\D/g, '')) : 0,
      });
      setLabel('');
      setTarget('');
      setCreating(false);
    } catch (e) {
      Alert.alert('Không tạo được hũ', describeError(e));
    }
  };

  const renderVault = ({ item }: { item: Vault }) => {
    const pct = progressPct(item);
    return (
      <Pressable
        onPress={() => router.push(`/(main)/vaults/${item.id}` as never)}
        className="bg-white dark:bg-slate-800 rounded-2xl p-4 mb-3 border border-slate-100 dark:border-slate-700"
      >
        <View className="flex-row items-center justify-between">
          <View className="flex-row items-center gap-2 flex-1">
            <View className="w-9 h-9 rounded-full items-center justify-center" style={{ backgroundColor: color.brand[50] }}>
              <Ionicons name="wallet-outline" size={18} color={color.brand[600]} />
            </View>
            <Text className="font-semibold text-slate-900 dark:text-white flex-1" numberOfLines={1}>
              {item.label}
            </Text>
            {item.round_up_target && (
              <View className="rounded-full px-2 py-0.5" style={{ backgroundColor: color.brand[50] }}>
                <Text className="text-[10px] font-bold" style={{ color: color.brand[600] }}>
                  Làm tròn
                </Text>
              </View>
            )}
          </View>
          <Text className="font-bold text-slate-900 dark:text-white tabular-nums">
            {formatAmountVND(BigInt(item.balance ?? 0))}
          </Text>
        </View>

        {pct !== null && (
          <View className="mt-3">
            <View className="h-2 rounded-full bg-slate-100 dark:bg-slate-700 overflow-hidden">
              <View className="h-2 rounded-full" style={{ width: `${pct}%`, backgroundColor: color.brand[400] }} />
            </View>
            <Text className="text-xs text-slate-500 mt-1">
              {pct}% của mục tiêu {formatAmountVND(BigInt(item.target_amount ?? 0))}
            </Text>
          </View>
        )}
      </Pressable>
    );
  };

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      <View className="flex-row items-center px-4 py-3">
        <Pressable onPress={() => router.back()} hitSlop={8} className="pr-3">
          <Ionicons name="arrow-back" size={22} color={color.neutral[500]} />
        </Pressable>
        <Text className="text-lg font-bold text-slate-900 dark:text-white">Hũ tiết kiệm</Text>
      </View>

      {/* Hero: total saved across every pocket */}
      <View className="mx-4 mb-4 rounded-2xl p-5" style={{ backgroundColor: color.brand[600] }}>
        <Text className="text-white/80 text-xs">Tổng để dành</Text>
        <Text className="text-white text-3xl font-bold tabular-nums mt-1">
          {formatAmountVND(BigInt(total))}
        </Text>
        <Text className="text-white/70 text-xs mt-1">
          {(vaults ?? []).length} hũ · tiền vẫn nằm trong ví của bạn
        </Text>
      </View>

      {isLoading ? (
        <ActivityIndicator className="mt-8" color={color.brand[600]} />
      ) : (
        <FlatList
          data={vaults ?? []}
          keyExtractor={(v) => String(v.id)}
          renderItem={renderVault}
          contentContainerClassName="px-4 pb-24"
          refreshing={isRefetching}
          onRefresh={refetch}
          ListEmptyComponent={
            <EmptyState
              title="Chưa có hũ nào"
              subtitle="Tạo một hũ để dành riêng cho mục tiêu của bạn."
            />
          }
        />
      )}

      {/* Create */}
      {creating ? (
        <View className="absolute bottom-0 left-0 right-0 bg-white dark:bg-slate-800 p-4 rounded-t-2xl border-t border-slate-200 dark:border-slate-700">
          <Text className="font-bold text-slate-900 dark:text-white mb-2">Hũ mới</Text>
          <TextInput
            value={label}
            onChangeText={setLabel}
            placeholder="Tên hũ (vd: Du lịch Đà Lạt)"
            maxLength={60}
            className="border border-slate-200 dark:border-slate-600 rounded-xl px-3 py-2.5 mb-2 text-slate-900 dark:text-white"
          />
          <TextInput
            value={target}
            onChangeText={setTarget}
            placeholder="Mục tiêu (tuỳ chọn)"
            keyboardType="number-pad"
            className="border border-slate-200 dark:border-slate-600 rounded-xl px-3 py-2.5 mb-3 text-slate-900 dark:text-white"
          />
          <View className="flex-row gap-2">
            <Pressable
              onPress={() => setCreating(false)}
              className="flex-1 items-center py-3 rounded-xl border border-slate-200 dark:border-slate-600"
            >
              <Text className="font-semibold text-slate-700 dark:text-slate-200">Huỷ</Text>
            </Pressable>
            <Pressable
              onPress={submitCreate}
              disabled={createVault.isPending}
              className="flex-1 items-center py-3 rounded-xl"
              style={{ backgroundColor: color.brand[600], opacity: createVault.isPending ? 0.6 : 1 }}
            >
              <Text className="font-bold text-white">Tạo hũ</Text>
            </Pressable>
          </View>
        </View>
      ) : (
        <Pressable
          onPress={() => setCreating(true)}
          className="absolute bottom-6 right-5 w-14 h-14 rounded-full items-center justify-center"
          style={{ backgroundColor: color.brand[600] }}
        >
          <Ionicons name="add" size={28} color="#fff" />
        </Pressable>
      )}
    </SafeAreaView>
  );
}
