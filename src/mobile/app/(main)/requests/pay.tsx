import React from 'react';
import { Text, View, Pressable, ActivityIndicator, Alert } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useRequestDetail, usePayRequest, useFeePreview, useMainWallet } from '@/src/queries';
import { useIdempotencyKey } from '@/src/lib/idempotency';
import { formatAmountVND } from '@/src/lib/money';

export default function PayRequestScreen() {
  const router = useRouter();
  const { id } = useLocalSearchParams<{ id: string }>();

  const detailQuery = useRequestDetail(id || '');
  const { wallet } = useMainWallet();

  const amountStr = detailQuery.data ? detailQuery.data.amount.toString() : null;
  const feeQuery = useFeePreview(amountStr);
  const payMutation = usePayRequest();

  const idempotencyKey = useIdempotencyKey(`pay:${id}`);

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
          <Text className="text-lg font-bold text-slate-900 dark:text-white">Thanh toán yêu cầu</Text>
          <View className="w-8" />
        </View>
        <View className="flex-1 items-center justify-center">
          <Text className="text-base color-rose-600 font-semibold">Không thể tải thông tin thanh toán.</Text>
        </View>
      </SafeAreaView>
    );
  }

  const req = detailQuery.data;
  const feeData = feeQuery.data;

  const walletBalanceBig = BigInt(wallet.balance);
  const totalAmountBig = feeData ? BigInt(feeData.total) : BigInt(req.amount);
  const hasInsufficientFunds = walletBalanceBig < totalAmountBig;

  const handlePay = async () => {
    if (hasInsufficientFunds) {
      Alert.alert('Không đủ số dư', 'Số dư ví của bạn không đủ để thanh toán yêu cầu này.');
      return;
    }

    try {
      const res = await payMutation.mutateAsync({
        id: req.id,
        idempotencyKey,
      });

      router.replace({
        pathname: '/(main)/transfer/processing',
        params: { refCode: res.ref_code, amount: req.amount.toString() },
      });
    } catch (err: any) {
      Alert.alert('Lỗi', err?.message || 'Có lỗi xảy ra khi thực hiện thanh toán.');
    }
  };

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      {/* Header */}
      <View className="flex-row items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
        <Pressable onPress={() => router.back()} className="p-1">
          <Ionicons name="arrow-back" size={24} color="#0f172a" />
        </Pressable>
        <Text className="text-lg font-bold text-slate-900 dark:text-white">Thanh toán yêu cầu</Text>
        <View className="w-8" />
      </View>

      <View className="flex-1 p-4 gap-4">
        {/* Payee Info */}
        <View className="bg-white dark:bg-slate-800 rounded-xl p-5 border border-slate-100 dark:border-slate-700 shadow-xs">
          <Text className="text-sm font-bold text-slate-900 dark:text-white mb-4">Thông tin người nhận</Text>
          <View className="flex-row items-center gap-3">
            <View className="w-12 h-12 rounded-full bg-cyan-50 items-center justify-center">
              <Text className="text-xl font-bold text-brand">{req.requester_name?.charAt(0).toUpperCase()}</Text>
            </View>
            <View>
              <Text className="text-base font-bold text-slate-900 dark:text-white mb-0.5">{req.requester_name}</Text>
              <Text className="text-xs text-slate-500">{req.note || 'Không có ghi chú'}</Text>
            </View>
          </View>
        </View>

        {/* Financial Breakdown */}
        <View className="bg-white dark:bg-slate-800 rounded-xl p-5 border border-slate-100 dark:border-slate-700 shadow-xs">
          <Text className="text-sm font-bold text-slate-900 dark:text-white mb-4">Chi tiết giao dịch</Text>
          
          <View className="flex-row justify-between items-center w-full">
            <Text className="text-sm text-slate-600 dark:text-slate-400">Nguồn tiền</Text>
            <Text className="text-sm font-semibold text-slate-900 dark:text-white">Ví HPay VND</Text>
          </View>
          <View className="flex-row justify-between items-center w-full">
            <Text className="text-xs text-slate-400 mt-0.5">Số dư khả dụng: {formatAmountVND(walletBalanceBig)}</Text>
          </View>

          <View className="h-px bg-slate-100 dark:bg-slate-700 w-full my-3" />

          <View className="flex-row justify-between items-center w-full">
            <Text className="text-sm text-slate-600 dark:text-slate-400">Số tiền thanh toán</Text>
            <Text className="text-sm font-semibold text-slate-900 dark:text-white">{formatAmountVND(BigInt(req.amount))}</Text>
          </View>

          <View className="flex-row justify-between items-center w-full mt-2">
            <Text className="text-sm text-slate-600 dark:text-slate-400">Phí giao dịch</Text>
            <Text className="text-sm font-semibold text-slate-900 dark:text-white">
              {feeData ? formatAmountVND(BigInt(feeData.fee)) : 'Đang tính...'}
            </Text>
          </View>

          <View className="h-px bg-slate-100 dark:bg-slate-700 w-full my-3" />

          <View className="flex-row justify-between items-center w-full">
            <Text className="text-base font-bold text-slate-900 dark:text-white">Tổng cộng</Text>
            <Text className="text-lg font-extrabold text-brand">
              {feeData ? formatAmountVND(BigInt(feeData.total)) : formatAmountVND(BigInt(req.amount))}
            </Text>
          </View>
        </View>

        {hasInsufficientFunds ? (
          <View className="flex-row items-center bg-rose-50 rounded-xl p-3 gap-2 border border-rose-100">
            <Ionicons name="warning-outline" size={20} color="#dc2626" />
            <Text className="flex-1 text-xs color-rose-600 font-semibold">Số dư tài khoản không đủ để thực hiện giao dịch.</Text>
          </View>
        ) : null}
      </View>

      <View className="p-4 bg-white dark:bg-slate-800 border-t border-slate-100 dark:border-slate-700">
        <Pressable
          onPress={handlePay}
          disabled={payMutation.isPending || hasInsufficientFunds}
          className={`w-full h-12 rounded-xl bg-brand items-center justify-center active:bg-brand-600 ${
            payMutation.isPending || hasInsufficientFunds ? 'opacity-60' : ''
          }`}
        >
          {payMutation.isPending ? (
            <ActivityIndicator color="#FFFFFF" />
          ) : (
            <Text className="text-white text-base font-bold">Xác nhận & Thanh toán</Text>
          )}
        </Pressable>
      </View>
    </SafeAreaView>
  );
}
