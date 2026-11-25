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
import { withIntentKey } from '@/src/lib/intentKey';
import { formatAmountUSD, formatAmountWhileTyping, parseAmountStrict } from '@/src/lib/money';
import { useSubmitGuard } from '@/src/lib/useSubmitGuard';
import { useOrgStore } from '@/src/stores/org';

export default function DisburseScreen() {
  const activeOrg = useOrgStore((s) => s.activeOrg);
  const disburseMutation = useDisburseOrgMoney();
  const guard = useSubmitGuard();

  const [phone, setPhone] = useState('');
  const [amount, setAmount] = useState('');
  const [note, setNote] = useState('');

  if (!activeOrg || activeOrg.role !== 'maker') {
    return (
      <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
        <View className="flex-1 items-center justify-center gap-4">
          <Text className="text-base color-rose-600 font-semibold text-center">Only a Maker can create a disbursement order.</Text>
          <Pressable onPress={() => router.back()} className="px-5 py-2.5 bg-brand rounded-xl">
            <Text className="text-white font-bold">Back</Text>
          </Pressable>
        </View>
      </SafeAreaView>
    );
  }

  const parsed = parseAmountStrict(amount);

  const handleSend = () =>
    guard.run(async () => {
      if (!phone.trim() || !parsed.ok) {
        Alert.alert('Missing information', 'Please enter a phone number and a valid amount.');
        return;
      }
      const orgId = activeOrg.orgId;
      const recipientPhone = phone.trim();
      const trimmedNote = note.trim();
      try {
        // One key per disbursement intent: a retry after a timeout replays the same key.
        const res = await withIntentKey(
          `disburse:${orgId}:${recipientPhone}:${parsed.value}:${trimmedNote}`,
          (idempotencyKey) =>
            disburseMutation.mutateAsync({
              orgId,
              recipientPhone,
              amount: parsed.value,
              note: trimmedNote || undefined,
              idempotencyKey,
            }),
        );
        router.replace({ pathname: '/(main)/org/order-status', params: { refCode: res.ref_code } });
      } catch (e: unknown) {
        Alert.alert('Error', (e instanceof Error ? e.message : String(e)));
      }
    });

  const handleAmountChange = (raw: string) => {
    setAmount(formatAmountWhileTyping(raw));
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
            Create disbursement order · {activeOrg.name}
          </Text>
          <View className="w-5" />
        </View>

        <ScrollView contentContainerStyle={{ padding: 16, gap: 20, paddingBottom: 40 }} keyboardShouldPersistTaps="handled">
          <View className="bg-brand rounded-2xl p-5 flex-row items-center justify-between shadow-xs">
            <View>
              <Text className="text-xs font-semibold text-white/80 mb-1">Business wallet</Text>
              <Text className="text-2xl font-bold text-white">{formatAmountUSD(BigInt(activeOrg.balance))}</Text>
            </View>
            <View className="bg-white/20 px-2.5 py-1 rounded-full">
              <Text className="text-[11px] font-bold text-white">MAKER</Text>
            </View>
          </View>

          <View>
            <Text className="text-xs font-bold text-slate-500 tracking-wider mb-1.5 uppercase">Chi cho</Text>
            <TextInput
              className="h-12 bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 px-4 text-base text-slate-900 dark:text-white"
              placeholder="Phone number or partner name…"
              placeholderTextColor="#94a3b8"
              value={phone}
              onChangeText={setPhone}
              keyboardType="phone-pad"
              autoComplete="tel"
            />
          </View>

          <View>
            <Text className="text-xs font-bold text-slate-500 tracking-wider mb-1.5 uppercase">Amount</Text>
            <View className="h-16 bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 px-4 flex-row items-center">
              <TextInput
                className="flex-1 text-2xl font-bold text-slate-900 dark:text-white h-full"
                placeholder="0"
                placeholderTextColor="#94a3b8"
                value={amount}
                onChangeText={handleAmountChange}
                keyboardType="decimal-pad"
              />
              <Text className="text-base text-slate-500 ml-2">USD</Text>
            </View>
          </View>

          <View>
            <Text className="text-xs font-bold text-slate-500 tracking-wider mb-1.5 uppercase">Payment note</Text>
            <TextInput
              className="h-12 bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 px-4 text-base text-slate-900 dark:text-white"
              placeholder="E.g. Payment for contract #HD-001"
              placeholderTextColor="#94a3b8"
              value={note}
              onChangeText={setNote}
            />
          </View>

          <View className="bg-cyan-50 dark:bg-slate-800 rounded-xl p-4 flex-row items-start gap-2.5">
            <Text className="text-lg">🛃</Text>
            <Text className="flex-1 text-xs text-slate-700 dark:text-slate-300 leading-relaxed">
              The order will go through approval under the policy of {activeOrg.name} before it is executed.
            </Text>
          </View>

          <Pressable
            onPress={handleSend}
            disabled={guard.isPending}
            className={`h-13 bg-brand rounded-xl items-center justify-center mt-2 active:bg-brand-600 ${guard.isPending ? 'opacity-70' : ''}`}
          >
            {guard.isPending ? (
              <ActivityIndicator color="#fff" />
            ) : (
              <Text className="text-base font-bold text-white">Submit for approval</Text>
            )}
          </Pressable>
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
