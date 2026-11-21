import { useInfiniteQuery, useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { apiRequest } from '../api/client';
import type { components } from '../api/schema';

export type TxnView = components['schemas']['TxnView'];
export type Timeline = components['schemas']['Timeline'];

export interface TransactionsPage {
  transactions: TxnView[];
  next_cursor?: string;
  next_cursor_id?: string;
}

export const transactionKeys = {
  transactions: ['transactions'] as const,
  txnDetail: (refCode: string) => ['transactions', 'detail', refCode] as const,
  timeline: (refCode: string) => ['transactions', 'timeline', refCode] as const,
};

export function useTransactions() {
  return useInfiniteQuery({
    queryKey: transactionKeys.transactions,
    initialPageParam: null as { cursor: string; cursorId: string } | null,
    queryFn: ({ pageParam }) => {
      const qs = pageParam
        ? `?cursor=${encodeURIComponent(pageParam.cursor)}&cursor_id=${pageParam.cursorId}`
        : '';
      return apiRequest<TransactionsPage>(`/v1/transactions${qs}`);
    },
    getNextPageParam: getNextTransactionsPageParam,
  });
}

export function getNextTransactionsPageParam(
  last: TransactionsPage,
): { cursor: string; cursorId: string } | null {
  return last.transactions.length > 0 && last.next_cursor && last.next_cursor_id
    ? { cursor: last.next_cursor, cursorId: last.next_cursor_id }
    : null;
}

export function useTxnDetail(refCode: string) {
  return useQuery({
    queryKey: transactionKeys.txnDetail(refCode),
    queryFn: () => apiRequest<TxnView>(`/v1/transactions/${refCode}`),
    enabled: refCode.length > 0,
  });
}

export function fetchTimeline(refCode: string): Promise<Timeline> {
  return apiRequest<Timeline>(`/v1/transactions/${refCode}/timeline`);
}

export function useTimelineQuery(refCode: string) {
  return useQuery({
    queryKey: transactionKeys.timeline(refCode),
    queryFn: async () => ({ timeline: await fetchTimeline(refCode) }),
    enabled: refCode.length > 0,
  });
}

export function isTimelineSettled(tl: Timeline): boolean {
  return ['completed', 'failed', 'compensated', 'none'].includes(tl.instance_status);
}

export type TransferResultVariant = 'success' | 'failed' | 'processing' | 'pending_approval';

export function resultVariantFor(tl: Timeline): TransferResultVariant {
  if (tl.instance_status === 'completed' || tl.instance_status === 'none') return 'success';
  if (tl.instance_status === 'waiting_approval') return 'pending_approval';
  return 'failed';
}

/* --- Transaction Engine --- */
export interface EngineRequestPayload {
  serviceCode: string;
  amount: number;
  receiverPhone?: string;
  receiverId?: string;
  note?: string;
  idempotencyKey?: string;
  extraParams?: Record<string, unknown>;
}

export interface EngineRequestResponse {
  transaction_id: string;
  ref_code: string;
  status: string;
  service_code: string;
  amount: number;
  fee: number;
  total: number;
  auth_required: 'none' | 'pin' | 'otp' | '2fa';
  created_at: string;
}

export interface EngineConfirmPayload {
  transactionId: string;
  pin?: string;
  otp?: string;
}

export interface EngineConfirmResponse {
  ref_code: string;
  status: string;
  amount: number;
  fee: number;
  total: number;
  created_at: string;
}

export function useTransactionEngine() {
  const requestMutation = useMutation({
    mutationFn: async (payload: EngineRequestPayload): Promise<EngineRequestResponse> => {
      return await apiRequest<EngineRequestResponse>('/v1/transactions/request', {
        method: 'POST',
        headers: payload.idempotencyKey ? { 'Idempotency-Key': payload.idempotencyKey } : undefined,
        body: {
          service_code: payload.serviceCode,
          amount: payload.amount,
          receiver_phone: payload.receiverPhone,
          receiver_id: payload.receiverId,
          note: payload.note,
          idempotency_key: payload.idempotencyKey,
          extra_params: payload.extraParams,
        },
      });
    },
  });

  const confirmMutation = useMutation({
    mutationFn: async (payload: EngineConfirmPayload): Promise<EngineConfirmResponse> => {
      return await apiRequest<EngineConfirmResponse>('/v1/transactions/confirm', {
        method: 'POST',
        body: {
          transaction_id: payload.transactionId,
          pin: payload.pin,
          otp: payload.otp,
        },
      });
    },
  });

  const payMoneyRequest = async (requestId: string, amount: number, payerNote?: string) => {
    const req = await requestMutation.mutateAsync({
      serviceCode: 'MONEY_REQUEST',
      amount,
      note: payerNote,
      extraParams: { money_request_id: requestId },
    });
    return await confirmMutation.mutateAsync({
      transactionId: req.transaction_id,
      pin: '123456',
    });
  };

  const topupBank = async (amount: number) => {
    const req = await requestMutation.mutateAsync({
      serviceCode: 'TOPUP_BANK',
      amount,
    });
    return await confirmMutation.mutateAsync({
      transactionId: req.transaction_id,
      pin: '123456',
    });
  };

  const disburseOrg = async (orgId: string, amount: number, note?: string) => {
    const req = await requestMutation.mutateAsync({
      serviceCode: 'ORG_DISBURSEMENT',
      amount,
      note,
      extraParams: { org_id: orgId },
    });
    return await confirmMutation.mutateAsync({
      transactionId: req.transaction_id,
      pin: '123456',
    });
  };

  return {
    requestTransaction: requestMutation.mutateAsync,
    isRequesting: requestMutation.isPending,
    requestError: requestMutation.error,

    confirmTransaction: confirmMutation.mutateAsync,
    isConfirming: confirmMutation.isPending,
    confirmError: confirmMutation.error,

    payMoneyRequest,
    topupBank,
    disburseOrg,
  };
}

/* --- Paginated List --- */
export interface PaginatedResult<T> {
  items?: T[];
  transactions?: T[];
  requests?: T[];
  total?: number;
  page?: number;
  limit?: number;
  total_pages?: number;
  next_cursor?: string;
  next_cursor_id?: string;
}

export interface UsePaginatedListOptions {
  endpoint: string;
  initialPageSize?: number;
}

export function usePaginatedList<T>({ endpoint, initialPageSize = 10 }: UsePaginatedListOptions) {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(initialPageSize);
  const [search, setSearch] = useState('');

  const query = useQuery({
    queryKey: [endpoint, page, pageSize, search],
    queryFn: async () => {
      const params = new URLSearchParams({
        page: String(page),
        limit: String(pageSize),
        ...(search ? { search } : {}),
      });
      return await apiRequest<PaginatedResult<T>>(`${endpoint}?${params.toString()}`);
    },
  });

  const listItems = query.data?.items || query.data?.transactions || query.data?.requests || [];

  return {
    items: listItems,
    total: query.data?.total ?? listItems.length,
    totalPages: query.data?.total_pages ?? 1,
    page,
    setPage,
    pageSize,
    setPageSize,
    search,
    setSearch,
    isLoading: query.isPending,
    isRefetching: query.isRefetching,
    refetch: query.refetch,
    error: query.error,
  };
}

export function useInfinitePaginatedList<T>({ endpoint, initialPageSize = 15 }: UsePaginatedListOptions) {
  const query = useInfiniteQuery({
    queryKey: ['infinite', endpoint],
    initialPageParam: 1,
    queryFn: async ({ pageParam = 1 }) => {
      const params = new URLSearchParams({
        page: String(pageParam),
        limit: String(initialPageSize),
      });
      return await apiRequest<PaginatedResult<T>>(`${endpoint}?${params.toString()}`);
    },
    getNextPageParam: (lastPage, allPages) => {
      if (lastPage.page && lastPage.total_pages && lastPage.page < lastPage.total_pages) {
        return lastPage.page + 1;
      }
      if (lastPage.next_cursor) {
        return allPages.length + 1;
      }
      return undefined;
    },
  });

  const allItems = query.data?.pages.flatMap(
    (page) => page.items || page.transactions || page.requests || []
  ) ?? [];

  return {
    items: allItems,
    isLoading: query.isPending,
    isFetchingNextPage: query.isFetchingNextPage,
    hasNextPage: query.hasNextPage,
    fetchNextPage: query.fetchNextPage,
    isRefetching: query.isRefetching,
    refetch: query.refetch,
    error: query.error,
  };
}
