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

import { apiRequest } from '@/src/api/client';
import { withIntentKey } from '@/src/lib/intentKey';
import { parseAmountStrict } from '@/src/lib/money';
import { useSubmitGuard } from '@/src/lib/useSubmitGuard';
import { useOrgStore } from '@/src/stores/org';

type Tab = 'members' | 'policy';

function MembersTab({ orgId }: { orgId: string }) {
  const [phone, setPhone] = useState('');
  const [role, setRole] = useState<'maker' | 'checker'>('maker');
  const [level, setLevel] = useState(1);
  const [loading, setLoading] = useState(false);

  const handleAdd = async () => {
    if (!phone.trim()) {
      Alert.alert('Thiếu thông tin', 'Vui lòng nhập số điện thoại.');
      return;
    }
    try {
      setLoading(true);
      await apiRequest(`/v1/org/${orgId}/members`, {
        method: 'POST',
        body: JSON.stringify({
          phone: phone.trim(),
          role,
          checker_level: role === 'checker' ? level : 0,
        }),
      });
      setPhone('');
      Alert.alert('Thành công', 'Đã thêm thành viên vào doanh nghiệp.');
    } catch (e: unknown) {
      Alert.alert('Lỗi', (e instanceof Error ? e.message : String(e)));
    } finally {
      setLoading(false);
    }
  };

  return (
    <View className="gap-4">
      <View>
        <Text className="text-xs font-bold text-slate-500 tracking-wider mb-1.5 uppercase">Số điện thoại thành viên</Text>
        <TextInput
          className="h-12 bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 px-4 text-base text-slate-900 dark:text-white"
          placeholder="Vd: 0912345678"
          placeholderTextColor="#94a3b8"
          value={phone}
          onChangeText={setPhone}
          keyboardType="phone-pad"
        />
      </View>

      <View>
        <Text className="text-xs font-bold text-slate-500 tracking-wider mb-1.5 uppercase">Vai trò</Text>
        <View className="flex-row gap-2">
          {(['maker', 'checker'] as const).map((r) => (
            <Pressable
              key={r}
              onPress={() => setRole(r)}
              className={`flex-1 h-11 rounded-xl border-1.5 items-center justify-center ${role === r ? 'border-brand bg-cyan-50' : 'border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800'}`}
            >
              <Text className={`text-sm font-semibold ${role === r ? 'text-brand' : 'text-slate-500'}`}>
                {r === 'maker' ? 'Maker' : 'Checker'}
              </Text>
            </Pressable>
          ))}
        </View>
      </View>

      {role === 'checker' && (
        <View>
          <Text className="text-xs font-bold text-slate-500 tracking-wider mb-1.5 uppercase">Cấp Checker</Text>
          <View className="flex-row gap-2">
            {[1, 2].map((l) => (
              <Pressable
                key={l}
                onPress={() => setLevel(l)}
                className={`flex-1 h-11 rounded-xl border-1.5 items-center justify-center ${level === l ? 'border-brand bg-cyan-50' : 'border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800'}`}
              >
                <Text className={`text-sm font-semibold ${level === l ? 'text-brand' : 'text-slate-500'}`}>
                  Level {l}
                </Text>
              </Pressable>
            ))}
          </View>
        </View>
      )}

      <Pressable
        onPress={handleAdd}
        disabled={loading}
        className={`h-13 bg-brand rounded-xl items-center justify-center mt-2 active:bg-brand-600 ${loading ? 'opacity-70' : ''}`}
      >
        {loading ? <ActivityIndicator color="#fff" /> : <Text className="text-base font-bold text-white">Thêm thành viên</Text>}
      </Pressable>
    </View>
  );
}

interface PolicyTier {
  level: number;
  min_amount: string;
  required_approvals: number;
}

