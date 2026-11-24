import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useRef } from 'react';
import { ActivityIndicator, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  fetchTimeline,
  isTimelineSettled,
  resultVariantFor,
  type Timeline,
} from '@/src/queries';
import { PollingTransactionWatcher } from '@/src/lib/polling';
import { TimelineStep } from '@/src/ui';

const DISPLAY_STEPS = [
  { key: 'sent', label: 'Đã gửi lệnh' },
  { key: 'checking', label: 'Đang kiểm tra' },
  { key: 'done', label: 'Hoàn tất' },
] as const;

export default function TransferProcessingScreen() {
  const { refCode, amount } = useLocalSearchParams<{ refCode: string; amount: string }>();
  const navigated = useRef(false);

  useEffect(() => {
    if (!refCode) return;
    const watcher = new PollingTransactionWatcher<Timeline>(fetchTimeline, isTimelineSettled);
    const { result, cancel } = watcher.watch(refCode);

    result.then((outcome) => {
      if (navigated.current || outcome.kind === 'aborted') return;
      navigated.current = true;

      if (outcome.kind === 'timeout') {
        router.replace({
          pathname: '/(main)/transfer/result',
          params: { refCode, amount, variant: 'processing' },
        });
        return;
      }
      const tl = outcome.data;
      const variant = resultVariantFor(tl);
      router.replace({
        pathname: '/(main)/transfer/result',
        params: { refCode, amount, variant, failureCode: tl.failure_code ?? '' },
      });
    });

    return cancel;
  }, [refCode, amount]);

  return (
    <SafeAreaView className="flex-1 bg-white dark:bg-slate-900" edges={['top']}>
      <View className="flex-1 items-center justify-center gap-6 p-6">
        <ActivityIndicator size="large" color="#0E90A3" />
        <Text className="text-lg font-bold text-slate-900 dark:text-white">Đang xử lý giao dịch…</Text>
        <View className="self-stretch px-8">
          <TimelineStep label={DISPLAY_STEPS[0].label} state="done" />
          <TimelineStep label={DISPLAY_STEPS[1].label} state="active" />
          <TimelineStep label={DISPLAY_STEPS[2].label} state="waiting" isLast />
        </View>
        <Text className="text-xs text-slate-500">Vui lòng không thoát app trong giây lát.</Text>
      </View>
    </SafeAreaView>
  );
}
