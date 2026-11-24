import { Ionicons } from '@expo/vector-icons';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Alert, Pressable, ScrollView, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useApprovalDetailByRefCode, useCancelApproval, type ApprovalDetail } from '@/src/queries';
import { formatAmountVND } from '@/src/lib/money';
import { describeError } from '@/src/lib/errors';

type DotState = 'done' | 'active' | 'wait' | 'rejected';

interface TimelineStep {
  title: string;
  sub?: string;
  dot: DotState;
}

const DOT_STYLE: Record<DotState, { circle: string; icon?: 'checkmark' | 'close'; textClass: string }> = {
  done: { circle: 'bg-emerald-500', icon: 'checkmark', textClass: 'text-slate-900 dark:text-white' },
  active: { circle: 'bg-brand', textClass: 'color-brand font-bold' },
  wait: { circle: 'bg-slate-200 dark:bg-slate-700', textClass: 'text-slate-400' },
  rejected: { circle: 'bg-rose-500', icon: 'close', textClass: 'color-rose-600 font-bold' },
};

function hoursLeft(iso: string): number {
  return Math.max(0, Math.round((new Date(iso).getTime() - Date.now()) / (60 * 60 * 1000)));
}

// Mockup 03-business.html frame 19 "Maker — Trạng thái lệnh" (order status): created -> each
// recorded signature -> current step (pending) or final result -> transfer.
export function buildTimeline(d: ApprovalDetail): TimelineStep[] {
  const steps: TimelineStep[] = [
    {
      title: 'Bạn đã tạo lệnh',
      sub: new Date(d.created_at).toLocaleString('vi-VN'),
      dot: 'done',
    },
  ];

  for (const sig of d.signatures) {
    if (sig.action === 'reject') {
      steps.push({
        title: `Level ${sig.level} — ${sig.checker_name} đã từ chối`,
        sub: `${new Date(sig.acted_at).toLocaleString('vi-VN')} · OTP ✓`,
        dot: 'rejected',
      });
      return steps;
    }
    steps.push({
      title: `Level ${sig.level} — ${sig.checker_name} đã duyệt`,
      sub: `${new Date(sig.acted_at).toLocaleString('vi-VN')} · OTP ✓`,
      dot: 'done',
    });
  }

  if (d.status === 'pending') {
    const remaining = Math.max(0, d.signatures_required - d.signatures.length);
    steps.push({
      title: `Level ${d.current_level} — chờ ${remaining} chữ ký`,
      sub: `hết hạn sau ${hoursLeft(d.expires_at)} giờ`,
      dot: 'active',
    });
    steps.push({ title: 'Thực hiện chuyển tiền', dot: 'wait' });
  } else if (d.status === 'approved') {
    steps.push({ title: 'Đã thực hiện chuyển tiền', dot: 'done' });
  } else if (d.status === 'expired') {
    steps.push({ title: 'Lệnh đã hết hạn — không thực hiện', dot: 'rejected' });
  } else if (d.status === 'cancelled') {
    steps.push({ title: 'Bạn đã hủy lệnh', dot: 'rejected' });
  }

  return steps;
}

function TimelineStepRow({ step, isLast }: { step: TimelineStep; isLast: boolean }) {
  const style = DOT_STYLE[step.dot];
  return (
    <View className="flex-row" style={{ paddingBottom: isLast ? 0 : 18 }}>
      <View className="items-center" style={{ width: 28 }}>
        <View className={`w-6 h-6 rounded-full items-center justify-center ${style.circle}`}>
          {style.icon && <Ionicons name={style.icon} size={13} color="#fff" />}
        </View>
        {!isLast && <View className="w-0.5 flex-1 bg-slate-200 dark:bg-slate-700 mt-1" />}
      </View>
      <View className="flex-1 pl-3" style={{ paddingBottom: isLast ? 0 : 4 }}>
        <Text className={`text-sm ${style.textClass}`}>{step.title}</Text>
        {step.sub && <Text className="text-xs text-slate-500 mt-0.5">{step.sub}</Text>}
      </View>
    </View>
  );
}

const STATUS_CHIP: Record<string, { text: string; className: string }> = {
  pending: { text: 'Chờ duyệt', className: 'bg-amber-50 color-amber-700' },
  approved: { text: 'Đã duyệt', className: 'bg-emerald-50 color-emerald-700' },
  rejected: { text: 'Đã từ chối', className: 'bg-rose-50 color-rose-700' },
  expired: { text: 'Đã hết hạn', className: 'bg-slate-100 color-slate-500' },
  cancelled: { text: 'Đã hủy', className: 'bg-slate-100 color-slate-500' },
};