function PolicyTab({ orgId }: { orgId: string }) {
  const [tiers, setTiers] = useState<PolicyTier[]>([
    { level: 1, min_amount: '10000000', required_approvals: 1 },
  ]);
  const guard = useSubmitGuard();

  const addTier = () => {
    setTiers((t) => [...t, { level: t.length + 1, min_amount: '', required_approvals: 1 }]);
  };
  const removeTier = (i: number) => {
    setTiers((t) => t.filter((_, idx) => idx !== i).map((tier, idx) => ({ ...tier, level: idx + 1 })));
  };
  const updateTier = (i: number, field: keyof PolicyTier, value: string | number) => {
    setTiers((t) => t.map((tier, idx) => idx === i ? { ...tier, [field]: value } : tier));
  };

  const handleSave = () =>
    guard.run(async () => {
      // Tier thresholds are money: parse with BigInt and bound them (C-01). Empty means 0.
      const parsedTiers: { level: number; min_amount: bigint; required_approvals: number }[] = [];
      for (const t of tiers) {
        if (t.min_amount.trim() === '') {
          parsedTiers.push({ level: t.level, min_amount: 0n, required_approvals: t.required_approvals });
          continue;
        }
        const parsed = parseAmountStrict(t.min_amount);
        if (!parsed.ok) {
          Alert.alert('Thiếu thông tin', `Mức tiền của Level ${t.level} không hợp lệ.`);
          return;
        }
        parsedTiers.push({ level: t.level, min_amount: parsed.value, required_approvals: t.required_approvals });
      }
      const payload = { name: 'default', min_amount: 0, max_amount: null, tiers: parsedTiers };
      try {
        await withIntentKey(`policy:${orgId}:${JSON.stringify(tiers)}`, (idempotencyKey) =>
          apiRequest(`/v1/org/${orgId}/policies`, {
            method: 'POST',
            idempotencyKey,
            body: payload,
          }),
        );
        Alert.alert('Đã lưu', 'Chính sách phê duyệt mới đã có hiệu lực ngay lập tức.');
      } catch (e: unknown) {
        Alert.alert('Lỗi', (e instanceof Error ? e.message : String(e)));
      }
    });

  return (
    <View className="gap-4">
      <Text className="text-xs text-slate-600 dark:text-slate-300 leading-relaxed bg-cyan-50 dark:bg-slate-800 p-3.5 rounded-xl border border-cyan-100 dark:border-slate-700">
        Mỗi tier là một cấp độ phê duyệt. Các lệnh chi từ mức tiền tương ứng sẽ cần đủ chữ ký của cấp đó.
      </Text>
      {tiers.map((tier, i) => (
        <View key={i} className="bg-white dark:bg-slate-800 rounded-2xl p-4 gap-3 border border-slate-100 dark:border-slate-700 shadow-xs">
          <View className="flex-row items-center justify-between">
            <Text className="text-sm font-bold text-slate-900 dark:text-white">Level {tier.level}</Text>
            {tiers.length > 1 && (
              <Pressable onPress={() => removeTier(i)} hitSlop={8}>
                <Ionicons name="trash-outline" size={18} color="#dc2626" />
              </Pressable>
            )}
          </View>
          <View>
            <Text className="text-xs font-bold text-slate-500 tracking-wider mb-1.5 uppercase">Từ mức tiền (₫)</Text>
            <TextInput
              className="h-12 bg-slate-50 dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-700 px-4 text-base text-slate-900 dark:text-white"
              placeholder="Vd: 10000000"
              placeholderTextColor="#94a3b8"
              keyboardType="numeric"
              value={tier.min_amount}
              onChangeText={(v) => updateTier(i, 'min_amount', v.replace(/\D/g, ''))}
            />
          </View>
          <View>
            <Text className="text-xs font-bold text-slate-500 tracking-wider mb-1.5 uppercase">Số chữ ký yêu cầu</Text>
            <View className="flex-row gap-2">
              {[1, 2, 3].map((n) => (
                <Pressable
                  key={n}
                  onPress={() => updateTier(i, 'required_approvals', n)}
                  className={`flex-1 h-11 rounded-xl border-1.5 items-center justify-center ${tier.required_approvals === n ? 'border-brand bg-cyan-50' : 'border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800'}`}
                >
                  <Text className={`text-sm font-semibold ${tier.required_approvals === n ? 'text-brand' : 'text-slate-500'}`}>
                    {n}
                  </Text>
                </Pressable>
              ))}
            </View>
          </View>
        </View>
      ))}

      <Pressable onPress={addTier} className="h-11 rounded-xl border-1.5 border-dashed border-cyan-300 items-center justify-center">
        <Text className="text-sm font-semibold text-brand">+ Thêm cấp phê duyệt</Text>
      </Pressable>

      <Pressable
        onPress={handleSave}
        disabled={guard.isPending}
        className={`h-13 bg-brand rounded-xl items-center justify-center mt-2 active:bg-brand-600 ${guard.isPending ? 'opacity-70' : ''}`}
      >
        {guard.isPending ? <ActivityIndicator color="#fff" /> : <Text className="text-base font-bold text-white">Lưu chính sách</Text>}
      </Pressable>
    </View>
  );
}

export default function AdminScreen() {
  const activeOrg = useOrgStore((s) => s.activeOrg);
  const [tab, setTab] = useState<Tab>('members');

  if (!activeOrg || activeOrg.role !== 'admin') {
    return (
      <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
        <View className="flex-1 items-center justify-center gap-4">
          <Text className="text-sm color-rose-600 font-semibold text-center">Chỉ Admin mới có thể truy cập màn hình này.</Text>
          <Pressable onPress={() => router.back()} className="px-5 py-2.5 bg-brand rounded-xl">
            <Text className="text-white font-bold">Quay lại</Text>
          </Pressable>
        </View>
      </SafeAreaView>
    );
  }

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
            Quản trị · {activeOrg.name}
          </Text>
          <View className="w-5" />
        </View>

        <View className="flex-row bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
          {(['members', 'policy'] as const).map((t) => (
            <Pressable
              key={t}
              onPress={() => setTab(t)}
              className={`flex-1 h-11 items-center justify-center ${tab === t ? 'border-b-2 border-brand' : ''}`}
            >
              <Text className={`text-sm font-semibold ${tab === t ? 'text-brand' : 'text-slate-400'}`}>
                {t === 'members' ? 'Thành viên' : 'Chính sách'}
              </Text>
            </Pressable>
          ))}
        </View>

        <ScrollView contentContainerStyle={{ padding: 16, paddingBottom: 40 }} keyboardShouldPersistTaps="handled">
          {tab === 'members' ? (
            <MembersTab orgId={activeOrg.orgId} />
          ) : (
            <PolicyTab orgId={activeOrg.orgId} />
          )}
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
