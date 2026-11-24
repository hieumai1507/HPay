import { Ionicons } from '@expo/vector-icons';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import {
  ActivityIndicator,
  Alert,
  Pressable,
  ScrollView,
  Text,
  TextInput,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useQueryClient } from '@tanstack/react-query';

import { apiRequest } from '@/src/api/client';
import { useApprovalDetail, useApprovalInbox, useDecideApproval, approvalsKeys, type ApprovalSignature } from '@/src/queries';
import { formatAmountVND } from '@/src/lib/money';
import { describeError, isApiError } from '@/src/lib/errors';

const STATUS_LABEL: Record<string, { text: string; className: string }> = {
  pending: { text: 'Chờ duyệt', className: 'bg-amber-50 color-amber-700' },
  approved: { text: 'Đã duyệt', className: 'bg-emerald-50 color-emerald-700' },
  rejected: { text: 'Đã từ chối', className: 'bg-rose-50 color-rose-700' },
  expired: { text: 'Đã hết hạn', className: 'bg-slate-100 color-slate-500' },
  cancelled: { text: 'Đã hủy', className: 'bg-slate-100 color-slate-500' },
};

// Mockup 03-business.html frame 22 (success) / frame 23 (conflict) — a
// full-screen result that replaces the form after decide, not an Alert.
type DecideOutcome =
  | { kind: 'approved_final'; amount: number }
  | { kind: 'approved_pending'; remaining: number }
  | { kind: 'rejected' }
  | { kind: 'conflict'; message: string };

function DecideResultView({ outcome, inboxCount }: { outcome: DecideOutcome; inboxCount: number }) {
  if (outcome.kind === 'conflict') {
    return (
      <View className="flex-1 items-center justify-center px-8 gap-4">
        <View className="w-20 h-20 rounded-full bg-slate-100 dark:bg-slate-800 items-center justify-center">
          <Text className="text-4xl">🤝</Text>
        </View>
        <View className="items-center gap-1.5">
          <Text className="text-lg font-extrabold text-slate-900 dark:text-white text-center">Lệnh này vừa được xử lý xong</Text>
          <Text className="text-sm text-slate-500 text-center leading-relaxed">{outcome.message}</Text>
        </View>
        <View className="px-3 py-1 rounded-full bg-emerald-50">
          <Text className="text-xs font-bold color-emerald-700">Đã xử lý xong · đang thực hiện</Text>
        </View>
        <Pressable
          onPress={() => router.back()}
          className="w-full h-13 mt-3 border border-slate-200 dark:border-slate-700 rounded-xl items-center justify-center active:bg-slate-50"
        >
          <Text className="text-sm font-bold text-slate-700 dark:text-slate-200">
            Về hàng chờ{inboxCount > 0 ? ` (còn ${inboxCount} lệnh)` : ''}
          </Text>
        </Pressable>
      </View>
    );
  }

  if (outcome.kind === 'rejected') {
    return (
      <View className="flex-1 items-center justify-center px-8 gap-4">
        <View className="w-14 h-14 rounded-full bg-rose-600 items-center justify-center">
          <Ionicons name="close" size={26} color="#fff" />
        </View>
        <Text className="text-lg font-extrabold text-slate-900 dark:text-white text-center">Đã từ chối lệnh</Text>
        <Text className="text-sm text-slate-500 text-center">Lệnh đã bị từ chối và sẽ không được thực hiện.</Text>
        <Pressable onPress={() => router.back()} className="w-full h-13 mt-2 bg-brand rounded-xl items-center justify-center">
          <Text className="text-base font-bold text-white">Về hàng chờ</Text>
        </Pressable>
      </View>
    );
  }

  const isLastSignature = outcome.kind === 'approved_final';
  return (
    <View className="flex-1 items-center justify-center px-8 gap-4">
      <View className="w-14 h-14 rounded-full bg-emerald-600 items-center justify-center">
        <Ionicons name="checkmark" size={26} color="#fff" />
      </View>
      <Text className="text-lg font-extrabold text-slate-900 dark:text-white text-center">
        {isLastSignature ? 'Đã duyệt — lệnh đang thực hiện' : 'Đã ghi nhận chữ ký'}
      </Text>
      <Text className="text-sm text-slate-500 text-center leading-relaxed">
        {isLastSignature
          ? `Bạn là chữ ký cuối. Hệ thống đang chuyển ${formatAmountVND(BigInt(outcome.amount))}, maker và bạn sẽ nhận thông báo khi hoàn tất.`
          : `Còn chờ ${outcome.remaining} người duyệt nữa.`}
      </Text>
      <View className={`px-3 py-1 rounded-full ${isLastSignature ? 'bg-cyan-50' : 'bg-amber-50'}`}>
        <Text className={`text-xs font-bold ${isLastSignature ? 'color-cyan-700' : 'color-amber-700'}`}>
          {isLastSignature ? 'Đang xử lý' : 'Chờ duyệt'}
        </Text>
      </View>
      <Pressable onPress={() => router.back()} className="w-full h-13 mt-2 bg-brand rounded-xl items-center justify-center">
        <Text className="text-base font-bold text-white">Về hàng chờ</Text>
      </Pressable>
    </View>
  );
}