export default function OrderStatusScreen() {
  const { refCode } = useLocalSearchParams<{ refCode: string }>();
  const { data, isLoading, isError } = useApprovalDetailByRefCode(refCode || '');
  const cancelMutation = useCancelApproval();
  const [cancelling, setCancelling] = useState(false);

  const confirmCancel = () => {
    if (!data) return;
    Alert.alert('Hủy lệnh', 'Bạn chắc chắn muốn hủy lệnh chi này? Thao tác này không thể hoàn tác.', [
      { text: 'Không', style: 'cancel' },
      {
        text: 'Hủy lệnh',
        style: 'destructive',
        onPress: async () => {
          try {
            setCancelling(true);
            await cancelMutation.mutateAsync({ id: data.id, refCode: data.ref_code });
          } catch (e: unknown) {
            Alert.alert('Không hủy được', describeError(e));
          } finally {
            setCancelling(false);
          }
        },
      },
    ]);
  };

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      <View className="flex-row items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
        <Pressable onPress={() => router.back()} hitSlop={12}>
          <Ionicons name="arrow-back" size={22} color="#0f172a" />
        </Pressable>
        <Text className="text-base font-bold text-slate-900 dark:text-white flex-1 text-center mx-2" numberOfLines={1}>
          Lệnh chi #{refCode}
        </Text>
        <View className="w-5" />
      </View>

      {isLoading ? (
        <View className="flex-1 items-center justify-center gap-3">
          <ActivityIndicator size="large" color="#0E90A3" />
          <Text className="text-xs text-slate-500">Đang khởi tạo lệnh…</Text>
        </View>
      ) : isError || !data ? (
        <View className="flex-1 items-center justify-center">
          <Text className="text-sm color-rose-600 font-semibold">Không thể tải trạng thái lệnh.</Text>
        </View>
      ) : (
        <ScrollView contentContainerStyle={{ padding: 16, gap: 16, paddingBottom: 40 }}>
          <View className="items-center py-3 gap-2">
            <Text className="text-3xl font-extrabold text-slate-900 dark:text-white">{formatAmountVND(BigInt(data.amount))}</Text>
            <Text className="text-xs text-slate-500">{data.org_name}</Text>
            <View className={`px-3 py-1 rounded-full ${STATUS_CHIP[data.status]?.className ?? 'bg-slate-100'}`}>
              <Text className={`text-xs font-bold ${STATUS_CHIP[data.status]?.className ?? ''}`}>
                {STATUS_CHIP[data.status]?.text ?? data.status}
                {data.status === 'pending' ? ` · ${data.signatures.length}/${data.signatures_required} chữ ký` : ''}
              </Text>
            </View>
          </View>

          <View className="bg-white dark:bg-slate-800 rounded-xl p-5 border border-slate-100 dark:border-slate-700 shadow-xs">
            <Text className="text-xs font-bold text-slate-500 tracking-wide mb-4">TIẾN TRÌNH PHÊ DUYỆT</Text>
            {buildTimeline(data).map((step, i, arr) => (
              <TimelineStepRow key={i} step={step} isLast={i === arr.length - 1} />
            ))}
          </View>

          <View className="bg-white dark:bg-slate-800 rounded-xl px-4 border border-slate-100 dark:border-slate-700 shadow-xs">
            <View className="flex-row justify-between items-center py-3.5">
              <Text className="text-xs font-semibold text-slate-500">Chính sách</Text>
              <Text className="text-sm font-medium text-slate-900 dark:text-white">{data.policy_name}</Text>
            </View>
            <View className="flex-row justify-between items-center py-3.5 border-t border-slate-100 dark:border-slate-700">
              <Text className="text-xs font-semibold text-slate-500">Phí</Text>
              <Text className="text-sm font-medium text-slate-900 dark:text-white">{formatAmountVND(BigInt(data.fee))}</Text>
            </View>
          </View>

          {data.status === 'pending' && (
            <Pressable
              onPress={confirmCancel}
              disabled={cancelling}
              className={`h-13 border-1.5 border-rose-100 dark:border-slate-700 rounded-xl items-center justify-center active:bg-rose-50 ${cancelling ? 'opacity-60' : ''}`}
            >
              {cancelling ? (
                <ActivityIndicator color="#e11d48" />
              ) : (
                <Text className="text-sm font-bold color-rose-600">Hủy lệnh</Text>
              )}
            </Pressable>
          )}
        </ScrollView>
      )}
    </SafeAreaView>
  );
}
