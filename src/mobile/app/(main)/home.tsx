import { useSecureScreen } from '@/src/features/security/useSecureScreen';
import { Ionicons } from '@expo/vector-icons';
import { router } from 'expo-router';
import { useCallback } from 'react';
import {
  Pressable,
  RefreshControl,
  ScrollView,
  Text,
  View,
} from 'react-native';
import Animated, { FadeInUp, useAnimatedStyle, useSharedValue, withSpring } from 'react-native-reanimated';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useQuery } from '@tanstack/react-query';

import { apiRequest } from '@/src/api/client';
import { useMainWallet, useTransactions } from '@/src/queries';
import { describeError } from '@/src/lib/errors';
import { formatAmountVND } from '@/src/lib/money';
import { useOrgStore } from '@/src/stores/org';
import { useSessionStore } from '@/src/stores/session';
import { useSettingsStore } from '@/src/stores/settings';
import {
  BalanceCard,
  ErrorState,
  Skeleton,
  TxnListItem,
  color,
} from '@/src/ui';

function usePendingApprovalCount() {
  return useQuery({
    queryKey: ['org', 'approvals', 'count'],
    queryFn: async () => {
      const res = await apiRequest<{ approvals: unknown[] }>('/v1/org/approvals');
      return res.approvals.length;
    },
    staleTime: 30_000,
  });
}

const PERSONAL_ACTIONS = [
  { icon: 'wallet-outline' as const,  label: 'Top up',   route: '/(main)/topup/amount' },
  { icon: 'send-outline' as const,    label: 'Transfer', route: '/(main)/transfer/receiver' },
  { icon: 'qr-code-outline' as const, label: 'QR',          route: '/(main)/qr' },
  { icon: 'receipt-outline' as const, label: 'Request',    route: '/(main)/requests' },
  { icon: 'flower-outline' as const,  label: 'Vaults',          route: '/(main)/vaults' },
];

function QuickActionButton({
  icon,
  label,
  onPress,
  badge,
}: {
  icon: keyof typeof Ionicons.glyphMap;
  label: string;
  onPress: () => void;
  badge?: number;
}) {
  const scale = useSharedValue(1);

  const handlePressIn = () => {
    scale.value = withSpring(0.95, { damping: 10, stiffness: 250 });
  };
  const handlePressOut = () => {
    scale.value = withSpring(1, { damping: 10, stiffness: 250 });
  };
  const animatedStyle = useAnimatedStyle(() => ({
    transform: [{ scale: scale.value }],
  }));

  return (
    <Animated.View className="flex-1" style={animatedStyle}>
      <Pressable
        className="bg-white dark:bg-slate-800 rounded-xl py-3 items-center justify-center gap-1.5 border border-slate-200 dark:border-slate-700 min-h-[80px] shadow-xs active:bg-cyan-50"
        onPressIn={handlePressIn}
        onPressOut={handlePressOut}
        onPress={onPress}
        accessibilityRole="button"
        accessibilityLabel={label}
      >
        <View className="relative">
          <Ionicons name={icon} size={24} color={color.brand[600]} />
          {!!badge && badge > 0 && (
            <View className="absolute -top-1 -right-1 min-w-[16px] h-4 rounded-full bg-rose-600 items-center justify-center px-1">
              <Text className="text-[10px] font-bold text-white">{badge}</Text>
            </View>
          )}
        </View>
        <Text className="text-xs font-semibold text-slate-700 dark:text-slate-200">{label}</Text>
      </Pressable>
    </Animated.View>
  );
}

function BusinessBanner({ orgName, role }: { orgName: string; role: string }) {
  const exitOrg = useOrgStore((s) => s.exitOrg);
  return (
    <View className="flex-row items-center justify-between bg-white dark:bg-slate-800 rounded-xl p-4 border-l-4 border-brand border border-slate-200 dark:border-slate-700 shadow-xs">
      <View className="flex-1">
        <Text className="text-base font-bold text-slate-900 dark:text-white">{orgName}</Text>
        <Text className="text-xs text-slate-500 mt-0.5">Role: {role}</Text>
      </View>
      <Pressable onPress={exitOrg} className="flex-row items-center gap-1 bg-cyan-50 px-2.5 py-1.5 rounded-full">
        <Ionicons name="person-outline" size={14} color={color.brand[600]} />
        <Text className="text-xs font-semibold text-brand">Personal Wallet</Text>
      </Pressable>
    </View>
  );
}