function SignatureRow({ sig }: { sig: ApprovalSignature }) {
  return (
    <View className="flex-row items-center justify-between py-3 border-t border-slate-100 dark:border-slate-700 first:border-t-0">
      <View className="flex-row items-center gap-2.5 flex-1">
        <View className={`w-8 h-8 rounded-full items-center justify-center ${sig.action === 'approve' ? 'bg-emerald-50' : 'bg-rose-50'}`}>
          <Ionicons
            name={sig.action === 'approve' ? 'checkmark' : 'close'}
            size={16}
            color={sig.action === 'approve' ? '#059669' : '#e11d48'}
          />
        </View>
        <View className="flex-1">
          <Text className="text-sm font-semibold text-slate-900 dark:text-white" numberOfLines={1}>
            {sig.checker_name} <Text className="font-normal text-slate-400">· L{sig.level}</Text>
          </Text>
          <Text className="text-xs text-slate-500">
            {sig.action === 'approve' ? 'Đã duyệt' : 'Đã từ chối'} lúc {new Date(sig.acted_at).toLocaleString('vi-VN')}
          </Text>
        </View>
      </View>
    </View>
  );
}

function OtpModal({
  onSubmit,
  onCancel,
  loading,
  action,
}: {
  onSubmit: (otp: string) => void;
  onCancel: () => void;
  loading: boolean;
  action: 'approve' | 'reject';
}) {
  const [otp, setOtp] = useState('');

  return (
    <View className="absolute inset-0 bg-black/50 justify-end z-50">
      <View className="bg-white dark:bg-slate-800 rounded-t-3xl p-7 pb-10 gap-4">
        <Text className="text-lg font-extrabold text-slate-900 dark:text-white text-center">Nhập OTP để ký duyệt</Text>
        <Text className="text-xs text-slate-500 text-center leading-relaxed">
          Mã đã gửi tới số điện thoại của bạn. Chữ ký này có giá trị pháp lý trong tổ chức.
        </Text>
        <View className="flex-row justify-center gap-2.5 my-2">
          {[0, 1, 2, 3, 4, 5].map((i) => (
            <View key={i} className={`w-11 h-13 rounded-xl border-2 items-center justify-center ${otp.length === i ? 'border-brand bg-cyan-50' : 'border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900'}`}>
              <Text className="text-xl font-bold text-slate-900 dark:text-white">{otp[i] ?? ''}</Text>
            </View>
          ))}
        </View>
        <TextInput
          className="absolute w-1 h-1 opacity-0"
          value={otp}
          onChangeText={(v) => setOtp(v.replace(/\D/g, '').slice(0, 6))}
          keyboardType="number-pad"
          autoFocus
          maxLength={6}
        />
        <Pressable
          onPress={() => { if (otp.length === 6) onSubmit(otp); }}
          disabled={otp.length < 6 || loading}
          className={`h-13 rounded-xl items-center justify-center ${action === 'reject' ? 'bg-rose-600' : 'bg-brand'} ${otp.length < 6 || loading ? 'opacity-50' : 'active:opacity-85'}`}
        >
          {loading ? (
            <ActivityIndicator color="#fff" />
          ) : (
            <Text className="text-base font-bold text-white">
              {action === 'approve' ? 'Xác nhận duyệt' : 'Xác nhận từ chối'}
            </Text>
          )}
        </Pressable>
        <Pressable onPress={onCancel} className="items-center py-2">
          <Text className="text-sm text-slate-500">Huỷ</Text>
        </Pressable>
      </View>
    </View>
  );
}

