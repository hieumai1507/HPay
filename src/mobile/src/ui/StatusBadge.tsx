import { Text, View } from 'react-native';

export type Status =
  | 'created'
  | 'pending_approval'
  | 'processing'
  | 'running'
  | 'manual_review'
  | 'completed'
  | 'failed'
  | 'rejected'
  | 'expired'
  | 'reversed'
  | 'compensated';

const STATUS_CLASSES: Record<Status, { bg: string; fg: string; dot: string; icon?: string }> = {
  completed: { bg: 'bg-emerald-50', fg: 'color-emerald-700', dot: 'bg-emerald-600' },
  processing: { bg: 'bg-cyan-50', fg: 'color-cyan-700', dot: 'bg-cyan-600' },
  running: { bg: 'bg-cyan-50', fg: 'color-cyan-700', dot: 'bg-cyan-600' },
  pending_approval: { bg: 'bg-amber-50', fg: 'color-amber-700', dot: 'bg-amber-600' },
  manual_review: { bg: 'bg-amber-50', fg: 'color-amber-700', dot: 'bg-amber-600' },
  failed: { bg: 'bg-rose-50', fg: 'color-rose-700', dot: 'bg-rose-600' },
  rejected: { bg: 'bg-rose-50', fg: 'color-rose-700', dot: 'bg-rose-600' },
  expired: { bg: 'bg-rose-50', fg: 'color-rose-700', dot: 'bg-rose-600' },
  created: { bg: 'bg-slate-100', fg: 'color-slate-700', dot: 'bg-slate-500' },
  reversed: { bg: 'bg-slate-100', fg: 'color-slate-700', dot: 'bg-slate-500', icon: '↩' },
  compensated: { bg: 'bg-slate-100', fg: 'color-slate-700', dot: 'bg-slate-500', icon: '↩' },
};

const STATUS_LABEL: Record<Status, string> = {
  completed: 'Completed',
  processing: 'Processing',
  running: 'Processing',
  pending_approval: 'Pending',
  manual_review: 'Under review',
  failed: 'Failed',
  rejected: 'Rejected',
  expired: 'Expired',
  created: 'Created',
  reversed: 'Refunded',
  compensated: 'Refunded',
};

export interface StatusBadgeProps {
  status: Status;
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const s = STATUS_CLASSES[status] ?? STATUS_CLASSES.created;
  return (
    <View className={`flex-row items-center self-start rounded-full py-1 px-3 gap-1.5 ${s.bg}`}>
      {s.icon ? (
        <Text className={`text-xs font-bold ${s.fg}`}>{s.icon}</Text>
      ) : (
        <View className={`w-1.5 h-1.5 rounded-full ${s.dot}`} />
      )}
      <Text className={`text-xs font-semibold ${s.fg}`}>{STATUS_LABEL[status] ?? status}</Text>
    </View>
  );
}
