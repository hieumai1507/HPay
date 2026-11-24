import { Pressable, Text, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';

import type { TxnView } from '@/src/queries';
import { AmountDisplay } from './AmountDisplay';
import { StatusBadge, type Status } from './StatusBadge';
import { color } from './tokens';

const TYPE_LABEL: Record<string, string> = {
  topup: 'Top up',
  p2p: 'Transfer',
  org_disbursement: 'Business disbursement',
  reversal: 'Refund',
};

export interface TxnListItemProps {
  txn: TxnView;
  onPress?: (refCode: string) => void;
}

export function TxnListItem({ txn, onPress }: TxnListItemProps) {
  const title = txn.counterparty_name ?? TYPE_LABEL[txn.type] ?? txn.type;
  const sign = txn.direction === 'in' ? 'in' : 'out';
  const isIncoming = txn.direction === 'in';

  let iconName: keyof typeof Ionicons.glyphMap = 'arrow-up';
  let iconBg: string = color.neutral[100];
  let iconColor: string = color.neutral[700];

  if (txn.type === 'topup') {
    iconName = 'arrow-down';
    iconBg = color.success[50];
    iconColor = color.success[600];
  } else if (txn.type === 'reversal') {
    iconName = 'arrow-undo';
    iconBg = color.warning[50];
    iconColor = color.warning[600];
  } else if (txn.type === 'org_disbursement') {
    iconName = 'business';
    iconBg = color.info[50];
    iconColor = color.info[600];
  } else {
    iconName = isIncoming ? 'arrow-down' : 'arrow-up';
    iconBg = isIncoming ? color.success[50] : color.neutral[100];
    iconColor = isIncoming ? color.success[600] : color.neutral[700];
  }

  return (
    <Pressable
      className="flex-row items-center gap-3 py-3 px-4 active:bg-slate-100 dark:active:bg-slate-800"
      onPress={() => onPress?.(txn.ref_code)}
      accessibilityRole="button"
      accessibilityLabel={`Transaction ${title}`}
    >
      <View className="w-10 h-10 rounded-full items-center justify-center" style={{ backgroundColor: iconBg }}>
        <Ionicons name={iconName} size={18} color={iconColor} />
      </View>
      <View className="flex-1 gap-0.5">
        <Text className="text-sm font-semibold text-slate-900 dark:text-slate-100" numberOfLines={1}>
          {title}
        </Text>
        {txn.note ? (
          <Text className="text-xs text-slate-500" numberOfLines={1}>
            {txn.note}
          </Text>
        ) : null}
      </View>
      <View className="items-end gap-1">
        <AmountDisplay valueMinorUnits={BigInt(txn.amount)} sign={sign} size="sm" />
        <StatusBadge status={txn.status as Status} />
      </View>
    </Pressable>
  );
}
