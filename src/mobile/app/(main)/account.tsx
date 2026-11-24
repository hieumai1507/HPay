import React, { useCallback } from 'react';
import {
  ActivityIndicator,
  Alert,
  Pressable,
  ScrollView,
  Switch,
  Text,
  View,
} from 'react-native';
import { useQuery } from '@tanstack/react-query';
import { useRouter } from 'expo-router';

import { apiRequest } from '@/src/api/client';
import { RESET_PIN_ROUTE, SET_PIN_ROUTE } from '@/src/features/security/pinErrors';
import { useMe } from '@/src/queries';
import { formatAmountUSD } from '@/src/lib/money';
import { useSecurityStore } from '@/src/stores/security';
import { clearStoredRefreshToken, useSessionStore } from '@/src/stores/session';
import { useOrgStore, type OrgItem } from '@/src/stores/org';

const orgKeys = { list: ['orgs', 'list'] as const };

function useMyOrgs() {
  return useQuery({
    queryKey: orgKeys.list,
    queryFn: () =>
      apiRequest<{ organizations: OrgItem[] }>('/v1/org'),
  });
}

function roleLabel(role: OrgItem['role']): string {
  switch (role) {
    case 'admin':   return 'Admin';
    case 'maker':   return 'Maker';
    case 'checker': return 'Checker';
  }
}

function orgInitials(name: string): string {
  return name
    .split(' ')
    .slice(0, 3)
    .map((w) => w[0] ?? '')
    .join('')
    .toUpperCase()
    .slice(0, 3);
}

const ORG_COLORS = [
  '#312E81', '#9A3412', '#065F46', '#7C2D12', '#1E3A5F', '#4A1942',
];
function orgColor(orgId: string): string {
  let hash = 0;
  for (const ch of orgId) hash = (hash * 31 + ch.charCodeAt(0)) | 0;
  return ORG_COLORS[Math.abs(hash) % ORG_COLORS.length]!;
}

function OrgListItem({ org }: { org: OrgItem }) {
  const switchOrg = useOrgStore((s) => s.switchOrg);
  const activeOrg = useOrgStore((s) => s.activeOrg);
  const router    = useRouter();
  const isActive  = activeOrg?.orgId === org.org_id;

  const handlePress = useCallback(() => {
    switchOrg({
      orgId:        org.org_id,
      name:         org.name,
      role:         org.role,
      checkerLevel: org.checker_level,
      walletId:     org.wallet_id,
      balance:      org.balance,
    });
    router.replace('/(main)/home');
  }, [org, switchOrg, router]);

  const bg = orgColor(org.org_id);
  const label = org.role === 'checker'
    ? `Role: Checker · Level ${org.checker_level}`
    : `Role: ${roleLabel(org.role)}`;

  return (
    <Pressable
      onPress={handlePress}
      className="flex-row items-center gap-3 py-3 border-t border-slate-100 dark:border-slate-800 active:opacity-70"
    >
      <View className="w-11 h-11 rounded-xl items-center justify-center" style={{ backgroundColor: bg }}>
        <Text className="text-xs font-bold text-white">{orgInitials(org.name)}</Text>
      </View>
      <View className="flex-1">
        <Text className="text-sm font-semibold text-slate-900 dark:text-white">{org.name}</Text>
        <Text className="text-xs text-slate-500 mt-0.5">{label}</Text>
      </View>
      {isActive ? (
        <View className="flex-row items-center gap-1.5 bg-cyan-50 px-2 py-1 rounded-full">
          <View className="w-1.5 h-1.5 rounded-full bg-brand" />
          <Text className="text-[11px] font-semibold text-brand">In use</Text>
        </View>
      ) : (
        <Text className="text-xl text-slate-300">›</Text>
      )}
    </Pressable>
  );
}

