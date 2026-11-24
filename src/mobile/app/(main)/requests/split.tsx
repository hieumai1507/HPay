import React, { useState } from 'react';
import { Text, View, TextInput, Pressable, ActivityIndicator, Alert, ScrollView, KeyboardAvoidingView, Platform } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useCreateSplit } from '@/src/queries';
import { splitEven, validateCustomSplit } from '@/src/lib/split';
import { useIdempotencyKey } from '@/src/lib/idempotency';
import { formatAmountWhileTyping, parseAmountToBigInt, formatAmountVND } from '@/src/lib/money';

type SplitMode = 'even' | 'custom';

export default function SplitBillScreen() {
  const router = useRouter();
  const splitMutation = useCreateSplit();

  const [phones, setPhones] = useState<string[]>(['', '']);
  const [totalAmountStr, setTotalAmountStr] = useState('');
  const [note, setNote] = useState('');
  const [splitMode, setSplitMode] = useState<SplitMode>('even');
  
  const [customAmounts, setCustomAmounts] = useState<string[]>(['', '']);
  const [errors, setErrors] = useState<{ phones?: string[]; total?: string; customSum?: string }>({});

  const idempotencyKey = useIdempotencyKey(`split:${phones.join(',')}:${totalAmountStr}:${splitMode}`);

  const handleAddParticipant = () => {
    setPhones([...phones, '']);
    setCustomAmounts([...customAmounts, '']);
    setErrors({});
  };

  const handleRemoveParticipant = (index: number) => {
    if (phones.length <= 1) return;
    const newPhones = [...phones];
    newPhones.splice(index, 1);
    setPhones(newPhones);

    const newCustoms = [...customAmounts];
    newCustoms.splice(index, 1);
    setCustomAmounts(newCustoms);
    setErrors({});
  };

  const handlePhoneChange = (text: string, index: number) => {
    const newPhones = [...phones];
    newPhones[index] = text;
    setPhones(newPhones);
    setErrors({});
  };

  const handleCustomAmountChange = (text: string, index: number) => {
    const newCustoms = [...customAmounts];
    newCustoms[index] = formatAmountWhileTyping(text);
    setCustomAmounts(newCustoms);
    setErrors({});
  };

  const totalAmountVal = parseAmountToBigInt(totalAmountStr) ?? 0n;

  const evenSplitShares = totalAmountVal > 0n && phones.length > 0
    ? splitEven(totalAmountVal, phones.length)
    : [];

  const customAmountsBigInts = customAmounts.map((str) => parseAmountToBigInt(str) ?? 0n);
  const customSum = customAmountsBigInts.reduce((acc, val) => acc + val, 0n);

  const handleSubmit = async () => {
    let hasErr = false;
    const newErrors: typeof errors = {};

    if (totalAmountVal <= 0n) {
      newErrors.total = 'Enter a valid total amount';
      hasErr = true;
    }

    const phoneErrors = phones.map((p) => {
      const trimmed = p.trim();
      if (!trimmed) return 'Enter a phone number';
      if (!/^0\d{9}$/.test(trimmed)) return 'Invalid phone number';
      return '';
    });
    if (phoneErrors.some((e) => e !== '')) {
      newErrors.phones = phoneErrors;
      hasErr = true;
    }

    if (splitMode === 'custom') {
      const isCustomValid = validateCustomSplit(totalAmountVal, customAmountsBigInts);
      if (!isCustomValid) {
        newErrors.customSum = `The custom amounts (${formatAmountVND(customSum)}) must exactly match the bill total (${formatAmountVND(totalAmountVal)})`;
        hasErr = true;
      }
    }

    if (hasErr) {
      setErrors(newErrors);
      return;
    }

    try {
      await splitMutation.mutateAsync({
        payerPhones: phones.map((p) => p.trim()),
        totalAmount: totalAmountVal,
        note: note.trim() || undefined,
        amounts: splitMode === 'custom' ? customAmountsBigInts : undefined,
        idempotencyKey,
      });

      Alert.alert(
        'Success',
        `Split bill created for a total of ${formatAmountVND(totalAmountVal)}`,
        [{ text: 'OK', onPress: () => router.replace('/(main)/requests') }]
      );
    } catch (err: any) {
      Alert.alert('Error', err?.message || 'Something went wrong. Please try again.');
    }
  };

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      <View className="flex-row items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
        <Pressable onPress={() => router.back()} className="p-1">
          <Ionicons name="close" size={24} color="#0f172a" />
        </Pressable>
        <Text className="text-lg font-bold text-slate-900 dark:text-white">Split bill</Text>
        <View className="w-8" />
      </View>

      <KeyboardAvoidingView
        behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
        className="flex-1"
      >
        <ScrollView contentContainerStyle={{ padding: 16, gap: 16 }}>
          <View className="bg-white dark:bg-slate-800 rounded-xl p-5 border border-slate-100 dark:border-slate-700 gap-4 shadow-xs">
            <View className="w-full">
              <Text className="text-xs font-bold text-slate-700 dark:text-slate-200 mb-2">Bill total</Text>
              <View className={`flex-row items-center bg-slate-50 dark:bg-slate-900 border ${errors.total ? 'border-rose-500' : 'border-slate-200 dark:border-slate-700'} rounded-xl h-12 px-3`}>
                <Text className="text-lg font-bold text-slate-400 mr-2">₫</Text>
                <TextInput
                  placeholder="0"
                  placeholderTextColor="#94a3b8"
                  keyboardType="number-pad"
                  value={totalAmountStr}
                  onChangeText={(t) => {
                    setTotalAmountStr(formatAmountWhileTyping(t));
                    setErrors({});
                  }}
                  className="flex-1 text-lg font-bold text-brand h-full p-0"
                />
              </View>
              {errors.total ? <Text className="color-rose-600 text-xs mt-1">{errors.total}</Text> : null}
            </View>

            <View className="w-full">
              <Text className="text-xs font-bold text-slate-700 dark:text-slate-200 mb-2">Note / Occasion</Text>
              <View className="flex-row items-center bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl h-12 px-3">
                <Ionicons name="receipt-outline" size={20} color="#94a3b8" className="mr-2" />
                <TextInput
                  placeholder="e.g. Weekend hotpot, taxi fare..."
                  placeholderTextColor="#94a3b8"
                  maxLength={255}
                  value={note}
                  onChangeText={setNote}
                  className="flex-1 text-sm text-slate-900 dark:text-white h-full p-0"
                />
              </View>
            </View>
          </View>

          {/* Mode Selector */}
          <View className="flex-row bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 p-1">
            <Pressable
              onPress={() => setSplitMode('even')}
              className={`flex-1 py-2 items-center rounded-lg ${splitMode === 'even' ? 'bg-cyan-50' : ''}`}
            >
              <Text className={`text-sm font-semibold ${splitMode === 'even' ? 'text-brand' : 'text-slate-500'}`}>
                Split evenly
              </Text>
            </Pressable>
            <Pressable
              onPress={() => setSplitMode('custom')}
              className={`flex-1 py-2 items-center rounded-lg ${splitMode === 'custom' ? 'bg-cyan-50' : ''}`}
            >
              <Text className={`text-sm font-semibold ${splitMode === 'custom' ? 'text-brand' : 'text-slate-500'}`}>
                Custom
              </Text>
            </Pressable>
          </View>

          {/* Participants */}
          <View className="bg-white dark:bg-slate-800 rounded-xl p-5 border border-slate-100 dark:border-slate-700 gap-4 shadow-xs">
            <View className="flex-row justify-between items-center w-full">
              <Text className="text-sm font-bold text-slate-900 dark:text-white">Participants</Text>
              <Pressable onPress={handleAddParticipant} className="flex-row items-center gap-1 p-1">
                <Ionicons name="person-add" size={16} color="#0E90A3" />
                <Text className="text-xs font-bold text-brand">Add person</Text>
              </Pressable>
            </View>

            {phones.map((phoneVal, index) => {
              const phoneErr = errors.phones?.[index];
              const share = splitMode === 'even' ? evenSplitShares[index] : null;

              return (
                <View key={index} className="w-full">
                  <View className="flex-row items-start w-full gap-2">
                    <View className="flex-[3]">
                      <TextInput
                        placeholder={`Phone number of person ${index + 1}`}
                        placeholderTextColor="#94a3b8"
                        keyboardType="phone-pad"
                        maxLength={10}
                        value={phoneVal}
                        onChangeText={(t) => handlePhoneChange(t, index)}
                        className={`h-10 bg-slate-50 dark:bg-slate-900 border ${phoneErr ? 'border-rose-500' : 'border-slate-200 dark:border-slate-700'} rounded-xl px-3 text-xs text-slate-900 dark:text-white`}
                      />
                      {phoneErr ? <Text className="color-rose-600 text-[11px] mt-1">{phoneErr}</Text> : null}
                    </View>

                    {splitMode === 'even' ? (
                      <View className="flex-[2] h-10 justify-center items-end">
                        <Text className="text-sm font-bold text-brand">
                          {share ? formatAmountVND(share as bigint) : '0 ₫'}
                        </Text>
                      </View>
                    ) : (
                      <View className="flex-[2]">
                        <TextInput
                          placeholder="Amount"
                          placeholderTextColor="#94a3b8"
                          keyboardType="number-pad"
                          value={customAmounts[index]}
                          onChangeText={(t) => handleCustomAmountChange(t, index)}
                          className="h-10 bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 text-xs font-semibold text-brand text-right"
                        />
                      </View>
                    )}

                    {phones.length > 1 ? (
                      <Pressable onPress={() => handleRemoveParticipant(index)} className="w-8 h-10 items-center justify-center">
                        <Ionicons name="trash-outline" size={20} color="#dc2626" />
                      </Pressable>
                    ) : (
                      <View className="w-8" />
                    )}
                  </View>
                </View>
              );
            })}

            {splitMode === 'custom' && errors.customSum ? (
              <View className="flex-row items-center bg-rose-50 rounded-xl p-3 gap-2 border border-rose-100 w-full">
                <Ionicons name="alert-circle-outline" size={18} color="#dc2626" />
                <Text className="flex-1 text-xs color-rose-600 font-semibold">{errors.customSum}</Text>
              </View>
            ) : null}
          </View>
        </ScrollView>

        <View className="p-4 bg-white dark:bg-slate-800 border-t border-slate-100 dark:border-slate-700">
          <Pressable
            onPress={handleSubmit}
            disabled={splitMutation.isPending}
            className={`w-full h-12 rounded-xl bg-brand items-center justify-center active:bg-brand-600 ${splitMutation.isPending ? 'opacity-60' : ''}`}
          >
            {splitMutation.isPending ? (
              <ActivityIndicator color="#FFFFFF" />
            ) : (
              <Text className="text-white text-base font-bold">Create split bill</Text>
            )}
          </Pressable>
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
