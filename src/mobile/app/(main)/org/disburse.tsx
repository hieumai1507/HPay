import { Ionicons } from '@expo/vector-icons';
import { router } from 'expo-router';
import { useState } from 'react';
import {
  ActivityIndicator,
  Alert,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ScrollView,
  Text,
  TextInput,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useDisburseOrgMoney } from '@/src/queries';
import { generateIdempotencyKey } from '@/src/lib/idempotency';
import { formatAmountVND } from '@/src/lib/money';
import { useOrgStore } from '@/src/stores/org';

export default function DisburseScreen() {
  const activeOrg = useOrgStore((s) => s.activeOrg);
  const disburseMutation = useDisburseOrgMoney();

  const [phone, setPhone] = useState('');
  const [amount, setAmount] = useState('');
  const [note, setNote] = useState('');

  if (!activeOrg || activeOrg.role !== 'maker') {
    return (
      <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
        <View className="flex-1 items-center justify-center gap-4">
          <Text className="text-base color-rose-600 font-semibold text-center">Chỉ Maker mới có thể tạo lệnh chi.</Text>
          <Pressable onPress={() => router.back()} className="px-5 py-2.5 bg-brand rounded-xl">
            <Text className="text-white font-bold">Quay lại</Text>
          </Pressable>
        </View>
      </SafeAreaView>
    );
  }

  const amountNum = parseInt(amount.replace(/\./g, ''), 10) || 0;

  const handleSend = async () => {
    if (!phone.trim() || amountNum <= 0) {
      Alert.alert('Thiếu thông tin', 'Vui lòng nhập đầy đủ SĐT và số tiền hợp lệ.');
      return;
    }
    try {
      const res = await disburseMutation.mutateAsync({
        orgId: activeOrg.orgId,
        recipientPhone: phone.trim(),
        amount: amountNum,
        note: note.trim() || undefined,
        idempotencyKey: generateIdempotencyKey(),
      });
      router.replace({ pathname: '/(main)/org/order-status', params: { refCode: res.ref_code } });
    } catch (e: unknown) {
      Alert.alert('Lỗi', (e instanceof Error ? e.message : String(e)));
    }
  };

  const handleAmountChange = (raw: string) => {
    const digits = raw.replace(/\D/g, '');
    const formatted = digits ? parseInt(digits, 10).toLocaleString('vi-VN') : '';
    setAmount(formatted);
  };

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      <KeyboardAvoidingView
        className="flex-1"
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
      >
        <View className="flex-row items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
          <Pressable onPress={() => router.back()} hitSlop={12}>
            <Ionicons name="arrow-back" size={22} color="#0f172a" />
          </Pressable>
          <Text className="text-base font-bold text-slate-900 dark:text-white flex-1 text-center mx-2" numberOfLines={1}>
            Tạo lệnh chi · {activeOrg.name}
          </Text>
          <View className="w-5" />
        </View>

        <ScrollView contentContainerStyle={{ padding: 16, gap: 20, paddingBottom: 40 }} keyboardShouldPersistTaps="handled">
          <View className="bg-brand rounded-2xl p-5 flex-row items-center justify-between shadow-xs">
            <View>
              <Text className="text-xs font-semibold text-white/80 mb-1">Ví doanh nghiệp</Text>
              <Text className="text-2xl font-bold text-white">{formatAmountVND(BigInt(activeOrg.balance))}</Text>
            </View>
            <View className="bg-white/20 px-2.5 py-1 rounded-full">
              <Text className="text-[11px] font-bold text-white">MAKER</Text>
            </View>
          </View>

          <View>
            <Text className="text-xs font-bold text-slate-500 tracking-wider mb-1.5 uppercase">Chi cho</Text>
            <TextInput
              className="h-12 bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 px-4 text-base text-slate-900 dark:text-white"
              placeholder="SĐT hoặc tên đối tác…"
              placeholderTextColor="#94a3b8"
              value={phone}
              onChangeText={setPhone}
              keyboardType="phone-pad"
              autoComplete="tel"
            />
          </View>

          <View>
            <Text className="text-xs font-bold text-slate-500 tracking-wider mb-1.5 uppercase">Số tiền</Text>
            <View className="h-16 bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 px-4 flex-row items-center">
              <TextInput
                className="flex-1 text-2xl font-bold text-slate-900 dark:text-white h-full"
                placeholder="0"
                placeholderTextColor="#94a3b8"
                value={amount}
                onChangeText={handleAmountChange}
                keyboardType="numeric"
              />
              <Text className="text-base text-slate-500 ml-2">₫</Text>
            </View>
          </View>

          <View>
            <Text className="text-xs font-bold text-slate-500 tracking-wider mb-1.5 uppercase">Nội dung chi</Text>
            <TextInput
              className="h-12 bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 px-4 text-base text-slate-900 dark:text-white"
              placeholder="Vd: Thanh toán hợp đồng #HD-001"
              placeholderTextColor="#94a3b8"
              value={note}
              onChangeText={setNote}
            />
          </View>

          <View className="bg-cyan-50 dark:bg-slate-800 rounded-xl p-4 flex-row items-start gap-2.5">
            <Text className="text-lg">🛃</Text>
            <Text className="flex-1 text-xs text-slate-700 dark:text-slate-300 leading-relaxed">
              Lệnh chi sẽ được phê duyệt theo chính sách của {activeOrg.name} trước khi thực hiện.
            </Text>
          </View>

          <Pressable
            onPress={handleSend}
            disabled={disburseMutation.isPending}
            className={`h-13 bg-brand rounded-xl items-center justify-center mt-2 active:bg-brand-600 ${disburseMutation.isPending ? 'opacity-70' : ''}`}
          >
            {disburseMutation.isPending ? (
              <ActivityIndicator color="#fff" />
            ) : (
              <Text className="text-base font-bold text-white">Gửi lệnh chờ duyệt</Text>
            )}
          </Pressable>
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
