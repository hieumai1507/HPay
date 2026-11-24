import React, { useState } from 'react';
import { Text, View, Pressable, FlatList, ActivityIndicator } from 'react-native';
import { useRouter } from 'expo-router';
import { SafeAreaView } from 'react-native-safe-area-context';
import { Ionicons } from '@expo/vector-icons';

import { useRequestInbox, useSentRequests, type InboxRequest, type SentRequest } from '@/src/queries';
import { formatAmountVND } from '@/src/lib/money';
import { color } from '@/src/ui/tokens';
import { EmptyState } from '@/src/ui';

type TabType = 'inbox' | 'sent';

export default function RequestsIndexScreen() {
  const router = useRouter();
  const [activeTab, setActiveTab] = useState<TabType>('inbox');

  const inboxQuery = useRequestInbox();
  const sentQuery = useSentRequests();

  const isLoading = activeTab === 'inbox' ? inboxQuery.isLoading : sentQuery.isLoading;
  const isRefetching = activeTab === 'inbox' ? inboxQuery.isRefetching : sentQuery.isRefetching;
  const data = activeTab === 'inbox' ? inboxQuery.data?.requests : sentQuery.data?.requests;

  const handleRefresh = () => {
    if (activeTab === 'inbox') {
      inboxQuery.refetch();
    } else {
      sentQuery.refetch();
    }
  };

  const getStatusLabel = (status: string) => {
    switch (status) {
      case 'pending': return { text: 'Pending', color: color.warning[600], bg: 'bg-amber-50' };
      case 'paid': return { text: 'Paid', color: color.success[600], bg: 'bg-emerald-50' };
      case 'declined': return { text: 'Rejected', color: color.danger[600], bg: 'bg-rose-50' };
      case 'canceled': return { text: 'Canceled', color: color.neutral[500], bg: 'bg-slate-100' };
      case 'expired': return { text: 'Expired', color: color.neutral[400], bg: 'bg-slate-100' };
      default: return { text: status, color: color.neutral[500], bg: 'bg-slate-100' };
    }
  };

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      {/* Header */}
      <View className="flex-row items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
        <Pressable onPress={() => router.back()} className="p-1">
          <Ionicons name="arrow-back" size={24} color="#0f172a" />
        </Pressable>
        <Text className="text-lg font-bold text-slate-900 dark:text-white">Money requests</Text>
        <View className="min-w-[32px] items-end">
          <Pressable onPress={() => router.push('/(main)/requests/create')} className="p-1">
            <Ionicons name="add" size={24} color="#0E90A3" />
          </Pressable>
        </View>
      </View>

      {/* Tabs */}
      <View className="flex-row bg-white dark:bg-slate-800 px-4 border-b border-slate-100 dark:border-slate-700">
        <Pressable
          onPress={() => setActiveTab('inbox')}
          className={`flex-1 py-4 items-center border-b-2 ${activeTab === 'inbox' ? 'border-brand' : 'border-transparent'}`}
        >
          <Text className={`text-sm font-semibold ${activeTab === 'inbox' ? 'text-brand' : 'text-slate-400'}`}>
            To pay
          </Text>
        </Pressable>
        <Pressable
          onPress={() => setActiveTab('sent')}
          className={`flex-1 py-4 items-center border-b-2 ${activeTab === 'sent' ? 'border-brand' : 'border-transparent'}`}
        >
          <Text className={`text-sm font-semibold ${activeTab === 'sent' ? 'text-brand' : 'text-slate-400'}`}>
            Sent by me
          </Text>
        </Pressable>
      </View>

      {/* Quick Actions Bar */}
      <View className="flex-row bg-white dark:bg-slate-800 py-3 my-3 mx-4 rounded-xl border border-slate-100 dark:border-slate-700">
        <Pressable onPress={() => router.push('/(main)/requests/create')} className="flex-1 flex-row items-center justify-center gap-2">
          <Ionicons name="send-outline" size={20} color="#0E90A3" />
          <Text className="text-sm font-semibold text-slate-700 dark:text-slate-200">Request money</Text>
        </Pressable>
        <View className="w-px bg-slate-100 dark:bg-slate-700" />
        <Pressable onPress={() => router.push('/(main)/requests/split')} className="flex-1 flex-row items-center justify-center gap-2">
          <Ionicons name="people-outline" size={20} color="#0E90A3" />
          <Text className="text-sm font-semibold text-slate-700 dark:text-slate-200">Split bill</Text>
        </Pressable>
      </View>

      {/* Content */}
      {isLoading ? (
        <View className="flex-1 items-center justify-center">
          <ActivityIndicator size="large" color="#0E90A3" />
        </View>
      ) : (
        <FlatList
          data={data as any[]}
          keyExtractor={(item) => item.id}
          refreshing={isRefetching}
          onRefresh={handleRefresh}
          contentContainerStyle={{ paddingHorizontal: 16, paddingBottom: 24, gap: 12 }}
          ListEmptyComponent={
            <EmptyState
              title="Nothing here yet"
              subtitle={activeTab === 'inbox' ? 'You have no money requests to pay.' : 'You have not sent any money requests.'}
            />
          }
          renderItem={({ item }: { item: any }) => {
            const isInbox = activeTab === 'inbox';
            const name = isInbox ? (item as InboxRequest).requester_name : (item as SentRequest).payer_name;
            const status = isInbox ? 'pending' : (item as SentRequest).status;
            const badge = getStatusLabel(status);

            return (
              <Pressable
                onPress={() => router.push({ pathname: '/(main)/requests/detail', params: { id: item.id } })}
                className="bg-white dark:bg-slate-800 rounded-xl p-4 border border-slate-100 dark:border-slate-700 active:bg-slate-50"
              >
                <View className="flex-row items-center">
                  <View className="w-10 h-10 rounded-full bg-cyan-50 items-center justify-center mr-3">
                    <Text className="text-base font-bold text-brand">{name.charAt(0).toUpperCase()}</Text>
                  </View>
                  <View className="flex-1 mr-2">
                    <Text className="text-sm font-semibold text-slate-900 dark:text-white mb-0.5" numberOfLines={1}>{name}</Text>
                    <Text className="text-xs text-slate-400" numberOfLines={1}>
                      {item.note || 'No note'}
                    </Text>
                  </View>
                  <View className="items-end gap-1">
                    <Text className="text-sm font-bold text-slate-900 dark:text-white">{formatAmountVND(BigInt(item.amount))}</Text>
                    <View className={`px-2 py-0.5 rounded-md ${badge.bg}`}>
                      <Text className="text-[10px] font-bold" style={{ color: badge.color }}>{badge.text}</Text>
                    </View>
                  </View>
                </View>
              </Pressable>
            );
          }}
        />
      )}
    </SafeAreaView>
  );
}
