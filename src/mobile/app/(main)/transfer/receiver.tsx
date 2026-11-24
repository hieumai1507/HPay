import { router } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useLookupReceiver } from '@/src/queries';
import { useRecentContacts } from '@/src/features/wallet/recent-contacts';
import { describeError, isApiError } from '@/src/lib/errors';
import { Button } from '@/src/ui';

const PHONE_RE = /^0\d{9}$/;

export default function TransferReceiverScreen() {
  const [phone, setPhone] = useState('');
  const [maskedName, setMaskedName] = useState<string | null>(null);
  const [lookupError, setLookupError] = useState<string | null>(null);
  const { mutate: lookupReceiver, isPending: lookupPending } = useLookupReceiver();
  const recents = useRecentContacts((s) => s.contacts);
  const debounce = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    setMaskedName(null);
    setLookupError(null);
    if (debounce.current) clearTimeout(debounce.current);
    if (!PHONE_RE.test(phone)) return;
    debounce.current = setTimeout(() => {
      lookupReceiver(phone, {
        onSuccess: (res) => setMaskedName(res.masked_name),
        onError: (err) => {
          if (isApiError(err) && err.code === 'ERR_USER_NOT_FOUND') {
            setLookupError('Không tìm thấy người nhận với số này.');
          } else {
            setLookupError(describeError(err));
          }
        },
      });
    }, 500);
    return () => {
      if (debounce.current) clearTimeout(debounce.current);
    };
  }, [phone, lookupReceiver]);

  const proceed = (p: string, name: string) =>
    router.push({
      pathname: '/(main)/transfer/amount',
      params: { phone: p, maskedName: name },
    });

  return (
    <SafeAreaView className="flex-1 bg-white dark:bg-slate-900" edges={['top']}>
      <View className="flex-row items-center justify-between px-4 py-2 border-b border-slate-100 dark:border-slate-800">
        <Pressable onPress={() => router.back()} hitSlop={12} accessibilityRole="button">
          <Text className="text-xl font-bold text-slate-700 dark:text-slate-200 w-8">←</Text>
        </Pressable>
        <Text className="text-base font-semibold text-slate-900 dark:text-white">Chuyển tiền</Text>
        <View className="w-8" />
      </View>

      <View className="flex-1 p-4 gap-3">
        <Text className="text-xs font-semibold text-slate-600 dark:text-slate-400">Số điện thoại người nhận</Text>
        <TextInput
          className="border border-slate-200 dark:border-slate-700 rounded-xl p-3 text-lg font-bold text-slate-900 dark:text-white bg-white dark:bg-slate-800"
          value={phone}
          onChangeText={(t) => setPhone(t.replace(/\D/g, '').slice(0, 10))}
          keyboardType="number-pad"
          placeholder="09xxxxxxxx"
          placeholderTextColor="#94a3b8"
          autoFocus
          accessibilityLabel="Số điện thoại người nhận"
        />

        {lookupPending && <Text className="text-xs text-slate-400">Đang tìm…</Text>}
        {maskedName && (
          <View className="bg-cyan-50 dark:bg-slate-800 rounded-xl p-3 gap-0.5 border border-cyan-100 dark:border-slate-700">
            <Text className="text-sm font-semibold text-slate-900 dark:text-white">{maskedName}</Text>
            <Text className="text-xs text-slate-500">{phone}</Text>
          </View>
        )}
        {lookupError && <Text className="text-xs color-rose-600 font-semibold">{lookupError}</Text>}

        {recents.length > 0 && (
          <View className="mt-4 gap-2">
            <Text className="text-xs font-semibold text-slate-600 dark:text-slate-400">Gần đây</Text>
            {recents.map((c) => (
              <Pressable
                key={c.phone}
                className="border border-slate-200 dark:border-slate-700 rounded-xl p-3 bg-white dark:bg-slate-800 active:bg-slate-100"
                onPress={() => proceed(c.phone, c.maskedName)}
                accessibilityRole="button"
              >
                <Text className="text-sm font-semibold text-slate-900 dark:text-white">{c.maskedName}</Text>
                <Text className="text-xs text-slate-500">{c.phone}</Text>
              </Pressable>
            ))}
          </View>
        )}
      </View>

      <View className="p-4">
        <Button
          label="Tiếp tục"
          disabled={!maskedName}
          onPress={() => maskedName && proceed(phone, maskedName)}
        />
      </View>
    </SafeAreaView>
  );
}
