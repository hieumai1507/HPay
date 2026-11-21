import { useMutation, useQuery } from '@tanstack/react-query';
import { apiRequest } from '../api/client';
import { queryClient } from '../api/query-client';
import type { components } from '../api/schema';

export type Wallet = components['schemas']['Wallet'];
export type TopupResult = components['schemas']['TopupResult'];
export type TransferResult = components['schemas']['TransferResult'];

export const walletKeys = {
  wallets: ['wallets'] as const,
  me: ['users', 'me'] as const,
  feePreview: (amount: string) => ['fees', 'preview', amount] as const,
};

export function useWallets() {
  return useQuery({
    queryKey: walletKeys.wallets,
    queryFn: () => apiRequest<{ wallets: Wallet[] }>('/v1/wallets'),
  });
}

export function useMainWallet() {
  const q = useWallets();
  return { ...q, wallet: q.data?.wallets.find((w) => w.purpose === 'main') ?? null };
}

export function useMe() {
  return useQuery({
    queryKey: walletKeys.me,
    queryFn: () =>
      apiRequest<{ user: components['schemas']['User']; txn_limit: number }>('/v1/users/me'),
    staleTime: 5 * 60_000,
  });
}

export function useFeePreview(amountMinorUnits: string | null) {
  return useQuery({
    queryKey: walletKeys.feePreview(amountMinorUnits ?? ''),
    queryFn: () =>
      apiRequest<{ amount: number; fee: number; total: number }>(
        `/v1/fees/preview?amount=${amountMinorUnits}`,
      ),
    enabled: amountMinorUnits !== null && amountMinorUnits.length > 0,
    staleTime: 60_000,
  });
}

export function useLookupReceiver() {
  return useMutation({
    mutationFn: (phone: string) =>
      apiRequest<{ found: boolean; masked_name: string }>(
        `/v1/users/lookup?phone=${encodeURIComponent(phone)}`,
      ),
  });
}

function invalidateMoneyViews() {
  queryClient.invalidateQueries({ queryKey: walletKeys.wallets });
  queryClient.invalidateQueries({ queryKey: ['transactions'] });
}

export function useTopup() {
  return useMutation({
    mutationFn: (input: { amount: bigint; idempotencyKey: string }) =>
      apiRequest<TopupResult>('/v1/wallets/topup', {
        method: 'POST',
        body: { amount: input.amount },
        idempotencyKey: input.idempotencyKey,
      }),
    onSuccess: invalidateMoneyViews,
  });
}

export function useTransfer() {
  return useMutation({
    mutationFn: (input: {
      receiverPhone?: string;
      receiverToken?: string;
      amount: bigint;
      note?: string;
      idempotencyKey: string;
    }) =>
      apiRequest<TransferResult>('/v1/transactions/transfer', {
        method: 'POST',
        body: {
          receiver_phone: input.receiverPhone || undefined,
          receiver_token: input.receiverToken || undefined,
          amount: input.amount,
          note: input.note || undefined,
        },
        idempotencyKey: input.idempotencyKey,
      }),
    onSuccess: invalidateMoneyViews,
  });
}
