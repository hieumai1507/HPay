import React, { useState } from 'react';
import { Text, View, Pressable, ScrollView, ActivityIndicator, Alert } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useRequestDetail, useDeclineRequest, useCancelRequest, useRemindRequest, useMe } from '@/src/queries';
import { formatAmountUSD } from '@/src/lib/money';
import { color } from '@/src/ui/tokens';
import { SplitAvatarStack } from '@/src/ui';

export default function RequestDetailScreen() {
  const router = useRouter();
  const { id } = useLocalSearchParams<{ id: string }>();

  const meQuery = useMe();
  const detailQuery = useRequestDetail(id || '');

  const declineMutation = useDeclineRequest();
  const cancelMutation = useCancelRequest();
  const remindMutation = useRemindRequest();
  const [remindingSiblingId, setRemindingSiblingId] = useState<string | null>(null);

  const isLoading = detailQuery.isLoading || meQuery.isLoading;

  if (isLoading) {
    return (
      <View className="flex-1 items-center justify-center bg-slate-50 dark:bg-slate-900">
        <ActivityIndicator size="large" color="#0E90A3" />
      </View>
    );
  }

  if (detailQuery.isError || !detailQuery.data || !meQuery.data) {
    return (
      <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
        <View className="flex-row items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
          <Pressable onPress={() => router.back()} className="p-1">
            <Ionicons name="arrow-back" size={24} color="#0f172a" />
          </Pressable>
          <Text className="text-lg font-bold text-slate-900 dark:text-white">Request details</Text>
          <View className="w-8" />
        </View>
        <View className="flex-1 items-center justify-center">
          <Text className="text-base color-rose-600 font-semibold">Could not load request details.</Text>
        </View>
      </SafeAreaView>
    );
  }

  const req = detailQuery.data;
  const myUser = meQuery.data.user;

  const isPayer = req.payer_name === myUser.full_name;
  const isRequester = req.requester_name === myUser.full_name;

  const getStatusLabel = (status: string) => {
    switch (status) {
      case 'pending': return { text: 'Awaiting payment', color: color.warning[600], bg: 'bg-amber-50' };
      case 'paid': return { text: 'Paid', color: color.success[600], bg: 'bg-emerald-50' };
      case 'declined': return { text: 'Rejected', color: color.danger[600], bg: 'bg-rose-50' };
      case 'canceled': return { text: 'Canceled', color: color.neutral[500], bg: 'bg-slate-100' };
      case 'expired': return { text: 'Expired', color: color.neutral[400], bg: 'bg-slate-100' };
      default: return { text: status, color: color.neutral[500], bg: 'bg-slate-100' };
    }
  };

  const badge = getStatusLabel(req.status);

  const handleDecline = () => {
    Alert.alert(
      'Reject request',
      'Are you sure you want to reject this payment request?',
      [
        { text: 'Back', style: 'cancel' },
        {
          text: 'Reject',
          style: 'destructive',
          onPress: async () => {
            try {
              await declineMutation.mutateAsync(req.id);
              Alert.alert('Success', 'Request rejected.');
            } catch (err: any) {
              Alert.alert('Error', err?.message || 'Something went wrong. Please try again.');
            }
          },
        },
      ]
    );
  };

  const handleCancel = () => {
    Alert.alert(
      'Cancel request',
      'Are you sure you want to cancel this money request?',
      [
        { text: 'Back', style: 'cancel' },
        {
          text: 'Cancel request',
          style: 'destructive',
          onPress: async () => {
            try {
              await cancelMutation.mutateAsync(req.id);
              Alert.alert('Success', 'Request canceled.');
            } catch (err: any) {
              Alert.alert('Error', err?.message || 'Something went wrong. Please try again.');
            }
          },
        },
      ]
    );
  };

  const handleRemind = async () => {
    try {
      await remindMutation.mutateAsync(req.id);
      Alert.alert('Success', 'Reminder sent to the payer.');
    } catch (err: any) {
      if (err?.code === 'ERR_REMIND_TOO_SOON') {
        Alert.alert('Reminder sent too recently', 'You can send a reminder once every 24 hours.');
      } else {
        Alert.alert('Error', err?.message || 'Something went wrong. Please try again.');
      }
    }
  };

  const handleRemindSibling = async (siblingId: string, siblingName: string) => {
    setRemindingSiblingId(siblingId);
    try {
      await remindMutation.mutateAsync(siblingId);
      Alert.alert('Success', `Reminder sent to ${siblingName}.`);
    } catch (err: any) {
      if (err?.code === 'ERR_REMIND_TOO_SOON') {
        Alert.alert('Reminder sent too recently', 'You can remind this person once every 24 hours.');
      } else {
        Alert.alert('Error', err?.message || 'Something went wrong. Please try again.');
      }
    } finally {
      setRemindingSiblingId(null);
    }
  };

  const formatDate = (dateStr: string) => {
    if (!dateStr) return 'N/A';
    return new Date(dateStr).toLocaleString('en-GB');
  };

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      {/* Header */}
      <View className="flex-row items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
        <Pressable onPress={() => router.back()} className="p-1">
          <Ionicons name="arrow-back" size={24} color="#0f172a" />
        </Pressable>
        <Text className="text-lg font-bold text-slate-900 dark:text-white">Request details</Text>
        <View className="w-8" />
      </View>

      <ScrollView contentContainerStyle={{ padding: 16, gap: 16 }}>
        <View className="bg-white dark:bg-slate-800 rounded-xl p-5 items-center border border-slate-100 dark:border-slate-700 shadow-xs">
          <Text className="text-3xl font-extrabold text-slate-900 dark:text-white">{formatAmountUSD(BigInt(req.amount))}</Text>
          <View className={`px-3 py-1 rounded-md mt-2 ${badge.bg}`}>
            <Text className="text-xs font-bold" style={{ color: badge.color }}>{badge.text}</Text>
          </View>

          <View className="h-px bg-slate-100 dark:bg-slate-700 w-full my-5" />

          <View className="flex-row justify-between w-full py-2">
            <Text className="text-sm text-slate-400">Description</Text>
            <Text className="text-sm font-semibold text-slate-900 dark:text-white">{req.note || 'No note'}</Text>
          </View>

          <View className="flex-row justify-between w-full py-2">
            <Text className="text-sm text-slate-400">Requester</Text>
            <Text className="text-sm font-semibold text-slate-900 dark:text-white">{req.requester_name}</Text>
          </View>

          <View className="flex-row justify-between w-full py-2">
            <Text className="text-sm text-slate-400">Payer</Text>
            <Text className="text-sm font-semibold text-slate-900 dark:text-white">{req.payer_name}</Text>
          </View>

          <View className="flex-row justify-between w-full py-2">
            <Text className="text-sm text-slate-400">Expires at</Text>
            <Text className="text-sm font-semibold text-slate-900 dark:text-white">{formatDate(req.expires_at)}</Text>
          </View>

          {req.last_reminded_at ? (
            <View className="flex-row justify-between w-full py-2">
              <Text className="text-sm text-slate-400">Last reminder</Text>
              <Text className="text-sm font-semibold text-slate-900 dark:text-white">{formatDate(req.last_reminded_at)}</Text>
            </View>
          ) : null}
        </View>

        {req.siblings && req.siblings.length > 0 ? (
          <>
            <SplitAvatarStack
              participants={req.siblings.map((sib) => ({ id: sib.id, name: sib.payer_name, paid: sib.status === 'paid' }))}
              collectedAmount={req.siblings
                .filter((sib) => sib.status === 'paid')
                .reduce((sum, sib) => sum + BigInt(sib.amount), 0n)}
              totalAmount={req.siblings.reduce((sum, sib) => sum + BigInt(sib.amount), 0n)}
              expiresAt={req.expires_at}
            />

            <View className="bg-white dark:bg-slate-800 rounded-xl p-5 border border-slate-100 dark:border-slate-700 shadow-xs">
              <Text className="text-sm font-bold text-slate-900 dark:text-white self-start mb-4">Split bill members</Text>
              {req.siblings.map((sib) => {
                const sibBadge = getStatusLabel(sib.status);
                const canRemind = isRequester && sib.status === 'pending';
                return (
                  <View key={sib.id} className="flex-row items-center justify-between w-full py-3 border-b border-slate-100 dark:border-slate-700">
                    <View className="gap-1">
                      <Text className="text-sm font-semibold text-slate-900 dark:text-white">{sib.payer_name}</Text>
                      <Text className="text-xs text-slate-500">{formatAmountUSD(BigInt(sib.amount))}</Text>
                    </View>
                    {canRemind ? (
                      <Pressable
                        onPress={() => handleRemindSibling(sib.id, sib.payer_name)}
                        disabled={remindingSiblingId === sib.id}
                        className="px-2.5 py-1 rounded-md bg-cyan-50 flex-row items-center gap-1"
                      >
                        {remindingSiblingId === sib.id ? (
                          <ActivityIndicator size="small" color={color.brand[600]} />
                        ) : (
                          <>
                            <Ionicons name="notifications-outline" size={12} color={color.brand[600]} />
                            <Text className="text-xs font-bold text-brand">Remind</Text>
                          </>
                        )}
                      </Pressable>
                    ) : (
                      <View className={`px-2.5 py-1 rounded-md ${sibBadge.bg}`}>
                        <Text className="text-xs font-bold" style={{ color: sibBadge.color }}>{sibBadge.text}</Text>
                      </View>
                    )}
                  </View>
                );
              })}
            </View>
          </>
        ) : null}
      </ScrollView>

      {req.status === 'pending' ? (
        <View className="p-4 bg-white dark:bg-slate-800 border-t border-slate-100 dark:border-slate-700">
          {isPayer ? (
            <View className="flex-row gap-3">
              <Pressable
                onPress={handleDecline}
                disabled={declineMutation.isPending}
                className="flex-1 h-12 rounded-xl bg-rose-50 items-center justify-center active:opacity-70"
              >
                <Text className="color-rose-600 text-sm font-bold">Reject</Text>
              </Pressable>
              <Pressable
                onPress={() => router.push({ pathname: '/(main)/requests/pay', params: { id: req.id } })}
                className="flex-2 h-12 rounded-xl bg-brand items-center justify-center active:bg-brand-600"
              >
                <Text className="text-white text-sm font-bold">Pay</Text>
              </Pressable>
            </View>
          ) : isRequester ? (
            <View className="flex-row gap-3">
              <Pressable
                onPress={handleCancel}
                disabled={cancelMutation.isPending}
                className="flex-1 h-12 rounded-xl bg-slate-100 items-center justify-center active:bg-slate-200"
              >
                <Text className="text-slate-600 text-sm font-bold">Cancel request</Text>
              </Pressable>
              <Pressable
                onPress={handleRemind}
                disabled={remindMutation.isPending}
                className="flex-2 h-12 rounded-xl bg-brand items-center justify-center active:bg-brand-600"
              >
                {remindMutation.isPending ? (
                  <ActivityIndicator color="#FFFFFF" />
                ) : (
                  <Text className="text-white text-sm font-bold">Remind</Text>
                )}
              </Pressable>
            </View>
          ) : null}
        </View>
      ) : null}
    </SafeAreaView>
  );
}
