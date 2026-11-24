import { router } from 'expo-router';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { SECURITY_ROUTES } from '@/src/features/security/routes';
import { describeError } from '@/src/lib/errors';
import { useTotpStatus } from '@/src/queries';
import { Button } from '@/src/ui';

export const LOW_RECOVERY_CODES = 3;

export default function TotpStatusScreen() {
  const q = useTotpStatus();
  const data = q.data;
  const low = !!data?.enabled && data.recovery_codes_left <= LOW_RECOVERY_CODES;

  return (
    <SafeAreaView className="flex-1 bg-white dark:bg-slate-900" edges={['top']}>
      <View className="flex-row items-center justify-between px-4 py-2 border-b border-slate-100 dark:border-slate-800">
        <Pressable onPress={() => router.back()} hitSlop={12} accessibilityRole="button" accessibilityLabel="Back">
          <Text className="text-xl font-bold text-slate-700 dark:text-slate-200 w-8">←</Text>
        </Pressable>
        <Text className="text-base font-semibold text-slate-900 dark:text-white">Two-step verification</Text>
        <View className="w-8" />
      </View>
      <View className="p-4 gap-3">
        {q.isLoading ? <ActivityIndicator accessibilityLabel="Loading" /> : null}
        {q.isError ? <Text className="text-sm color-rose-600">{describeError(q.error)}</Text> : null}
        {data ? (
          <>
            <Text className="text-base font-semibold text-slate-900 dark:text-white">
              {data.enabled ? 'On' : 'Off'}
            </Text>
            {data.enabled ? (
              <Text className="text-sm text-slate-600 dark:text-slate-300">
                Recovery codes left: {data.recovery_codes_left}
              </Text>
            ) : (
              <Button
                label="Turn on two-step verification"
                accessibilityLabel="Turn on two-step verification"
                onPress={() => router.push(SECURITY_ROUTES.totpEnroll as never)}
              />
            )}
            {low ? (
              <Text accessibilityRole="alert" className="text-sm text-amber-700">
                {data.recovery_codes_left === 0
                  ? 'You have no recovery codes left. Contact support if you lose your authenticator.'
                  : 'You are running low on recovery codes. Contact support if you lose your authenticator.'}
              </Text>
            ) : null}
          </>
        ) : null}
      </View>
    </SafeAreaView>
  );
}