import React, { useState } from 'react';
import { Text, View, TextInput, Pressable, ActivityIndicator, Alert, KeyboardAvoidingView, Platform, ScrollView } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useCreateRequest } from '@/src/queries';
import { useIdempotencyKey } from '@/src/lib/idempotency';
import { formatAmountWhileTyping, parseAmountToBigInt, formatAmountVND } from '@/src/lib/money';

export default function CreateRequestScreen() {
  const router = useRouter();
  const createMutation = useCreateRequest();

  const [phone, setPhone] = useState('');
  const [amountStr, setAmountStr] = useState('');
  const [note, setNote] = useState('');

  const [phoneError, setPhoneError] = useState('');
  const [amountError, setAmountError] = useState('');

  const idempotencyKey = useIdempotencyKey(`request:${phone}:${amountStr}`);

  const handleAmountChange = (text: string) => {
    setAmountStr(formatAmountWhileTyping(text));
    setAmountError('');
  };

  const handleSubmit = async () => {
    let hasErr = false;

    const phoneClean = phone.trim();
    if (!phoneClean) {
      setPhoneError('Vui lòng nhập số điện thoại');
      hasErr = true;
    } else if (!/^0\d{9}$/.test(phoneClean)) {
      setPhoneError('Số điện thoại không hợp lệ (10 số, bắt đầu bằng 0)');
      hasErr = true;
    } else {
      setPhoneError('');
    }

    const amountVal = parseAmountToBigInt(amountStr);
    if (!amountVal || amountVal <= 0n) {
      setAmountError('Vui lòng nhập số tiền hợp lệ');
      hasErr = true;
    } else {
      setAmountError('');
    }

    if (hasErr) return;

    try {
      await createMutation.mutateAsync({
        payerPhone: phoneClean,
        amount: amountVal!,
        note: note.trim() || undefined,
        idempotencyKey,
      });

      Alert.alert(
        'Thành công',
        `Đã gửi yêu cầu thanh toán ${formatAmountVND(amountVal!)} tới ${phoneClean}`,
        [{ text: 'OK', onPress: () => router.replace('/(main)/requests') }]
      );
    } catch (err: any) {
      Alert.alert('Lỗi', err?.message || 'Có lỗi xảy ra khi gửi yêu cầu.');
    }
  };

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      <View className="flex-row items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
        <Pressable onPress={() => router.back()} className="p-1">
          <Ionicons name="close" size={24} color="#0f172a" />
        </Pressable>
        <Text className="text-lg font-bold text-slate-900 dark:text-white">Yêu cầu xin tiền</Text>
        <View className="w-8" />
      </View>

      <KeyboardAvoidingView
        behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
        className="flex-1"
      >
        <ScrollView contentContainerStyle={{ padding: 16 }}>
          <View className="bg-white dark:bg-slate-800 rounded-xl p-5 border border-slate-100 dark:border-slate-700 gap-4 shadow-xs">
            <View className="w-full">
              <Text className="text-xs font-bold text-slate-700 dark:text-slate-200 mb-2">Người nhận yêu cầu (Payer)</Text>
              <View className={`flex-row items-center bg-slate-50 dark:bg-slate-900 border ${phoneError ? 'border-rose-500' : 'border-slate-200 dark:border-slate-700'} rounded-xl h-12 px-3`}>
                <Ionicons name="call-outline" size={20} color="#94a3b8" className="mr-2" />
                <TextInput
                  placeholder="Nhập số điện thoại"
                  placeholderTextColor="#94a3b8"
                  keyboardType="phone-pad"
                  maxLength={10}
                  value={phone}
                  onChangeText={(t) => {
                    setPhone(t);
                    setPhoneError('');
                  }}
                  className="flex-1 text-sm text-slate-900 dark:text-white h-full p-0"
                />
              </View>
              {phoneError ? <Text className="color-rose-600 text-xs mt-1">{phoneError}</Text> : null}
            </View>

            <View className="w-full">
              <Text className="text-xs font-bold text-slate-700 dark:text-slate-200 mb-2">Số tiền yêu cầu (VND)</Text>
              <View className={`flex-row items-center bg-slate-50 dark:bg-slate-900 border ${amountError ? 'border-rose-500' : 'border-slate-200 dark:border-slate-700'} rounded-xl h-12 px-3`}>
                <Text className="text-lg font-bold text-slate-400 mr-2">đ</Text>
                <TextInput
                  placeholder="0"
                  placeholderTextColor="#94a3b8"
                  keyboardType="number-pad"
                  value={amountStr}
                  onChangeText={handleAmountChange}
                  className="flex-1 text-lg font-bold text-brand h-full p-0"
                />
              </View>
              {amountError ? <Text className="color-rose-600 text-xs mt-1">{amountError}</Text> : null}
            </View>

            <View className="w-full">
              <Text className="text-xs font-bold text-slate-700 dark:text-slate-200 mb-2">Nội dung lời nhắn</Text>
              <View className="flex-row items-center bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl h-12 px-3">
                <Ionicons name="chatbubble-ellipses-outline" size={20} color="#94a3b8" className="mr-2" />
                <TextInput
                  placeholder="Ví dụ: Tiền ăn trưa hôm nay..."
                  placeholderTextColor="#94a3b8"
                  maxLength={255}
                  value={note}
                  onChangeText={setNote}
                  className="flex-1 text-sm text-slate-900 dark:text-white h-full p-0"
                />
              </View>
            </View>
          </View>
        </ScrollView>

        <View className="p-4 bg-white dark:bg-slate-800 border-t border-slate-100 dark:border-slate-700">
          <Pressable
            onPress={handleSubmit}
            disabled={createMutation.isPending}
            className={`w-full h-12 rounded-xl bg-brand items-center justify-center active:bg-brand-600 ${createMutation.isPending ? 'opacity-60' : ''}`}
          >
            {createMutation.isPending ? (
              <ActivityIndicator color="#FFFFFF" />
            ) : (
              <Text className="text-white text-base font-bold">Gửi yêu cầu</Text>
            )}
          </Pressable>
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
