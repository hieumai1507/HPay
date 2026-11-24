import { Ionicons } from '@expo/vector-icons';
import { useLocalSearchParams, useRouter } from 'expo-router';
import React, { useMemo, useState } from 'react';
import { ActivityIndicator, Alert, Pressable, ScrollView, Switch, Text, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useSecureScreen } from '@/src/features/security/useSecureScreen';
import { describeError } from '@/src/lib/errors';
import { usePin } from '@/src/features/security/usePin';
import { withIntentKey } from '@/src/lib/intentKey';
import { formatAmountVND, parseAmountStrict, percentOf } from '@/src/lib/money';
import { useSubmitGuard } from '@/src/lib/useSubmitGuard';
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
  const { deposit, withdraw } = useVaultMove();
  const setRoundUpTarget = useSetRoundUpTarget();

  const [mode, setMode] = useState<Mode>('deposit');
  const [raw, setRaw] = useState('');
  const guard = useSubmitGuard();
  const { requestPin, pinPrompt } = usePin();

  const vault = useMemo(() => (vaults ?? []).find((v) => String(v.id) === String(id)), [vaults, id]);

  // Money stays a bigint end to end, including the API boundary (C-01).
  const parsed = parseAmountStrict(raw);
  const amount = parsed.ok ? parsed.value : 0n;
  const available = BigInt(
    (mode === 'deposit' ? mainWallet?.balance : vault?.balance) ?? 0,
  );
  const pct = vault?.target_amount
    ? percentOf(BigInt(vault.balance ?? 0), BigInt(vault.target_amount))
    : null;

  const submit = () =>
    guard.run(async () => {
      if (!vault?.id) return;
      if (!parsed.ok) {
        Alert.alert('Invalid amount', 'Enter an amount greater than 0.');
        return;
      }
      if (amount > available) {
        Alert.alert(
          'Insufficient balance',
          mode === 'deposit' ? 'Your main Wallet has insufficient funds.' : 'This Vault has insufficient funds to withdraw.',
        );
        return;
      }
      const pin = await requestPin();
      if (pin === null) return;
      try {
        const run = mode === 'deposit' ? deposit : withdraw;
        const vaultId = String(vault.id);
        await withIntentKey(`vault:${vaultId}:${mode}:${amount}`, (idempotencyKey) =>
          run(vaultId, amount, { idempotencyKey, pin }),
        );
        setRaw('');
        Alert.alert(
          'Success',
          mode === 'deposit' ? 'Money added to the Vault.' : 'Money withdrawn to your main Wallet.',
        );
      } catch (e) {
        Alert.alert('Could not complete', describeError(e));
      }
    });

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
        <Text className="text-slate-500 mt-10">Vault not found.</Text>
        <Pressable onPress={() => router.back()} className="mt-3">
          <Text style={{ color: color.brand[600] }} className="font-semibold">← Back</Text>
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
          <Text className="text-white/80 text-xs">Saved</Text>
          <Text className="text-white text-3xl font-bold tabular-nums mt-1">
            {formatAmountVND(BigInt(vault.balance ?? 0))}
          </Text>
          {pct !== null && (
            <>
              <View className="h-2 rounded-full bg-white/25 overflow-hidden mt-3">
                <View className="h-2 rounded-full bg-white" style={{ width: `${pct}%` }} />
              </View>
              <Text className="text-white/80 text-xs mt-1">
                {pct}% of target {formatAmountVND(BigInt(vault.target_amount ?? 0))}
              </Text>
            </>
          )}
        </View>

        {/* Round-up (docs 04 §2.2, stretch): sweeps the rounding difference of
            P2P spends into this vault. At most one vault can be the target. */}
        <View className="flex-row items-center justify-between bg-white dark:bg-slate-800 rounded-2xl p-4 mb-3 border border-slate-100 dark:border-slate-700">
          <View className="flex-1 pr-3">
            <Text className="font-semibold text-slate-900 dark:text-white">Receive round-up</Text>
            <Text className="text-xs text-slate-500 mt-0.5">
              On each Transfer, the spare change is added to this Vault automatically.
            </Text>
          </View>
          <Switch
            value={!!vault.round_up_target}
            disabled={setRoundUpTarget.isPending}
            onValueChange={(next) => {
              setRoundUpTarget.mutate(
                { vaultId: String(vault.id), enabled: next },
                {
                  onError: (e) => Alert.alert('Could not complete', describeError(e)),
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
                {m === 'deposit' ? 'Add' : 'Withdraw'}
              </Text>
            </Pressable>
          ))}
        </View>

        <View className="bg-white dark:bg-slate-800 rounded-2xl p-4 border border-slate-100 dark:border-slate-700">
          <Text className="text-xs text-slate-500 mb-1">Amount</Text>
          <TextInput
            value={raw}
            onChangeText={setRaw}
            placeholder="0"
            keyboardType="number-pad"
            className="text-2xl font-bold text-slate-900 dark:text-white tabular-nums"
          />
          <Text className="text-xs text-slate-500 mt-2">
            Available: {formatAmountVND(available)}{' '}
            {mode === 'deposit' ? '(main Wallet)' : '(in Vault)'}
          </Text>
        </View>

        <Pressable
          onPress={submit}
          disabled={guard.isPending}
          className="items-center py-4 rounded-2xl mt-4"
          style={{ backgroundColor: color.brand[600], opacity: guard.isPending ? 0.6 : 1 }}
        >
          <Text className="font-bold text-white text-base">
            {guard.isPending ? 'Processing…' : mode === 'deposit' ? 'Add to Vault' : 'Withdraw to Wallet'}
          </Text>
        </Pressable>

        <Text className="text-xs text-slate-400 text-center mt-3">
          Every deposit or withdrawal is a Transaction recorded in the ledger.
        </Text>
      </ScrollView>
      {pinPrompt}
    </SafeAreaView>
  );
}
