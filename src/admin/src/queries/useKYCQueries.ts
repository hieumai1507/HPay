import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest, api } from '@/api/client';
import type { BadgeTone } from '@/ui';
import { type AdminQueryParams, type Paged, buildQueryString } from './common';

export type KYCTab = 'manual' | 'auto' | 'rejected';

export interface KYCQueueItem {
  id: string;
  user_id: string;
  full_name: string;
  phone: string;
  id_number: string;
  id_card_url: string;
  selfie_url: string;
  ai_score: number;
  ai_verdict: string;
  ai_reasons?: unknown;
  status: string;
  created_at: string;
}

export function useKYCQueueQuery(paramsInput: KYCTab | (AdminQueryParams & { tab?: KYCTab })) {
  const params: AdminQueryParams = typeof paramsInput === 'string' ? { tab: paramsInput } : paramsInput;
  return useQuery({
    queryKey: ['admin', 'kyc', 'queue', params],
    queryFn: async (): Promise<KYCQueueItem[]> => {
      try {
        const res = await apiRequest<{ items?: KYCQueueItem[]; requests?: KYCQueueItem[] }>(
          `/v1/admin/kyc${buildQueryString(params)}`
        );
        return res?.items || res?.requests || [];
      } catch {
        return [];
      }
    },
  });
}

export function useKYCDecisionMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, decision, reason }: { id: string; decision: 'approve' | 'reject'; reason?: string }) => {
      return await apiRequest(`/v1/admin/kyc/${id}/decision`, {
        method: 'POST',
        body: { decision, reason },
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'kyc'] });
    },
  });
}

export function useKYCQueue(paramsInput: KYCTab | (AdminQueryParams & { tab?: KYCTab })) {
  const params: AdminQueryParams = typeof paramsInput === 'string' ? { tab: paramsInput } : paramsInput;
  return useQuery({
    queryKey: ['kyc', params],
    queryFn: () => api<{ requests: KYCQueueItem[]; total?: number }>(`/v1/admin/kyc${buildQueryString(params)}`),
    select: (d): Paged<KYCQueueItem> => ({
      rows: d.requests ?? [],
      total: d.total ?? d.requests?.length ?? 0,
    }),
  });
}

interface ReviewInput {
  id: string;
  action: 'approve' | 'reject';
  reason?: string;
}

export function useKYCReview() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, action, reason }: ReviewInput) =>
      api<{ status: string }>(`/v1/admin/kyc/${id}/review`, {
        method: 'POST',
        body: { action, reason },
      }),
    onSettled: () => qc.invalidateQueries({ queryKey: ['kyc'] }),
  });
}

export function toneForVerdict(verdict: string): BadgeTone {
  if (!verdict) return 'neutral';
  const v = verdict.toLowerCase();
  if (v.includes('pass') || v.includes('approve') || v.includes('khớp')) return 'success';
  if (v.includes('warn') || v.includes('check') || v.includes('nghi')) return 'warning';
  if (v.includes('fail') || v.includes('reject') || v.includes('giả')) return 'danger';
  return 'neutral';
}

export function toneForStatus(status: string): BadgeTone {
  switch (status) {
    case 'verified':
    case 'auto_approved':
      return 'success';
    case 'manual_review':
      return 'warning';
    case 'rejected':
      return 'danger';
    default:
      return 'neutral';
  }
}
