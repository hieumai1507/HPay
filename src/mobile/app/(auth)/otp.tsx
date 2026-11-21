import { useSecureScreen } from '@/src/features/security/useSecureScreen';
import { useLocalSearchParams, useRouter } from 'expo-router';
import React, { useState, useEffect } from 'react';
import { Text, View, Pressable, ScrollView, Alert } from 'react-native';

import { apiRequest } from '@/src/api/client';
import { OTPInput } from '@/src/features/auth/components/OTPInput';
import { describeError } from '@/src/lib/errors';
import { storeRefreshToken, useSessionStore } from '@/src/stores/session';
import { useSecurityStore } from '@/src/stores/security';
import { Button } from '@/src/ui/Button';

import type { components } from '@/src/api/schema';

type AuthResponse = components['schemas']['AuthResponse'];

function formatTime(seconds: number): string {
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
}

function maskPhoneNumber(phoneStr: string): string {
  if (!phoneStr) return '';
  const clean = phoneStr.replace(/^\+84/, '0');
  if (clean.length >= 10) {
    const part1 = clean.substring(1, 4);
    const part2 = clean.substring(7);
    return `+84 ${part1} ••• ${part2}`;
  }
  return phoneStr;
}

export default function OTPScreen() {
  useSecureScreen();
  const router = useRouter();
  const params = useLocalSearchParams();
  
  const phone = (params.phone as string) || '';
  const password = (params.password as string) || '';
  const fullName = (params.fullName as string) || '';
  const mode = (params.mode as string) || 'register';

  const [otp, setOtp] = useState('');
  const [countdown, setCountdown] = useState(120);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (countdown === 0) return;
    const timer = setInterval(() => {
      setCountdown((prev) => prev - 1);
    }, 1000);
    return () => clearInterval(timer);
  }, [countdown]);

  const handleResend = () => {
    setCountdown(120);
    setOtp('');
    Alert.alert('Thông báo', 'Mã xác thực mới đã được gửi.');
  };

  const handleVerify = async () => {
    if (otp.length < 6) return;

    setLoading(true);
    try {
      if (mode === 'register') {
        const response = await apiRequest<AuthResponse>('/v1/auth/register', {
          method: 'POST',
          body: {
            phone,
            password,
            full_name: fullName,
          },
          skipAuth: true,
        });

        if (response.tokens?.access_token && response.tokens?.refresh_token) {
          await storeRefreshToken(response.tokens.refresh_token);
          useSessionStore.getState().setSession(response.tokens.access_token, response.user);

          const pendingRoute = useSecurityStore.getState().pendingRoute;
          if (pendingRoute) {
            useSecurityStore.getState().setPendingRoute(null);
            router.replace(pendingRoute as any);
          } else if (response.user?.kyc_status === 'none') {
            router.replace('/(kyc)/intro');
          } else {
            router.replace('/(main)/home');
          }
        } else {
          Alert.alert('Lỗi', 'Không nhận được mã xác thực hoặc mã phiên hoạt động từ server.');
        }
      } else {
        Alert.alert('Thành công', 'Mã OTP chính xác.');
        router.back();
      }
    } catch (err: any) {
      Alert.alert('Đăng ký thất bại', describeError(err));
    } finally {
      setLoading(false);
    }
  };

  return (
    <ScrollView contentContainerStyle={{ flexGrow: 1, paddingHorizontal: 24, paddingBottom: 40, justifyContent: 'space-between' }} className="bg-slate-50 dark:bg-slate-900" keyboardShouldPersistTaps="handled">
      <View>
        <Pressable onPress={() => router.back()} className="w-10 h-10 rounded-full bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 items-center justify-center mt-5 mb-6" accessibilityLabel="Back">
          <Text className="text-xl font-bold text-slate-900 dark:text-white">←</Text>
        </Pressable>

        <View className="mb-8">
          <Text className="text-2xl font-bold text-slate-900 dark:text-white mb-2">Nhập mã xác thực</Text>
          <Text className="text-sm text-slate-500">
            Mã 6 số đã gửi tới <Text className="font-semibold text-slate-900 dark:text-white">{maskPhoneNumber(phone)}</Text>
          </Text>
        </View>

        <View className="my-5">
          <OTPInput value={otp} onChange={setOtp} disabled={loading} />
        </View>

        <View className="items-center my-3">
          {countdown > 0 ? (
            <Text className="text-xs text-slate-500">
              Gửi lại mã sau <Text className="font-bold text-slate-900 dark:text-white">{formatTime(countdown)}</Text>
            </Text>
          ) : (
            <Pressable onPress={handleResend} className="py-1.5 px-3">
              <Text className="text-sm font-semibold text-brand">Gửi lại mã xác thực</Text>
            </Pressable>
          )}
        </View>

        <View className="flex-row bg-cyan-50 dark:bg-slate-800 rounded-xl p-4 items-center gap-3 my-5 border border-cyan-100 dark:border-slate-700">
          <Text className="text-xl">🛡️</Text>
          <Text className="flex-1 text-xs leading-relaxed text-slate-700 dark:text-slate-300">
            HPay không bao giờ gọi điện hỏi mã OTP. Tuyệt đối không chia sẻ mã với bất kỳ ai.
          </Text>
        </View>
      </View>

      <View className="mt-auto">
        <Button
          label="Xác nhận"
          disabled={otp.length < 6}
          loading={loading}
          onPress={handleVerify}
          className="w-full"
        />
      </View>
    </ScrollView>
  );
}
