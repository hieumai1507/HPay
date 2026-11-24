import { useRouter } from 'expo-router';
import React, { useEffect } from 'react';
import { Text, View, ActivityIndicator, ScrollView } from 'react-native';

import { useKYCStatus } from '@/src/queries';
import { useSessionStore } from '@/src/stores/session';
import { Button } from '@/src/ui/Button';
import { StatusBadge } from '@/src/ui/StatusBadge';

export default function KycStatusScreen() {
  const router = useRouter();
  const user = useSessionStore((s) => s.user);
  const accessToken = useSessionStore((s) => s.accessToken);

  const { data, isLoading } = useKYCStatus();

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
        <Text className="color-slate-600 text-sm">Loading...</Text>
      </View>
    );
  }

  let icon = '🔎';
  let title = 'AI is reviewing your documents';
  let desc = 'This usually takes under a minute. You can keep using the app and we will notify you as soon as it is done.';
  let badgeStatus: 'processing' | 'completed' | 'pending_approval' | 'failed' = 'processing';
  let showButton = true;
  let btnLabel = 'Back to home';
  let btnAction = handleStartApp;
  let btnVariant: 'primary' | 'secondary' = 'secondary';

  if (kycStatus === 'verified' || kycStatus === 'auto_approved') {
    icon = '✅';
    title = 'Verification successful';
    desc = 'Your Account Limit has been raised to $400.00 per day.';
    badgeStatus = 'completed';
    btnLabel = 'Start using your Wallet';
    btnAction = handleStartApp;
    btnVariant = 'primary';
  } else if (kycStatus === 'manual_review') {
    icon = '👩‍💼';
    title = 'Additional review needed';
    desc = 'Your submission has been sent to a reviewer. Expect a response within 1–2 business hours.';
    badgeStatus = 'pending_approval';
    btnLabel = 'Back to home';
    btnAction = handleStartApp;
    btnVariant = 'secondary';
  } else if (kycStatus === 'rejected') {
    icon = '❌';
    title = 'Verification rejected';
    desc = reasons
      ? `Your submission was not accepted. Reason: ${reasons}. Please verify again.`
      : 'Your submission or photos are invalid. Please verify again.';
    badgeStatus = 'failed';
    btnLabel = 'Verify again';
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
