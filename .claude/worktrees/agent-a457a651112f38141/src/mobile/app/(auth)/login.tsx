import { zodResolver } from '@hookform/resolvers/zod';
import { useRouter } from 'expo-router';
import React, { useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import {
  Text,
  TextInput,
  View,
  Pressable,
  ScrollView,
  KeyboardAvoidingView,
  Platform,
  Alert,
} from 'react-native';
import { apiRequest } from '@/src/api/client';
import { describeError } from '@/src/lib/errors';
import { storeRefreshToken, useSessionStore } from '@/src/stores/session';
import { useSecurityStore } from '@/src/stores/security';
import { Button } from '@/src/ui/Button';
import { loginSchema, type LoginFormData } from '@/src/features/auth/schemas/auth-validation';

import type { components } from '@/src/api/schema';

type AuthResponse = components['schemas']['AuthResponse'];

export default function LoginScreen() {
  const router = useRouter();
  const [secureText, setSecureText] = useState(true);
  const [loading, setLoading] = useState(false);

  const logoutReason = useSessionStore((s) => s.logoutReason);

  const {
    control,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      phone: '',
      password: '',
    },
  });

  const onSubmit = async (data: LoginFormData) => {
    setLoading(true);
    try {
      const formattedPhone = data.phone.startsWith('0') ? data.phone : '0' + data.phone;
      const response = await apiRequest<AuthResponse>('/v1/auth/login', {
        method: 'POST',
        body: {
          phone: formattedPhone,
          password: data.password,
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
        Alert.alert('Lỗi', 'Không nhận được mã xác thực từ server.');
      }
    } catch (err: any) {
      Alert.alert('Đăng nhập thất bại', describeError(err));
    } finally {
      setLoading(false);
    }
  };

  return (
    <KeyboardAvoidingView
      className="flex-1 bg-slate-50 dark:bg-slate-900"
      behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
    >
      <ScrollView contentContainerStyle={{ flexGrow: 1, paddingHorizontal: 24, paddingBottom: 40, justifyContent: 'space-between' }} keyboardShouldPersistTaps="handled">
        <View>
          <Pressable onPress={() => router.replace('/(auth)/onboarding')} className="w-10 h-10 rounded-full bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 items-center justify-center mt-5 mb-6" accessibilityLabel="Back">
            <Text className="text-xl font-bold text-slate-900 dark:text-white">←</Text>
          </Pressable>

          <View className="mb-8">
            <Text className="text-2xl font-bold text-slate-900 dark:text-white mb-2">Đăng nhập</Text>
            <Text className="text-sm text-slate-500">Chào mừng bạn quay lại với HPay</Text>
          </View>

          {logoutReason && (
            <View
              className={`flex-row rounded-xl p-4 items-center gap-3 mb-6 ${
                logoutReason === 'refresh_revoked' ? 'bg-rose-50' : 'bg-amber-50'
              }`}
            >
              <Text className="text-xl">
                {logoutReason === 'refresh_revoked' ? '🚨' : '🔑'}
              </Text>
              <Text className="flex-1 text-xs leading-relaxed text-slate-700">
                {logoutReason === 'refresh_revoked'
                  ? 'Phát hiện đăng nhập bất thường từ thiết bị khác. Vui lòng đăng nhập lại để bảo mật tài khoản.'
                  : 'Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.'}
              </Text>
            </View>
          )}

          <View className="gap-5">
            <View className="gap-2">
              <Text className="text-sm font-semibold text-slate-700 dark:text-slate-200">Số điện thoại</Text>
              <View className={`h-12 flex-row items-center rounded-lg border-1.5 ${errors.phone ? 'border-rose-600' : 'border-slate-300 dark:border-slate-700'} bg-white dark:bg-slate-800 px-4`}>
                <Text className="text-sm font-semibold text-slate-700 dark:text-slate-200">+84</Text>
                <View className="w-px h-5 bg-slate-200 dark:bg-slate-700 mx-3" />
                <Controller
                  control={control}
                  name="phone"
                  render={({ field: { onChange, onBlur, value } }) => (
                    <TextInput
                      placeholder="912345678"
                      placeholderTextColor="#94a3b8"
                      keyboardType="phone-pad"
                      maxLength={10}
                      onBlur={onBlur}
                      onChangeText={onChange}
                      value={value}
                      className="flex-1 h-full text-sm text-slate-900 dark:text-white"
                    />
                  )}
                />
              </View>
              {errors.phone && <Text className="text-xs color-rose-600 font-medium">{errors.phone.message}</Text>}
            </View>

            <View className="gap-2">
              <Text className="text-sm font-semibold text-slate-700 dark:text-slate-200">Mật khẩu</Text>
              <View className={`h-12 flex-row items-center rounded-lg border-1.5 ${errors.password ? 'border-rose-600' : 'border-slate-300 dark:border-slate-700'} bg-white dark:bg-slate-800 pl-4`}>
                <Controller
                  control={control}
                  name="password"
                  render={({ field: { onChange, onBlur, value } }) => (
                    <TextInput
                      placeholder="Nhập mật khẩu của bạn"
                      placeholderTextColor="#94a3b8"
                      secureTextEntry={secureText}
                      onBlur={onBlur}
                      onChangeText={onChange}
                      value={value}
                      className="flex-1 h-full text-sm text-slate-900 dark:text-white"
                    />
                  )}
                />
                <Pressable
                  onPress={() => setSecureText(!secureText)}
                  className="p-3 items-center justify-center"
                  accessibilityLabel="Toggle password visibility"
                >
                  <Text className="text-base">{secureText ? '👁️' : '🙈'}</Text>
                </Pressable>
              </View>
              {errors.password && <Text className="text-xs color-rose-600 font-medium">{errors.password.message}</Text>}
            </View>
          </View>
        </View>

        <View className="mt-10 gap-4">
          <Button
            label="Đăng nhập"
            loading={loading}
            onPress={handleSubmit(onSubmit)}
            className="w-full"
          />
          <Pressable onPress={() => router.push('/(auth)/register')} className="items-center py-2">
            <Text className="text-xs text-slate-600 dark:text-slate-400">
              Chưa có tài khoản? <Text className="text-brand font-bold">Đăng ký ngay</Text>
            </Text>
          </Pressable>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
