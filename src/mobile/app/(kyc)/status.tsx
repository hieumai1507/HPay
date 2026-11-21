import { useRouter } from 'expo-router';
import React, { useState, useEffect } from 'react';
import { Text, View, ActivityIndicator, Alert, ScrollView } from 'react-native';

import { useKYCStatus } from '@/src/queries';
import { useSessionStore } from '@/src/stores/session';
import { Button } from '@/src/ui/Button';
import { StatusBadge } from '@/src/ui/StatusBadge';

import type { components } from '@/src/api/schema';

export default function KycStatusScreen() {
  const router = useRouter();
  const user = useSessionStore((s) => s.user);
  const accessToken = useSessionStore((s) => s.accessToken);

  const { data, isLoading, isError, refetch } = useKYCStatus();

  const kycStatus = data?.kyc_status ?? 'pending';
  const reasons = data?.latest_request?.ai_reasons || null;

  useEffect(() => {
    if (data?.kyc_status && user && accessToken && user.kyc_status !== data.kyc_status) {
      useSessionStore.getState().setSession(accessToken, {
        ...user,
        kyc_status: data.kyc_status,
      });
    }
  }, [data, user, accessToken]);

  const handleStartApp = () => {
    router.replace('/(main)/home');
  };

  const handleRetryKyc = () => {
    router.replace('/(kyc)/intro');
  };

  if (isLoading) {
    return (
      <View className="flex-1 bg-slate-50 dark:bg-slate-900 justify-center items-center gap-3">
        <ActivityIndicator size="large" color="#0E90A3" />
        <Text className="color-slate-600 text-sm">Đang tải thông tin...</Text>
      </View>
    );
  }

  let icon = '🔎';
  let title = 'AI đang kiểm tra hồ sơ';
  let desc = 'Thường mất dưới 1 phút. Bạn có thể tiếp tục dùng app, chúng tôi sẽ thông báo ngay khi xong.';
  let badgeStatus: 'processing' | 'completed' | 'pending_approval' | 'failed' = 'processing';
  let showButton = true;
  let btnLabel = 'Trở về trang chủ';
  let btnAction = handleStartApp;
  let btnVariant: 'primary' | 'secondary' = 'secondary';

  if (kycStatus === 'verified' || kycStatus === 'auto_approved') {
    icon = '✅';
    title = 'Xác minh thành công';
    desc = 'Tài khoản của bạn đã được nâng hạn mức lên 100.000.000 ₫/ngày.';
    badgeStatus = 'completed';
    btnLabel = 'Bắt đầu sử dụng ví';
    btnAction = handleStartApp;
    btnVariant = 'primary';
  } else if (kycStatus === 'manual_review') {
    icon = '👩‍💼';
    title = 'Cần kiểm tra thêm';
    desc = 'Hồ sơ được chuyển cho chuyên viên xét duyệt. Dự kiến phản hồi trong 1–2 giờ làm việc.';
    badgeStatus = 'pending_approval';
    btnLabel = 'Trở về trang chủ';
    btnAction = handleStartApp;
    btnVariant = 'secondary';
  } else if (kycStatus === 'rejected') {
    icon = '❌';
    title = 'Xác minh bị từ chối';
    desc = reasons
      ? `Hồ sơ của bạn không được chấp nhận. Lý do: ${reasons}. Vui lòng thực hiện xác minh lại.`
      : 'Hồ sơ hoặc hình ảnh của bạn không hợp lệ. Vui lòng thực hiện xác minh lại.';
    badgeStatus = 'failed';
    btnLabel = 'Xác minh lại';
    btnAction = handleRetryKyc;
    btnVariant = 'primary';
  }

  return (
    <ScrollView contentContainerStyle={{ flexGrow: 1, paddingHorizontal: 24, paddingBottom: 40, justifyContent: 'space-between' }} className="bg-slate-50 dark:bg-slate-900">
      <View className="flex-1 justify-center items-center mt-10">
        <View className="bg-white dark:bg-slate-800 rounded-2xl p-6 items-center w-full border border-slate-200 dark:border-slate-700 shadow-sm">
          <Text className="text-6xl mb-4">{icon}</Text>
          <Text className="text-xl font-bold text-slate-900 dark:text-white text-center mb-2">{title}</Text>
          <Text className="text-xs leading-relaxed text-slate-500 text-center mb-5">{desc}</Text>
          
          <View className="mt-2">
            <StatusBadge status={badgeStatus} />
          </View>
        </View>
      </View>

      {showButton && (
        <View className="w-full">
          <Button label={btnLabel} variant={btnVariant} onPress={btnAction} className="w-full" />
        </View>
      )}
    </ScrollView>
  );
}
