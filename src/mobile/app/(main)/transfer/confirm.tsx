import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { Alert, Pressable, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useTransactionEngine, useFeePreview } from '@/src/queries';
import { confirmWithAuth } from '@/src/features/security/confirmWithAuth';
import { classifyConfirmError, type ConfirmFailure } from '@/src/features/security/confirmErrors';
import { ConfirmFailureNotice } from '@/src/features/security/components/ConfirmFailureNotice';
import { SECURITY_ROUTES } from '@/src/features/security/routes';
import { useTransactionAuth } from '@/src/features/security/useTransactionAuth';
import { useRecentContacts } from '@/src/features/wallet/recent-contacts';
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

  const [failure, setFailure] = useState<ConfirmFailure | null>(null);
  const guard = useSubmitGuard();
  const { collect, prompt } = useTransactionAuth();
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
      setFailure(null);
      try {
        const { reqRes, confirmRes } = await withIntentKey(intent, async (idempotencyKey) => {
          const reqRes = await engine.requestTransaction({
            serviceCode: 'P2P_TRANSFER',
            amount,
            receiverPhone: params.phone || undefined,
            note: params.note || undefined,
            idempotencyKey,
          });
          // The band comes from the server; the factors of that band are collected here.
          // A wrong credential re-asks for the SAME held transaction, never a second request.
          const confirmRes = await confirmWithAuth({
            request: reqRes,
            collect,
            confirm: engine.confirmTransaction,
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
        const f = classifyConfirmError(err);
        if (f.kind === 'cancelled') return; // nothing moved; the intent key stays for a retry
        setFailure(f);
        // The held request is gone; the amount screen starts a fresh intent.
        if (f.kind === 'restart') {
          Alert.alert('Payment expired', f.message);
          router.replace(SECURITY_ROUTES.transferAmount as never);
        }
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
        <Text className="text-base font-semibold text-slate-900 dark:text-white">Confirm transfer</Text>
        <View className="w-8" />
      </View>

      <View className="flex-1 p-4 gap-3">
        <View className="bg-slate-50 dark:bg-slate-800 rounded-xl p-4 gap-3">
          <Row label="Recipient" value={params.maskedName ?? ''} />
          <Row label="Phone number" value={params.phone ?? ''} />
          <Row label="Amount" value={formatAmountVND(amount)} />
          <Row
            label="Fee"
            value={fee !== null ? formatAmountVND(fee) : 'Calculating…'}
          />
          {params.note ? <Row label="Note" value={params.note} /> : null}
          <View className="h-px bg-slate-200 dark:bg-slate-700" />
          <Row
            label="Total"
            value={total !== null ? formatAmountVND(total) : '—'}
            bold
          />
        </View>

        {isFirstTime && (
          <View className="bg-amber-50 rounded-xl p-3">
            <Text className="text-xs text-amber-700">
              ⚠️ This is your first transfer to this recipient. Check the name and phone number carefully.
            </Text>
          </View>
        )}
        <ConfirmFailureNotice failure={failure} />
      </View>

      <View className="p-4">
        <Button
          label="Transfer"
          loading={guard.isPending}
          disabled={!parsed.ok || fee === null || guard.isPending || failure?.blocking === true}
          onPress={submit}
        />
      </View>
      {prompt}
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