export default function AccountScreen() {
  const router = useRouter();
  const { data, isLoading, error } = useMe();
  const { biometricEnabled, setBiometricEnabled } = useSecurityStore();
  const clearSession = useSessionStore((s) => s.clearSession);
  const { data: orgsData, isLoading: orgsLoading } = useMyOrgs();

  const handleLogout = async () => {
    try {
      await apiRequest('/v1/auth/logout', { method: 'POST' });
    } catch {
      // Ignore network errors on logout
    }
    await clearStoredRefreshToken();
    clearSession('user');
    router.replace('/(auth)/login');
  };

  const handleCreateOrg = () => {
    Alert.prompt(
      'Create organization',
      'Enter your company / organization name:',
      async (name) => {
        if (!name || name.trim().length < 2) return;
        try {
          await apiRequest('/v1/org', {
            method: 'POST',
            body: JSON.stringify({ name: name.trim() }),
          });
        } catch (e: unknown) {
          Alert.alert('Error', (e instanceof Error ? e.message : String(e)));
        }
      },
      'plain-text',
    );
  };

  if (isLoading) {
    return (
      <View className="flex-1 items-center justify-center bg-slate-50 dark:bg-slate-900">
        <ActivityIndicator size="large" color="#0E90A3" />
      </View>
    );
  }

  if (error || !data) {
    return (
      <View className="flex-1 items-center justify-center bg-slate-50 dark:bg-slate-900">
        <Text className="text-base color-rose-600 font-semibold">Could not load account details.</Text>
      </View>
    );
  }

  const { user, txn_limit } = data;
  const orgs = orgsData?.organizations ?? [];

  const getKycBadge = (status: string) => {
    switch (status) {
      case 'verified':
      case 'auto_approved':
        return { bg: 'bg-emerald-50', text: 'text-emerald-700', dot: 'bg-emerald-600', label: 'Verified · Limit 100M/day' };
      case 'pending':
      case 'manual_review':
        return { bg: 'bg-amber-50', text: 'text-amber-700', dot: 'bg-amber-600', label: 'Processing' };
      case 'rejected':
        return { bg: 'bg-rose-50', text: 'text-rose-700', dot: 'bg-rose-600', label: 'Rejected' };
      default:
        return { bg: 'bg-slate-100', text: 'text-slate-500', dot: 'bg-slate-400', label: 'Not verified' };
    }
  };
  const badge = getKycBadge(user.kyc_status || 'none');

  return (
    <ScrollView className="flex-1 bg-slate-50 dark:bg-slate-900" contentContainerStyle={{ padding: 16, gap: 16, paddingBottom: 60 }}>
      {/* Profile Header */}
      <View className="flex-row items-center gap-3.5 bg-white dark:bg-slate-800 rounded-xl p-5 shadow-xs">
        <View className="w-16 h-16 rounded-full bg-purple-700 items-center justify-center">
          <Text className="text-xl font-bold text-white">{(user.full_name || 'H').slice(0, 2).toUpperCase()}</Text>
        </View>
        <View className="flex-1">
          <Text className="text-lg font-bold text-slate-900 dark:text-white mb-0.5">{user.full_name || ''}</Text>
          <Text className="text-xs font-mono text-slate-500 mb-1.5">{user.phone || ''}</Text>
          <View className={`flex-row items-center gap-1.5 self-start px-2 py-1 rounded-full ${badge.bg}`}>
            <View className={`w-1.5 h-1.5 rounded-full ${badge.dot}`} />
            <Text className={`text-[11px] font-semibold ${badge.text}`}>{badge.label}</Text>
          </View>
        </View>
      </View>

      {/* Business Wallets */}
      <View className="bg-white dark:bg-slate-800 rounded-xl px-4 pb-4 shadow-xs">
        <Text className="text-[11px] font-bold text-slate-400 tracking-wider pt-3.5 pb-1">ORGANIZATION WALLETS</Text>
        {orgsLoading ? (
          <ActivityIndicator size="small" color="#0E90A3" className="my-3" />
        ) : orgs.length === 0 ? (
          <Text className="text-xs text-slate-400 py-3">You have not joined any organization yet.</Text>
        ) : (
          orgs.map((org) => <OrgListItem key={org.org_id} org={org} />)
        )}
        <Pressable
          onPress={handleCreateOrg}
          className="mt-2.5 h-10 rounded-lg border-1.5 border-dashed border-cyan-300 items-center justify-center active:opacity-70"
        >
          <Text className="text-xs font-semibold text-brand">+ Create new organization</Text>
        </Pressable>
      </View>

      {/* KYC & Limit */}
      <View className="bg-white dark:bg-slate-800 rounded-xl px-4 pb-4 shadow-xs">
        <Text className="text-[11px] font-bold text-slate-400 tracking-wider pt-3.5 pb-1">ACCOUNT</Text>
        <View className="flex-row items-center justify-between py-3 border-t border-slate-100 dark:border-slate-800">
          <Text className="text-sm font-semibold text-slate-700 dark:text-slate-200">Transaction limit</Text>
          <Text className="text-sm font-bold text-slate-900 dark:text-white">{formatAmountUSD(BigInt(txn_limit))}</Text>
        </View>
      </View>

      {/* Security */}
      <View className="bg-white dark:bg-slate-800 rounded-xl px-4 pb-4 shadow-xs">
        <Text className="text-[11px] font-bold text-slate-400 tracking-wider pt-3.5 pb-1">SECURITY</Text>
        <View className="flex-row items-center justify-between py-3 border-t border-slate-100 dark:border-slate-800">
          <View className="flex-row items-center gap-3 flex-1 pr-4">
            <Text className="text-xl w-8">🫆</Text>
            <View>
              <Text className="text-sm font-semibold text-slate-700 dark:text-slate-200">Unlock with fingerprint / face</Text>
              <Text className="text-xs text-slate-400 mt-0.5">Use biometrics to open the app faster</Text>
            </View>
          </View>
          <Switch
            value={biometricEnabled}
            onValueChange={setBiometricEnabled}
            trackColor={{ false: '#e2e8f0', true: '#a5f3fc' }}
            thumbColor={biometricEnabled ? '#0E90A3' : '#94a3b8'}
          />
        </View>
      </View>

      {/* PIN */}
      <View className="bg-white dark:bg-slate-800 rounded-xl px-4 pb-4 shadow-xs">
        <Text className="text-[11px] font-bold text-slate-400 tracking-wider pt-3.5 pb-1">PIN</Text>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Set PIN"
          onPress={() => router.push(SET_PIN_ROUTE)}
          className="flex-row items-center justify-between py-3 border-t border-slate-100 dark:border-slate-800 active:opacity-70"
        >
          <Text className="text-sm font-semibold text-slate-700 dark:text-slate-200">Set PIN</Text>
          <Text className="text-xl text-slate-300">›</Text>
        </Pressable>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Change or reset PIN"
          onPress={() => router.push(RESET_PIN_ROUTE)}
          className="flex-row items-center justify-between py-3 border-t border-slate-100 dark:border-slate-800 active:opacity-70"
        >
          <Text className="text-sm font-semibold text-slate-700 dark:text-slate-200">Change or reset PIN</Text>
          <Text className="text-xl text-slate-300">›</Text>
        </Pressable>
      </View>

      {/* Two-step verification */}
      <View className="bg-white dark:bg-slate-800 rounded-xl px-4 shadow-xs">
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Two-step verification"
          onPress={() => router.push('/(main)/security/totp-status' as never)}
          className="flex-row items-center justify-between py-4 active:opacity-70"
        >
          <Text className="text-sm font-semibold text-slate-700 dark:text-slate-200">Two-step verification</Text>
          <Text className="text-xl text-slate-300">›</Text>
        </Pressable>
      </View>

      {/* Logout */}
      <Pressable
        onPress={handleLogout}
        className="h-12 rounded-lg bg-rose-50 border border-rose-100 items-center justify-center active:opacity-80"
      >
        <Text className="color-rose-600 text-sm font-bold">Log out</Text>
      </Pressable>
    </ScrollView>
  );
}
