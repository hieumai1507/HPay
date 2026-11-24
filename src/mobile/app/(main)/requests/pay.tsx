import React from 'react';
import { Text, View, Pressable, ActivityIndicator, Alert } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useRequestDetail, usePayRequest, useFeePreview, useMainWallet } from '@/src/queries';
import { FEATURE_AVAILABILITY } from '@/src/lib/availability';
import { withIntentKey } from '@/src/lib/intentKey';
import { formatAmountUSD } from '@/src/lib/money';
import { useSubmitGuard } from '@/src/lib/useSubmitGuard';
import { NotAvailable } from '@/src/ui';

/**
 * BACKEND GAP: POST /v1/money-requests/:id/pay is not mounted (docs/reference/api.md).
 * Paying through the engine (service MONEY_REQUEST) moves the money but never marks the
 * request as paid, so it is not an equivalent. Gated by one constant until the backend
 * mounts the route.
 */
export default function PayRequestScreen() {
  if (!FEATURE_AVAILABILITY.payRequest) return <NotAvailable />;
  return <PayRequestForm />;
}

function PayRequestForm() {
  const router = useRouter();
  const { id } = useLocalSearchParams<{ id: string }>();

  const detailQuery = useRequestDetail(id || '');
  const { wallet } = useMainWallet();

  const amountStr = detailQuery.data ? detailQuery.data.amount.toString() : null;
  const feeQuery = useFeePreview(amountStr);
  const payMutation = usePayRequest();

  const guard = useSubmitGuard();

  const isLoading = detailQuery.isLoading || feeQuery.isLoading;

  if (isLoading) {
    return (
      <View className="flex-1 items-center justify-center bg-slate-50 dark:bg-slate-900">
        <ActivityIndicator size="large" color="#0E90A3" />
      </View>
    );
  }

  if (detailQuery.isError || !detailQuery.data || !wallet) {
    return (
      <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
        <View className="flex-row items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
          <Pressable onPress={() => router.back()} className="p-1">
            <Ionicons name="arrow-back" size={24} color="#0f172a" />
          </Pressable>
          <Text className="text-lg font-bold text-slate-900 dark:text-white">Pay money request</Text>
          <View className="w-8" />
        </View>
        <View className="flex-1 items-center justify-center">
          <Text className="text-base color-rose-600 font-semibold">Could not load payment details.</Text>
        </View>
      </SafeAreaView>
    );
  }

  const req = detailQuery.data;
  const feeData = feeQuery.data;

  const walletBalanceBig = BigInt(wallet.balance);
  const totalAmountBig = feeData ? BigInt(feeData.total) : BigInt(req.amount);
  const hasInsufficientFunds = walletBalanceBig < totalAmountBig;

  const handlePay = () =>
    guard.run(async () => {
      if (hasInsufficientFunds) {
        Alert.alert('Insufficient balance', 'Your Wallet balance is not enough to pay this request.');
        return;
      }

      try {
        const res = await withIntentKey(`pay:${req.id}`, (idempotencyKey) =>
          payMutation.mutateAsync({ id: req.id, idempotencyKey }),
        );

        router.replace({
          pathname: '/(main)/transfer/processing',
          params: { refCode: res.ref_code, amount: req.amount.toString() },
        });
      } catch (err: unknown) {
        Alert.alert('Error', err instanceof Error && err.message ? err.message : 'Something went wrong while making the payment.');
      }
    });

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      {/* Header */}
      <View className="flex-row items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
        <Pressable onPress={() => router.back()} className="p-1">
          <Ionicons name="arrow-back" size={24} color="#0f172a" />
        </Pressable>
        <Text className="text-lg font-bold text-slate-900 dark:text-white">Pay money request</Text>
        <View className="w-8" />
      </View>

      <View className="flex-1 p-4 gap-4">
        {/* Payee Info */}
        <View className="bg-white dark:bg-slate-800 rounded-xl p-5 border border-slate-100 dark:border-slate-700 shadow-xs">
          <Text className="text-sm font-bold text-slate-900 dark:text-white mb-4">Recipient details</Text>
          <View className="flex-row items-center gap-3">
            <View className="w-12 h-12 rounded-full bg-cyan-50 items-center justify-center">
              <Text className="text-xl font-bold text-brand">{req.requester_name?.charAt(0).toUpperCase()}</Text>
            </View>
            <View>
              <Text className="text-base font-bold text-slate-900 dark:text-white mb-0.5">{req.requester_name}</Text>
              <Text className="text-xs text-slate-500">{req.note || 'No note'}</Text>
            </View>
          </View>
        </View>

        {/* Financial Breakdown */}
        <View className="bg-white dark:bg-slate-800 rounded-xl p-5 border border-slate-100 dark:border-slate-700 shadow-xs">
          <Text className="text-sm font-bold text-slate-900 dark:text-white mb-4">Transaction details</Text>
          
          <View className="flex-row justify-between items-center w-full">
            <Text className="text-sm text-slate-600 dark:text-slate-400">Source</Text>
            <Text className="text-sm font-semibold text-slate-900 dark:text-white">HPay Wallet (USD)</Text>
          </View>
          <View className="flex-row justify-between items-center w-full">
            <Text className="text-xs text-slate-400 mt-0.5">Available balance: {formatAmountUSD(walletBalanceBig)}</Text>
          </View>

          <View className="h-px bg-slate-100 dark:bg-slate-700 w-full my-3" />

          <View className="flex-row justify-between items-center w-full">
            <Text className="text-sm text-slate-600 dark:text-slate-400">Amount</Text>
            <Text className="text-sm font-semibold text-slate-900 dark:text-white">{formatAmountUSD(BigInt(req.amount))}</Text>
          </View>

          <View className="flex-row justify-between items-center w-full mt-2">
            <Text className="text-sm text-slate-600 dark:text-slate-400">Fee</Text>
            <Text className="text-sm font-semibold text-slate-900 dark:text-white">
              {feeData ? formatAmountUSD(BigInt(feeData.fee)) : 'Calculating...'}
            </Text>
          </View>

          <View className="h-px bg-slate-100 dark:bg-slate-700 w-full my-3" />

          <View className="flex-row justify-between items-center w-full">
            <Text className="text-base font-bold text-slate-900 dark:text-white">Total</Text>
            <Text className="text-lg font-extrabold text-brand">
              {feeData ? formatAmountUSD(BigInt(feeData.total)) : formatAmountUSD(BigInt(req.amount))}
            </Text>
          </View>
        </View>

        {hasInsufficientFunds ? (
          <View className="flex-row items-center bg-rose-50 rounded-xl p-3 gap-2 border border-rose-100">
            <Ionicons name="warning-outline" size={20} color="#dc2626" />
            <Text className="flex-1 text-xs color-rose-600 font-semibold">Your balance is not enough for this Transaction.</Text>
          </View>
        ) : null}
      </View>

      <View className="p-4 bg-white dark:bg-slate-800 border-t border-slate-100 dark:border-slate-700">
        <Pressable
          onPress={handlePay}
          disabled={guard.isPending || hasInsufficientFunds}
          className={`w-full h-12 rounded-xl bg-brand items-center justify-center active:bg-brand-600 ${
            guard.isPending || hasInsufficientFunds ? 'opacity-60' : ''
          }`}
        >
          {guard.isPending ? (
            <ActivityIndicator color="#FFFFFF" />
          ) : (
            <Text className="text-white text-base font-bold">Confirm & pay</Text>
          )}
        </Pressable>
      </View>
    </SafeAreaView>
  );
}