export default function HomeScreen() {
  useSecureScreen();
  const user             = useSessionStore((s) => s.user);
  const balanceVisible   = useSettingsStore((s) => s.balanceVisible);
  const toggleBalanceVisible = useSettingsStore((s) => s.toggleBalanceVisible);
  const activeOrg        = useOrgStore((s) => s.activeOrg);

  const walletQ = useMainWallet();
  const txnsQ   = useTransactions();
  const { data: pendingCount } = usePendingApprovalCount();

  const refreshing = walletQ.isRefetching || txnsQ.isRefetching;
  const onRefresh = useCallback(() => {
    walletQ.refetch();
    txnsQ.refetch();
  }, [walletQ, txnsQ]);

  const recent = txnsQ.data?.pages[0]?.transactions.slice(0, 5) ?? [];

  const isBusinessMode = !!activeOrg;
  const quickActions = isBusinessMode
    ? buildBizActions(activeOrg.role, pendingCount ?? 0)
    : PERSONAL_ACTIONS;

  const balanceMinorUnits = isBusinessMode
    ? BigInt(activeOrg.balance)
    : walletQ.wallet
      ? BigInt(walletQ.wallet.balance)
      : null;

  const roleDisplay = isBusinessMode
    ? roleLabel(activeOrg.role, activeOrg.checkerLevel)
    : null;

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      <ScrollView
        contentContainerStyle={{ padding: 16, gap: 16 }}
        refreshControl={<RefreshControl refreshing={refreshing} onRefresh={onRefresh} />}
      >
        {isBusinessMode ? (
          <BusinessBanner orgName={activeOrg.name} role={roleDisplay!} />
        ) : (
          <View className="flex-row justify-between items-center">
            <View>
              <Text className="text-xs text-slate-500">Hello 👋</Text>
              <Text className="text-2xl font-bold text-slate-900 dark:text-white">{user?.full_name ?? ''}</Text>
            </View>
          </View>
        )}

        <BalanceCard
          balanceMinorUnits={balanceMinorUnits}
          visible={balanceVisible}
          onToggleVisible={toggleBalanceVisible}
        />

        <View className="flex-row gap-2">
          {quickActions.map((a) => (
            <QuickActionButton
              key={a.label}
              icon={a.icon}
              label={a.label}
              onPress={() => router.push(a.route as Parameters<typeof router.push>[0])}
              badge={'badge' in a ? (a.badge as number) : undefined}
            />
          ))}
        </View>

        <View className="flex-row justify-between items-center">
          <Text className="text-lg font-bold text-slate-900 dark:text-white">Recent transactions</Text>
          <Pressable onPress={() => router.push('/(main)/history')} accessibilityRole="button">
            <Text className="text-xs font-semibold text-brand">See all</Text>
          </Pressable>
        </View>

        {txnsQ.isLoading ? (
          <Skeleton rows={3} />
        ) : txnsQ.isError ? (
          <ErrorState message={describeError(txnsQ.error)} onRetry={() => txnsQ.refetch()} />
        ) : recent.length === 0 ? (
          <Text className="text-center text-slate-500 py-6">Nothing here yet. Top up to get started.</Text>
        ) : (
          <View className="bg-white dark:bg-slate-800 rounded-xl overflow-hidden shadow-xs">
            {recent.map((t, i) => (
              <Animated.View key={t.id} entering={FadeInUp.delay(i * 50).duration(300)}>
                <TxnListItem
                  txn={t}
                  onPress={(refCode) =>
                    router.push({ pathname: '/(main)/txn-detail', params: { refCode } })
                  }
                />
              </Animated.View>
            ))}
          </View>
        )}
      </ScrollView>
    </SafeAreaView>
  );
}

function roleLabel(role: string, checkerLevel: number): string {
  if (role === 'checker') return `Checker Level ${checkerLevel}`;
  if (role === 'maker')   return 'Maker';
  return 'Admin';
}

function buildBizActions(role: string, pendingCount: number) {
  const base = { badge: undefined as number | undefined };
  const history = { ...base, icon: 'time-outline' as const, label: 'History', route: '/(main)/history' };

  if (role === 'maker') {
    return [
      { ...base, icon: 'send-outline' as const, label: 'Create disbursement', route: '/(main)/org/disburse' },
      history,
    ];
  }
  if (role === 'checker') {
    return [
      { ...base, icon: 'checkmark-circle-outline' as const, label: `Approve (${pendingCount})`, route: '/(main)/org/approvals', badge: pendingCount || undefined },
      history,
    ];
  }
  return [
    { ...base, icon: 'wallet-outline' as const, label: 'Top up', route: '/(main)/org/topup' },
    { ...base, icon: 'settings-outline' as const, label: 'Admin', route: '/(main)/org/admin' },
    history,
  ];
}

void formatAmountVND;
