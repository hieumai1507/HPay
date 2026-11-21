import { useMutation, useQuery } from '@tanstack/react-query';
import { apiRequest } from '../api/client';
import { queryClient } from '../api/query-client';

export type MoneyRequestStatus = 'pending' | 'paid' | 'declined' | 'canceled' | 'expired';

export interface InboxRequest {
  id: string;
  requester_name: string;
  amount: number;
  note?: string;
  split_group_id?: string | null;
  expires_at: string;
  last_reminded_at?: string | null;
}

export interface SentRequest {
  id: string;
  payer_name: string;
  amount: number;
  note?: string;
  split_group_id?: string | null;
  status: MoneyRequestStatus;
  expires_at: string;
  last_reminded_at?: string | null;
}

export interface MoneyRequestDetail {
  id: string;
  requester_name?: string;
  payer_name?: string;
  amount: number;
  note?: string;
  status: MoneyRequestStatus;
  paid_txn_id?: string | null;
  split_group_id?: string | null;
  expires_at: string;
  last_reminded_at?: string | null;
  siblings?: SentRequest[];
}

export const requestKeys = {
  inbox: ['money-requests', 'inbox'] as const,
  sent: ['money-requests', 'sent'] as const,
  detail: (id: string) => ['money-requests', 'detail', id] as const,
};

export function useRequestInbox() {
  return useQuery({
    queryKey: requestKeys.inbox,
    queryFn: () => apiRequest<{ requests: InboxRequest[] }>('/v1/money-requests/inbox'),
  });
}

export function useSentRequests() {
  return useQuery({
    queryKey: requestKeys.sent,
    queryFn: () => apiRequest<{ requests: SentRequest[] }>('/v1/money-requests/sent'),
  });
}

export function useRequestDetail(id: string) {
  return useQuery({
    queryKey: requestKeys.detail(id),
    queryFn: () => apiRequest<MoneyRequestDetail>(`/v1/money-requests/${id}`),
    enabled: id.length > 0,
  });
}

function invalidateRequestViews() {
  queryClient.invalidateQueries({ queryKey: ['money-requests'] });
}

export function useCreateRequest() {
  return useMutation({
    mutationFn: (input: {
      payerPhone: string;
      amount: bigint;
      note?: string;
      idempotencyKey: string;
    }) =>
      apiRequest<{ id: string; amount: number; status: MoneyRequestStatus; expires_at: string }>(
        '/v1/money-requests',
        {
          method: 'POST',
          body: {
            payer_phone: input.payerPhone,
            amount: input.amount,
            note: input.note || undefined,
          },
          idempotencyKey: input.idempotencyKey,
        },
      ),
    onSuccess: invalidateRequestViews,
  });
}

export function useCreateSplit() {
  return useMutation({
    mutationFn: (input: {
      payerPhones: string[];
      totalAmount: bigint;
      note?: string;
      amounts?: bigint[];
      idempotencyKey: string;
    }) =>
      apiRequest<{ split_group_id: string; requests: SentRequest[] }>('/v1/money-requests/split', {
        method: 'POST',
        body: {
          payer_phones: input.payerPhones,
          total_amount: input.totalAmount,
          note: input.note || undefined,
          amounts: input.amounts,
        },
        idempotencyKey: input.idempotencyKey,
      }),
    onSuccess: invalidateRequestViews,
  });
}

export function usePayRequest() {
  return useMutation({
    mutationFn: (input: { id: string; idempotencyKey: string }) =>
      apiRequest<{ ref_code: string }>(`/v1/money-requests/${input.id}/pay`, {
        method: 'POST',
        idempotencyKey: input.idempotencyKey,
      }),
    onSuccess: () => {
      invalidateRequestViews();
      queryClient.invalidateQueries({ queryKey: ['wallets'] });
      queryClient.invalidateQueries({ queryKey: ['transactions'] });
    },
  });
}

export function useDeclineRequest() {
  return useMutation({
    mutationFn: (id: string) =>
      apiRequest<{ status: MoneyRequestStatus }>(`/v1/money-requests/${id}/decline`, {
        method: 'POST',
      }),
    onSuccess: invalidateRequestViews,
  });
}

export function useCancelRequest() {
  return useMutation({
    mutationFn: (id: string) =>
      apiRequest<{ status: MoneyRequestStatus }>(`/v1/money-requests/${id}/cancel`, {
        method: 'POST',
      }),
    onSuccess: invalidateRequestViews,
  });
}

export function useRemindRequest() {
  return useMutation({
    mutationFn: (id: string) =>
      apiRequest<{ last_reminded_at: string }>(`/v1/money-requests/${id}/remind`, {
        method: 'POST',
      }),
    onSuccess: invalidateRequestViews,
  });
}

export const FINALIZED_REQUEST_CODES = [
  'ERR_REQUEST_NOT_PAYABLE',
  'ERR_REQUEST_EXPIRED',
  'ERR_REQUEST_NOT_FOUND',
] as const;

export function isRequestFinalizedCode(code: string): boolean {
  return (FINALIZED_REQUEST_CODES as readonly string[]).includes(code);
}

export function useMyQR() {
  return useQuery({
    queryKey: ['users', 'me', 'qr'],
    queryFn: () =>
      apiRequest<{ payload: string; signature: string; expires_at: string }>('/v1/users/me/qr'),
  });
}

export function useResolveQR() {
  return useMutation({
    mutationFn: (input: { payload: string; signature: string }) =>
      apiRequest<{
        user_id: string;
        phone_masked: string;
        name_masked: string;
        receiver_token: string;
      }>('/v1/qr/resolve', {
        method: 'POST',
        body: {
          payload: input.payload,
          signature: input.signature,
        },
      }),
  });
}
