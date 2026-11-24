import { router, useLocalSearchParams } from 'expo-router';
import { useMemo, useState } from 'react';
import {
  KeyboardAvoidingView,
  Platform,
  Pressable,
  Text,
  TextInput,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useMainWallet, useMe } from '@/src/queries';
import { formatAmountVND, parseAmountToBigInt } from '@/src/lib/money';
import { AmountInput, Button } from '@/src/ui';

export default function TransferAmountScreen() {
  const { phone, maskedName } = useLocalSearchParams<{ phone: string; maskedName: string }>();
  const [display, setDisplay] = useState('');
  const [note, setNote] = useState('');

  const meQ = useMe();
  const walletQ = useMainWallet();

  const amount = useMemo(() => parseAmountToBigInt(display), [display]);
  const txnLimit = meQ.data ? BigInt(meQ.data.txn_limit) : null;
  const balance = walletQ.wallet ? BigInt(walletQ.wallet.balance) : null;

  const validationError =
    amount === null
      ? null
      : txnLimit !== null && amount > txnLimit
        ? `Exceeds KYC Limit (max ${formatAmountVND(txnLimit)})`
        : balance !== null && amount > balance
          ? 'Exceeds available balance'
          : null;

  const canContinue = amount !== null && amount > 0n && !validationError;

  return (
    <SafeAreaView className="flex-1 bg-white dark:bg-slate-900" edges={['top']}>
      <KeyboardAvoidingView
        className="flex-1"
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
      >
        <View className="flex-row items-center justify-between px-4 py-2 border-b border-slate-100 dark:border-slate-800">
          <Pressable onPress={() => router.back()} hitSlop={12} accessibilityRole="button">
            <Text className="text-xl font-bold text-slate-700 dark:text-slate-200 w-8">←</Text>
          </Pressable>
          <View className="items-center">
            <Text className="text-xs text-slate-500">Transfer to</Text>
            <Text className="text-sm font-semibold text-slate-900 dark:text-white">{maskedName}</Text>
          </View>
          <View className="w-8" />
        </View>

        <View className="flex-1 justify-center gap-4 p-6">
          <AmountInput
            value={display}
            onChangeValue={setDisplay}
            error={validationError ?? undefined}
            autoFocus
          />
          {balance !== null && (
            <Text className="text-center text-xs text-slate-500">Available balance: {formatAmountVND(balance)}</Text>
          )}
          <TextInput
            className="border border-slate-200 dark:border-slate-700 rounded-xl p-3 text-sm text-slate-900 dark:text-white bg-white dark:bg-slate-800"
            value={note}
            onChangeText={(t) => setNote(t.slice(0, 255))}
            placeholder="Note (optional)"
            placeholderTextColor="#94a3b8"
            accessibilityLabel="Note"
          />
        </View>

        <View className="p-4">
          <Button
            label="Next"
            disabled={!canContinue}
            onPress={() =>
              amount !== null &&
              router.push({
                pathname: '/(main)/transfer/confirm',
                params: { phone, maskedName, amount: amount.toString(), note },
              })
            }
          />
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
