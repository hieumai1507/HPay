import { router } from 'expo-router';
import { useMemo, useState } from 'react';
import { KeyboardAvoidingView, Platform, Pressable, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useMe, useTopup } from '@/src/queries';
import { describeError, isApiError } from '@/src/lib/errors';
import { useIdempotencyKey } from '@/src/lib/idempotency';
import { formatAmountVND, parseAmountToBigInt } from '@/src/lib/money';
import { AmountInput, Button } from '@/src/ui';

export default function TopupAmountScreen() {
  const [display, setDisplay] = useState('');
  const [submitError, setSubmitError] = useState<string | null>(null);

  const meQ = useMe();
  const topup = useTopup();

  const amount = useMemo(() => parseAmountToBigInt(display), [display]);
  const txnLimit = meQ.data ? BigInt(meQ.data.txn_limit) : null;

  const idempotencyKey = useIdempotencyKey(`topup:${amount ?? ''}`);

  const validationError =
    amount !== null && txnLimit !== null && amount > txnLimit
      ? `Vượt hạn mức KYC (tối đa ${formatAmountVND(txnLimit)})`
      : null;

  const canSubmit = amount !== null && amount > 0n && !validationError && !topup.isPending;

  const submit = () => {
    if (!canSubmit || amount === null) return;
    setSubmitError(null);
    topup.mutate(
      { amount, idempotencyKey },
      {
        onSuccess: (res) =>
          router.replace({
            pathname: '/(main)/topup/result',
            params: { refCode: res.ref_code, amount: amount.toString() },
          }),
        onError: (err) => {
          setSubmitError(describeError(err));
          if (isApiError(err) && err.code === 'ERR_LIMIT_EXCEEDED') meQ.refetch();
        },
      },
    );
  };

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
          <Text className="text-base font-semibold text-slate-900 dark:text-white">Nạp tiền</Text>
          <View className="w-8" />
        </View>

        <View className="flex-1 justify-center gap-3 p-6">
          <AmountInput
            value={display}
            onChangeValue={setDisplay}
            error={validationError ?? submitError ?? undefined}
            autoFocus
          />
          {txnLimit !== null && (
            <Text className="text-center text-xs text-slate-500">Hạn mức mỗi giao dịch: {formatAmountVND(txnLimit)}</Text>
          )}
        </View>

        <View className="p-4">
          <Button
            label="Nạp tiền"
            loading={topup.isPending}
            disabled={!canSubmit}
            onPress={submit}
          />
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
