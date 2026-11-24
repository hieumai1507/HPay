import { useSecureScreen } from '@/src/features/security/useSecureScreen';
import * as Clipboard from 'expo-clipboard';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { Pressable, ScrollView, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useTimelineQuery, useTxnDetail } from '@/src/queries';
import { describeError } from '@/src/lib/errors';
import { formatAmountUSD } from '@/src/lib/money';
import {
  AmountDisplay,
  ErrorState,
  Skeleton,
  StatusBadge,
  TimelineStep,
  type Status,
  type TimelineStepState,
} from '@/src/ui';

const STEP_LABEL: Record<string, string> = {
  VALIDATE: 'Checking details',
  FRAUD_CHECK: 'Security check',
  MOVE_FUNDS: 'Moving funds',
  CONFIRM: 'Completed',
};

function stepState(status: string): TimelineStepState {
  switch (status) {
    case 'completed':
      return 'done';
    case 'failed':
      return 'failed';
    case 'running':
      return 'active';
    default:
      return 'waiting';
  }
}

export default function TxnDetailScreen() {
  useSecureScreen();
  const { refCode } = useLocalSearchParams<{ refCode: string }>();
  const [copied, setCopied] = useState(false);
  const q = useTxnDetail(refCode ?? '');
  const tlQ = useTimelineQuery(refCode ?? '');

  const copyRef = async () => {
    await Clipboard.setStringAsync(refCode ?? '');
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900">
      <View className="flex-row items-center justify-between px-4 py-2 bg-white dark:bg-slate-800 border-b border-slate-200 dark:border-slate-700">
        <Pressable onPress={() => router.back()} hitSlop={12} accessibilityRole="button">
          <Text className="text-xl font-bold text-slate-700 dark:text-slate-200 w-8">←</Text>
        </Pressable>
        <Text className="text-base font-semibold text-slate-900 dark:text-white">Transaction details</Text>
        <View className="w-8" />
      </View>

      {q.isLoading ? (
        <Skeleton rows={4} />
      ) : q.isError || !q.data ? (
        <ErrorState message={describeError(q.error)} onRetry={() => q.refetch()} />
      ) : (
        <ScrollView contentContainerStyle={{ padding: 16, gap: 16 }}>
          <View className="items-center gap-2 py-4">
            <AmountDisplay
              valueMinorUnits={BigInt(q.data.amount)}
              sign={q.data.direction === 'in' ? 'in' : 'out'}
              size="display"
            />
            <StatusBadge status={q.data.status as Status} />
          </View>

          <View className="bg-white dark:bg-slate-800 rounded-xl p-4 gap-3 shadow-xs">
            {q.data.counterparty_name && (
              <Row label={q.data.direction === 'in' ? 'From' : 'To'} value={q.data.counterparty_name} />
            )}
            {q.data.counterparty_phone && <Row label="Phone number" value={q.data.counterparty_phone} />}
            {q.data.fee > 0 && (
              <Row label="Fee" value={formatAmountUSD(BigInt(q.data.fee))} />
            )}
            {q.data.note ? <Row label="Note" value={q.data.note} /> : null}
            <Row label="Time" value={new Date(q.data.created_at).toLocaleString('en-GB')} />
            <Pressable onPress={copyRef} accessibilityRole="button" accessibilityLabel="Copy transaction ID">
              <View className="flex-row justify-between gap-3">
                <Text className="text-sm text-slate-500">Transaction ID</Text>
                <Text className="text-sm font-mono text-brand">
                  {refCode} {copied ? '✓' : '⧉'}
                </Text>
              </View>
            </Pressable>
          </View>

          {tlQ.data && tlQ.data.timeline.instance_status !== 'none' && (
            <View className="bg-white dark:bg-slate-800 rounded-xl p-4 gap-3 shadow-xs">
              <Text className="text-sm font-semibold text-slate-900 dark:text-white">Progress</Text>
              {tlQ.data.timeline.steps.map((s, i) => (
                <TimelineStep
                  key={`${s.step}-${i}`}
                  label={STEP_LABEL[s.step] ?? s.step}
                  state={stepState(s.status)}
                  isLast={i === tlQ.data.timeline.steps.length - 1}
                  meta={s.finished_at ? new Date(s.finished_at).toLocaleTimeString('en-GB') : undefined}
                />
              ))}
            </View>
          )}
        </ScrollView>
      )}
    </SafeAreaView>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <View className="flex-row justify-between gap-3">
      <Text className="text-sm text-slate-500">{label}</Text>
      <Text className="text-sm text-slate-900 dark:text-white flex-shrink text-right">{value}</Text>
    </View>
  );
}
