import { router } from 'expo-router';
import { useMemo, useState } from 'react';
import { KeyboardAvoidingView, Platform, Pressable, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useMe, useTopup } from '@/src/queries';
import { describeError, isApiError } from '@/src/lib/errors';
import { FEATURE_AVAILABILITY } from '@/src/lib/availability';
import { withIntentKey } from '@/src/lib/intentKey';
import { formatAmountVND, parseAmountStrict } from '@/src/lib/money';
import { useSubmitGuard } from '@/src/lib/useSubmitGuard';
import { AmountInput, Button, NotAvailable } from '@/src/ui';

/**
 * BACKEND GAP: POST /v1/wallets/topup is not mounted (docs/reference/api.md). The engine
 * service TOPUP_BANK would credit the wallet from SYSTEM_CASH_IN with no bank step, so it is
 * not an equivalent. Until the backend decides, the screen is gated by one constant.
 */
export default function TopupAmountScreen() {
  if (!FEATURE_AVAILABILITY.topup) return <NotAvailable />;
  return <TopupForm />;
}

function TopupForm() {
  const [display, setDisplay] = useState('');
  const [submitError, setSubmitError] = useState<string | null>(null);

  const meQ = useMe();
  const topup = useTopup();
  const guard = useSubmitGuard();

  const parsed = useMemo(() => parseAmountStrict(display), [display]);
  const amount = parsed.ok ? parsed.value : null;
  const txnLimit = meQ.data ? BigInt(meQ.data.txn_limit) : null;

  const validationError =
    amount !== null && txnLimit !== null && amount > txnLimit
      ? `Vượt hạn mức KYC (tối đa ${formatAmountVND(txnLimit)})`
      : !parsed.ok && parsed.reason === 'too_large'
        ? 'Số tiền quá lớn'
        : null;

  const canSubmit = amount !== null && !validationError && !guard.isPending;

  const submit = () =>
    guard.run(async () => {
      if (!canSubmit || amount === null) return;
      setSubmitError(null);
      try {
        const res = await withIntentKey(`topup:${amount}`, (idempotencyKey) =>
          topup.mutateAsync({ amount, idempotencyKey }),
        );
        router.replace({
          pathname: '/(main)/topup/result',
          params: { refCode: res.ref_code, amount: amount.toString() },
        });
      } catch (err) {
        setSubmitError(describeError(err));
        if (isApiError(err) && err.code === 'ERR_LIMIT_EXCEEDED') meQ.refetch();
      }
    });

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
            loading={guard.isPending}
            disabled={!canSubmit}
            onPress={submit}
          />
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
