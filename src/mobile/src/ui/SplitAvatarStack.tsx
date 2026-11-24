import { Text, View } from 'react-native';

import { formatAmountUSD, percentOf } from '@/src/lib/money';
import { color } from './tokens';

// Distinct per-person identity colors, cycled by position — mirrors the
// hpay-docs mockup (06-requests-vaults.html #29) which colors each avatar
// differently rather than deriving a color from the name.
const AVATAR_COLORS = ['#F59E0B', '#10B981', '#8B5CF6', '#EF4444', '#3B82F6', '#EC4899'];

export interface SplitAvatarStackParticipant {
  id: string;
  name: string;
  paid: boolean;
}

export interface SplitAvatarStackProps {
  participants: SplitAvatarStackParticipant[];
  collectedAmount: bigint;
  totalAmount: bigint;
  /** ISO timestamp — when given, renders "Hh left" in the caption. */
  expiresAt?: string;
}

function initialsOf(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

function hoursRemaining(expiresAt?: string): number | null {
  if (!expiresAt) return null;
  const ms = new Date(expiresAt).getTime() - Date.now();
  if (Number.isNaN(ms)) return null;
  return Math.max(0, Math.round(ms / (60 * 60 * 1000)));
}

/**
 * Split Bill tracking hero (docs ui-design-spec.html §8, mockup screen 29):
 * overlapping avatars — unpaid at reduced opacity — plus the collected/total
 * progress bar for the whole group.
 */
export function SplitAvatarStack({ participants, collectedAmount, totalAmount, expiresAt }: SplitAvatarStackProps) {
  const paidCount = participants.filter((p) => p.paid).length;
  const pct = percentOf(collectedAmount, totalAmount);
  const hoursLeft = hoursRemaining(expiresAt);

  return (
    <View className="bg-white dark:bg-slate-800 rounded-xl p-5 items-center border border-slate-100 dark:border-slate-700 shadow-xs gap-3">
      <View className="flex-row">
        {participants.map((p, i) => (
          <View
            key={p.id}
            testID={`split-avatar-${p.id}`}
            className="w-10 h-10 rounded-full items-center justify-center border-2 border-white dark:border-slate-800"
            style={{
              backgroundColor: AVATAR_COLORS[i % AVATAR_COLORS.length],
              marginLeft: i === 0 ? 0 : -12,
              opacity: p.paid ? 1 : 0.45,
              zIndex: participants.length - i,
            }}
          >
            <Text className="text-white text-xs font-bold">{initialsOf(p.name)}</Text>
          </View>
        ))}
      </View>

      <Text className="text-lg font-bold text-slate-900 dark:text-white">
        Collected {formatAmountUSD(collectedAmount)} / {formatAmountUSD(totalAmount)}
      </Text>

      <View className="w-full h-2.5 rounded-full bg-slate-100 dark:bg-slate-700 overflow-hidden">
        <View className="h-full rounded-full" style={{ width: `${pct}%`, backgroundColor: color.success[600] }} />
      </View>

      <Text className="text-xs text-slate-500">
        {paidCount}/{participants.length} paid
        {hoursLeft !== null ? ` · ${hoursLeft}h left` : ''}
      </Text>
    </View>
  );
}
