import { Ionicons } from '@expo/vector-icons';
import { useLocalSearchParams, useRouter } from 'expo-router';
import React, { useMemo, useState } from 'react';
import { ActivityIndicator, Alert, Pressable, ScrollView, Switch, Text, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useSecureScreen } from '@/src/features/security/useSecureScreen';
import { describeError } from '@/src/lib/errors';
import { useIdempotencyKey } from '@/src/lib/idempotency';
import { formatAmountVND, parseAmountToBigInt } from '@/src/lib/money';
import { useMainWallet, useSetRoundUpTarget, useVaultMove, useVaults } from '@/src/queries';
import { color } from '@/src/ui/tokens';

type Mode = 'deposit' | 'withdraw';

export default function VaultDetailScreen() {
  // Shows a balance.
  useSecureScreen();

  const router = useRouter();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { data: vaults, isLoading } = useVaults();
  const { wallet: mainWallet } = useMainWallet();
  const { deposit, withdraw, isMoving } = useVaultMove();
  const setRoundUpTarget = useSetRoundUpTarget();

  const [mode, setMode] = useState<Mode>('deposit');
  const [raw, setRaw] = useState('');
  // One key per attempt: resets after a move so the next one is a new request
  // rather than a replay of the last (M01).
  const [attempt, setAttempt] = useState(0);
  const idempotencyKey = useIdempotencyKey(String(attempt));

  const vault = useMemo(() => (vaults ?? []).find((v) => String(v.id) === String(id)), [vaults, id]);

  // Money stays a bigint end to end; Number() only at the API boundary (M06).
  const amount = parseAmountToBigInt(raw) ?? 0n;
  const available = BigInt(
    (mode === 'deposit' ? mainWallet?.balance : vault?.balance) ?? 0,
  );
  const pct = vault?.target_amount
    ? Math.min(100, Math.round(((vault.balance ?? 0) / vault.target_amount) * 100))
    : null;

  const submit = async () => {
    if (!vault?.id) return;
    if (amount <= 0n) {
      Alert.alert('Số tiền không hợp lệ', 'Nhập số tiền lớn hơn 0.');
      return;
    }
    if (amount > available) {
      Alert.alert(
        'Không đủ số dư',
        mode === 'deposit' ? 'Ví chính không đủ tiền.' : 'Hũ này không đủ tiền để rút.',
      );
      return;
    }
    try {
      const run = mode === 'deposit' ? deposit : withdraw;
      await run(String(vault.id), Number(amount), idempotencyKey);
      setRaw('');
      setAttempt((n) => n + 1);
      Alert.alert(
        'Thành công',
        mode === 'deposit' ? 'Đã bỏ tiền vào hũ.' : 'Đã rút tiền về ví chính.',
      );
    } catch (e) {
      Alert.alert('Không thực hiện được', describeError(e));
    }
  };

  if (isLoading) {
    return (
      <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900">
        <ActivityIndicator className="mt-10" color={color.brand[600]} />
      </SafeAreaView>
    );
  }

  if (!vault) {
    return (
      <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900 px-4">
        <Text className="text-slate-500 mt-10">Không tìm thấy hũ này.</Text>
        <Pressable onPress={() => router.back()} className="mt-3">
          <Text style={{ color: color.brand[600] }} className="font-semibold">← Quay lại</Text>
        </Pressable>
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      <View className="flex-row items-center px-4 py-3">
        <Pressable onPress={() => router.back()} hitSlop={8} className="pr-3">
          <Ionicons name="arrow-back" size={22} color={color.neutral[500]} />
        </Pressable>
        <Text className="text-lg font-bold text-slate-900 dark:text-white flex-1" numberOfLines={1}>
          {vault.label}
        </Text>
      </View>

      <ScrollView contentContainerClassName="px-4 pb-10">
        <View className="rounded-2xl p-5 mb-4" style={{ backgroundColor: color.brand[600] }}>
          <Text className="text-white/80 text-xs">Đang để dành</Text>
          <Text className="text-white text-3xl font-bold tabular-nums mt-1">
            {formatAmountVND(BigInt(vault.balance ?? 0))}
          </Text>
          {pct !== null && (
            <>
              <View className="h-2 rounded-full bg-white/25 overflow-hidden mt-3">
                <View className="h-2 rounded-full bg-white" style={{ width: `${pct}%` }} />
              </View>
              <Text className="text-white/80 text-xs mt-1">
                {pct}% của mục tiêu {formatAmountVND(BigInt(vault.target_amount ?? 0))}
              </Text>
            </>
          )}
        </View>

        {/* Round-up (docs 04 §2.2, stretch): sweeps the rounding difference of
            P2P spends into this vault. At most one vault can be the target. */}
        <View className="flex-row items-center justify-between bg-white dark:bg-slate-800 rounded-2xl p-4 mb-3 border border-slate-100 dark:border-slate-700">
          <View className="flex-1 pr-3">
            <Text className="font-semibold text-slate-900 dark:text-white">Nhận tiền làm tròn</Text>
            <Text className="text-xs text-slate-500 mt-0.5">
              Mỗi lần chuyển tiền lẻ, phần dư sẽ tự động bỏ vào hũ này.
            </Text>
          </View>
          <Switch
            value={!!vault.round_up_target}
            disabled={setRoundUpTarget.isPending}
            onValueChange={(next) => {
              setRoundUpTarget.mutate(
                { vaultId: String(vault.id), enabled: next },
                {
                  onError: (e) => Alert.alert('Không thực hiện được', describeError(e)),
                },
              );
            }}
            trackColor={{ true: color.brand[600] }}
          />
        </View>

        {/* Direction */}
        <View className="flex-row bg-slate-100 dark:bg-slate-800 rounded-xl p-1 mb-3">
          {(['deposit', 'withdraw'] as Mode[]).map((m) => (
            <Pressable
              key={m}
              onPress={() => setMode(m)}
              className="flex-1 items-center py-2.5 rounded-lg"
              style={{ backgroundColor: mode === m ? '#fff' : 'transparent' }}
            >
              <Text
                className="font-semibold"
                style={{ color: mode === m ? color.brand[600] : color.neutral[500] }}
              >
                {m === 'deposit' ? 'Bỏ vào' : 'Rút về'}
              </Text>
            </Pressable>
          ))}
        </View>

        <View className="bg-white dark:bg-slate-800 rounded-2xl p-4 border border-slate-100 dark:border-slate-700">
          <Text className="text-xs text-slate-500 mb-1">Số tiền</Text>
          <TextInput
            value={raw}
            onChangeText={setRaw}
            placeholder="0"
            keyboardType="number-pad"
            className="text-2xl font-bold text-slate-900 dark:text-white tabular-nums"
          />
          <Text className="text-xs text-slate-500 mt-2">
            Khả dụng: {formatAmountVND(available)}{' '}
            {mode === 'deposit' ? '(ví chính)' : '(trong hũ)'}
          </Text>
        </View>

        <Pressable
          onPress={submit}
          disabled={isMoving}
          className="items-center py-4 rounded-2xl mt-4"
          style={{ backgroundColor: color.brand[600], opacity: isMoving ? 0.6 : 1 }}
        >
          <Text className="font-bold text-white text-base">
            {isMoving ? 'Đang xử lý…' : mode === 'deposit' ? 'Bỏ vào hũ' : 'Rút về ví'}
          </Text>
        </Pressable>

        <Text className="text-xs text-slate-400 text-center mt-3">
          Mỗi lần bỏ vào hoặc rút ra đều là một giao dịch có ghi sổ cái.
        </Text>
      </ScrollView>
    </SafeAreaView>
  );
}
