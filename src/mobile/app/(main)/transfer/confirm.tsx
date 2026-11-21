import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { Pressable, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useTransactionEngine, useFeePreview } from '@/src/queries';
import { usePin } from '@/src/features/security/usePin';
import { useRecentContacts } from '@/src/features/wallet/recent-contacts';
import { describeError } from '@/src/lib/errors';
import { withIntentKey } from '@/src/lib/intentKey';
import { formatAmountVND, parseAmountStrict } from '@/src/lib/money';
import { useSubmitGuard } from '@/src/lib/useSubmitGuard';
import { Button } from '@/src/ui';

export default function TransferConfirmScreen() {
  const params = useLocalSearchParams<{
    phone?: string;
    maskedName: string;
    amount: string;
    note: string;
    receiverToken?: string;
  }>();
  const parsed = parseAmountStrict(params.amount ?? '');
  const amount = parsed.ok ? parsed.value : 0n;

  const [submitError, setSubmitError] = useState<string | null>(null);
  const guard = useSubmitGuard();
  const { requestPin, pinPrompt } = usePin();
  const engine = useTransactionEngine();
  const feeQ = useFeePreview(parsed.ok ? amount.toString() : null);
  const recents = useRecentContacts();
  const isFirstTime = params.phone ? !recents.contacts.some((c) => c.phone === params.phone) : false;

  // One key per transfer intent, persisted: stable across retries, remounts and restarts.
  const intent = `transfer:${params.receiverToken || params.phone || ''}:${amount}:${params.note ?? ''}`;

  const fee = feeQ.data ? BigInt(feeQ.data.fee) : null;
  const total = feeQ.data ? BigInt(feeQ.data.total) : null;

  const submit = () =>
    guard.run(async () => {
      setSubmitError(null);
      const pin = await requestPin();
      if (pin === null) return; // user cancelled; nothing was sent
      try {
        const { reqRes, confirmRes } = await withIntentKey(intent, async (idempotencyKey) => {
          const reqRes = await engine.requestTransaction({
            serviceCode: 'P2P_TRANSFER',
            amount,
            receiverPhone: params.phone || undefined,
            note: params.note || undefined,
            idempotencyKey,
          });
          const confirmRes = await engine.confirmTransaction({
            transactionId: reqRes.transaction_id,
            pin,
          });
          return { reqRes, confirmRes };
        });

        if (params.phone) {
          recents.add({ phone: params.phone, maskedName: params.maskedName });
        }

        router.replace({
          pathname: '/(main)/transfer/processing',
          params: { refCode: confirmRes.ref_code || reqRes.ref_code, amount: amount.toString() },
        });
      } catch (err: unknown) {
        setSubmitError(describeError(err));
      }
    });
  return (
    <SafeAreaView className="flex-1 bg-white dark:bg-slate-900" edges={['top']}>
      <View className="flex-row items-center justify-between px-4 py-2 border-b border-slate-100 dark:border-slate-800">
        <Pressable
          onPress={() => router.back()}
          hitSlop={12}
          accessibilityRole="button"
          disabled={guard.isPending}
        >
          <Text className="text-xl font-bold text-slate-700 dark:text-slate-200 w-8">←</Text>
        </Pressable>
        <Text className="text-base font-semibold text-slate-900 dark:text-white">Xác nhận chuyển tiền</Text>
        <View className="w-8" />
      </View>

      <View className="flex-1 p-4 gap-3">
        <View className="bg-slate-50 dark:bg-slate-800 rounded-xl p-4 gap-3">
          <Row label="Người nhận" value={params.maskedName ?? ''} />
          <Row label="Số điện thoại" value={params.phone ?? ''} />
          <Row label="Số tiền" value={formatAmountVND(amount)} />
          <Row
            label="Phí giao dịch"
            value={fee !== null ? formatAmountVND(fee) : 'Đang tính…'}
          />
          {params.note ? <Row label="Ghi chú" value={params.note} /> : null}
          <View className="h-px bg-slate-200 dark:bg-slate-700" />
          <Row
            label="Tổng trừ"
            value={total !== null ? formatAmountVND(total) : '—'}
            bold
          />
        </View>

        {isFirstTime && (
          <View className="bg-amber-50 rounded-xl p-3">
            <Text className="text-xs text-amber-700">
              ⚠️ Đây là lần đầu bạn chuyển cho người nhận này. Kiểm tra kỹ tên và số điện thoại.
            </Text>
          </View>
        )}
        {submitError && <Text className="text-xs color-rose-600 text-center">{submitError}</Text>}
      </View>

      <View className="p-4">
        <Button
          label="Chuyển tiền"
          loading={guard.isPending}
          disabled={!parsed.ok || fee === null || guard.isPending}
          onPress={submit}
        />
      </View>
      {pinPrompt}
    </SafeAreaView>
  );
}

function Row({ label, value, bold }: { label: string; value: string; bold?: boolean }) {
  return (
    <View className="flex-row justify-between gap-3">
      <Text className="text-sm text-slate-500">{label}</Text>
      <Text className={`text-sm text-slate-900 dark:text-white flex-shrink text-right ${bold ? 'font-bold' : ''}`}>{value}</Text>
    </View>
  );
}
