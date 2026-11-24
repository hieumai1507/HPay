import { Ionicons } from '@expo/vector-icons';
import { router } from 'expo-router';
import {
  ActivityIndicator,
  Pressable,
  RefreshControl,
  ScrollView,
  Text,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useApprovalInbox, type ApprovalItem } from '@/src/queries';
import { formatAmountUSD } from '@/src/lib/money';

function timeAgo(iso: string): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000;
  if (diff < 60)   return 'just now';
  if (diff < 3600) return `${Math.floor(diff / 60)} min ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} h ago`;
  return `${Math.floor(diff / 86400)} d ago`;
}

function timeLeft(iso: string): string {
  const diff = (new Date(iso).getTime() - Date.now()) / 1000;
  if (diff <= 0) return 'Expired';
  if (diff < 3600) return `expires in ${Math.floor(diff / 60)} min`;
  return `expires in ${Math.floor(diff / 3600)} h`;
}

function ApprovalCard({ item }: { item: ApprovalItem }) {
  return (
    <Pressable
      className="bg-white dark:bg-slate-800 rounded-2xl p-4 border-l-4 border-amber-500 shadow-xs active:opacity-85"
      onPress={() =>
        router.push({ pathname: '/(main)/org/approval-detail', params: { id: item.id } })
      }
    >
      <View className="flex-row items-start justify-between mb-2">
        <View>
          <Text className="text-xs text-slate-500 mb-1">{item.org_name} · #{item.ref_code}</Text>
          <Text className="text-2xl font-bold text-slate-900 dark:text-white">{formatAmountUSD(BigInt(item.amount))}</Text>
        </View>
        <View className="flex-row items-center gap-1.5 bg-amber-50 px-2 py-1 rounded-full">
          <View className="w-1.5 h-1.5 rounded-full bg-amber-500" />
          <Text className="text-[11px] font-bold color-amber-700">L{item.current_level}</Text>
        </View>
      </View>
      <Text className="text-xs text-slate-500 leading-relaxed mb-3">
        Created by <Text className="font-bold text-slate-700 dark:text-slate-300">{item.maker_name}</Text>{' '}
        · {timeAgo(item.expires_at)} · {timeLeft(item.expires_at)}
      </Text>
      <View className="flex-row gap-2">
        <Pressable
          className="flex-1 h-11 bg-brand rounded-xl items-center justify-center active:bg-brand-600"
          onPress={() =>
            router.push({ pathname: '/(main)/org/approval-detail', params: { id: item.id } })
          }
        >
          <Text className="text-sm font-bold text-white">Review & approve</Text>
        </Pressable>
      </View>
    </Pressable>
  );
}

export default function ApprovalsScreen() {
  const { data, isLoading, isError, refetch, isRefetching } = useApprovalInbox();
  const items = data?.approvals ?? [];

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      <View className="flex-row items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
        <Pressable onPress={() => router.back()} hitSlop={12}>
          <Ionicons name="arrow-back" size={22} color="#0f172a" />
        </Pressable>
        <Text className="text-base font-bold text-slate-900 dark:text-white">Awaiting my approval</Text>
        {items.length > 0 ? (
          <View className="bg-rose-600 min-w-[22px] h-5.5 rounded-full items-center justify-center px-1">
            <Text className="text-xs font-bold text-white">{items.length}</Text>
          </View>
        ) : (
          <View className="w-5" />
        )}
      </View>

      {isLoading ? (
        <View className="flex-1 items-center justify-center">
          <ActivityIndicator size="large" color="#0E90A3" />
        </View>
      ) : isError ? (
        <View className="flex-1 items-center justify-center gap-3">
          <Text className="text-sm color-rose-600 font-semibold">Could not load approvals.</Text>
          <Pressable onPress={() => refetch()} className="px-5 py-2.5 bg-brand rounded-xl">
            <Text className="text-white font-bold">Try again</Text>
          </Pressable>
        </View>
      ) : (
        <ScrollView
          contentContainerStyle={{ padding: 16, gap: 12 }}
          refreshControl={<RefreshControl refreshing={isRefetching} onRefresh={() => refetch()} />}
        >
          {items.length === 0 ? (
            <View className="items-center gap-2.5 pt-20">
              <Text className="text-5xl">✅</Text>
              <Text className="text-lg font-bold text-slate-900 dark:text-white">No approvals pending</Text>
              <Text className="text-sm text-slate-500 text-center">All requests within your scope have been handled.</Text>
            </View>
          ) : (
            items.map((item) => <ApprovalCard key={item.id} item={item} />)
          )}
        </ScrollView>
      )}
    </SafeAreaView>
  );
}