export default function ApprovalDetailScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const qc = useQueryClient();

  const [otpVisible, setOtpVisible] = useState(false);
  const [pendingAction, setPendingAction] = useState<'approve' | 'reject' | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [outcome, setOutcome] = useState<DecideOutcome | null>(null);

  const { data, isLoading, isError } = useApprovalDetail(id || '');
  const { data: inboxData } = useApprovalInbox();
  const decideMutation = useDecideApproval();

  const handleDecide = async (action: 'approve' | 'reject') => {
    try {
      setSubmitting(true);
      const challengeRes = await apiRequest<{ otp_dev?: string }>(
        `/v1/org/approvals/${id}/challenge`,
        { method: 'POST' },
      );
      setPendingAction(action);
      setOtpVisible(true);
      if (challengeRes.otp_dev) {
        Alert.alert('DEV OTP', `Mã OTP (chỉ hiển thị trong môi trường dev): ${challengeRes.otp_dev}`);
      }
    } catch (e: unknown) {
      Alert.alert('Lỗi', (e instanceof Error ? e.message : String(e)));
    } finally {
      setSubmitting(false);
    }
  };

  const handleOtpSubmit = async (otp: string) => {
    if (!pendingAction || !id || !data) return;
    try {
      setSubmitting(true);
      const res = await decideMutation.mutateAsync({
        id,
        action: pendingAction === 'approve' ? 'approve' : 'reject',
        otp,
      });
      setOtpVisible(false);
      await qc.invalidateQueries({ queryKey: approvalsKeys.inbox });

      if (pendingAction === 'reject') {
        setOutcome({ kind: 'rejected' });
      } else if (res.status === 'approved') {
        setOutcome({ kind: 'approved_final', amount: data.amount });
      } else {
        const remaining = Math.max(0, data.signatures_required - data.signatures.length - 1);
        setOutcome({ kind: 'approved_pending', remaining });
      }
    } catch (e: unknown) {
      if (isApiError(e) && (e.code === 'ERR_ALREADY_FINALIZED' || e.code === 'ERR_ALREADY_ACTED')) {
        setOtpVisible(false);
        await qc.invalidateQueries({ queryKey: approvalsKeys.inbox });
        setOutcome({ kind: 'conflict', message: describeError(e) });
        return;
      }
      Alert.alert('Lỗi', describeError(e));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      <View className="flex-row items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
        <Pressable onPress={() => router.back()} hitSlop={12}>
          <Ionicons name="arrow-back" size={22} color="#0f172a" />
        </Pressable>
        <Text className="text-base font-bold text-slate-900 dark:text-white flex-1 text-center mx-2" numberOfLines={1}>
          Duyệt lệnh{data ? ` #${data.ref_code}` : ''}
        </Text>
        <View className="w-5" />
      </View>

      {outcome ? (
        <DecideResultView outcome={outcome} inboxCount={inboxData?.approvals.length ?? 0} />
      ) : isLoading ? (
        <View className="flex-1 items-center justify-center"><ActivityIndicator size="large" color="#0E90A3" /></View>
      ) : isError || !data ? (
        <View className="flex-1 items-center justify-center">
          <Text className="text-sm color-rose-600 font-semibold">Không thể tải thông tin lệnh.</Text>
        </View>
      ) : (
        <ScrollView contentContainerStyle={{ padding: 16, gap: 16, paddingBottom: 40 }}>
          <View className="items-center py-3 gap-2">
            <Text className="text-3xl font-extrabold text-slate-900 dark:text-white">{formatAmountVND(BigInt(data.amount))}</Text>
            <Text className="text-xs text-slate-500">{data.org_name}</Text>
            <View className={`px-3 py-1 rounded-full ${STATUS_LABEL[data.status]?.className ?? 'bg-slate-100'}`}>
              <Text className={`text-xs font-bold ${STATUS_LABEL[data.status]?.className ?? ''}`}>
                {STATUS_LABEL[data.status]?.text ?? data.status}
                {data.status === 'pending' ? ` · ${data.signatures.length}/${data.signatures_required} chữ ký` : ''}
              </Text>
            </View>
          </View>

          <View className="bg-white dark:bg-slate-800 rounded-xl px-4 border border-slate-100 dark:border-slate-700 shadow-xs">
            <View className="flex-row justify-between items-center py-3.5">
              <Text className="text-xs font-semibold text-slate-500">Người tạo</Text>
              <Text className="text-sm font-medium text-slate-900 dark:text-white">{data.maker_name} (Maker)</Text>
            </View>
            <View className="flex-row justify-between items-center py-3.5 border-t border-slate-100 dark:border-slate-700">
              <Text className="text-xs font-semibold text-slate-500">Phí</Text>
              <Text className="text-sm font-medium text-slate-900 dark:text-white">{formatAmountVND(BigInt(data.fee))}</Text>
            </View>
            <View className="flex-row justify-between items-center py-3.5 border-t border-slate-100 dark:border-slate-700">
              <Text className="text-xs font-semibold text-slate-500">Cấp phê duyệt hiện tại</Text>
              <Text className="text-sm font-medium text-slate-900 dark:text-white">Level {data.current_level}</Text>
            </View>
            <View className="flex-row justify-between items-center py-3.5 border-t border-slate-100 dark:border-slate-700">
              <Text className="text-xs font-semibold text-slate-500">Hết hạn</Text>
              <Text className="text-sm font-medium text-slate-900 dark:text-white">
                {new Date(data.expires_at).toLocaleString('vi-VN')}
              </Text>
            </View>
          </View>

          {data.signatures.length > 0 && (
            <View className="bg-white dark:bg-slate-800 rounded-xl px-4 border border-slate-100 dark:border-slate-700 shadow-xs">
              <Text className="text-xs font-bold text-slate-500 pt-3.5 tracking-wide">CHỮ KÝ</Text>
              {data.signatures.map((sig, i) => (
                <SignatureRow key={`${sig.checker_id}-${i}`} sig={sig} />
              ))}
            </View>
          )}

          {data.status === 'pending' && (
            <>
              <Pressable
                onPress={() => handleDecide('approve')}
                disabled={submitting}
                className={`h-13 bg-brand rounded-xl items-center justify-center active:bg-brand-600 ${submitting ? 'opacity-70' : ''}`}
              >
                {submitting ? (
                  <ActivityIndicator color="#fff" />
                ) : (
                  <Text className="text-base font-bold text-white">Duyệt lệnh · nhận OTP</Text>
                )}
              </Pressable>
              <Pressable
                onPress={() => handleDecide('reject')}
                disabled={submitting}
                className={`h-13 border-1.5 border-rose-100 dark:border-slate-700 rounded-xl items-center justify-center active:bg-rose-50 ${submitting ? 'opacity-70' : ''}`}
              >
                <Text className="text-sm font-bold color-rose-600">Từ chối (nêu lý do)</Text>
              </Pressable>
            </>
          )}
        </ScrollView>
      )}

      {otpVisible && pendingAction && (
        <OtpModal
          action={pendingAction}
          onSubmit={handleOtpSubmit}
          onCancel={() => { setOtpVisible(false); setPendingAction(null); }}
          loading={submitting}
        />
      )}
    </SafeAreaView>
  );
}
