import { useQuery } from '@tanstack/react-query';

import { apiRequest } from '@/api/client';

import { type AdminQueryParams, type Paged, buildQueryString } from './common';

/** G12: parked money cases that only a person can resolve. Read-only. */
export interface AttentionSummary {
  needsManualApprovals: number;
  compensationFailedWorkflows: number;
  total: number;
}

export interface NeedsManualApproval {
  id: string;
  subjectType: string;
  refCode: string;
  amount: number;
  reason: string;
  parkedAt: string;
}

interface BackendNeedsManualRow {
  id: string;
  subject_type: string;
  ref_code?: string;
  amount?: number;
  reason?: string;
  parked_at: string;
}

export const attentionKeys = {
  summary: ['admin', 'attention', 'summary'] as const,
  approvals: (params?: AdminQueryParams) => ['admin', 'attention', 'approvals', params] as const,
};

/** Polled so the badge appears without a reload. Pass enabled=false when the admin lacks the permission. */
export function useAttentionSummary(enabled = true) {
  return useQuery({
    queryKey: attentionKeys.summary,
    enabled,
    queryFn: async (): Promise<AttentionSummary> => {
      const res = await apiRequest<{
        needs_manual_approvals: number;
        compensation_failed_workflows: number;
        total: number;
      }>('/v1/admin/attention/summary');
      return {
        needsManualApprovals: res.needs_manual_approvals,
        compensationFailedWorkflows: res.compensation_failed_workflows,
        total: res.total,
      };
    },
    refetchInterval: 30_000,
  });
}

export function useNeedsManualApprovals(params?: AdminQueryParams) {
  return useQuery({
    queryKey: attentionKeys.approvals(params),
    queryFn: async (): Promise<Paged<NeedsManualApproval>> => {
      const res = await apiRequest<{ approvals: BackendNeedsManualRow[] | null; total?: number }>(
        `/v1/admin/approvals${buildQueryString({ ...params, status: 'needs_manual' })}`
      );
      const rows = (res?.approvals ?? []).map((r) => ({
        id: r.id,
        subjectType: r.subject_type,
        refCode: r.ref_code ?? '',
        amount: r.amount ?? 0,
        reason: r.reason ?? '',
        parkedAt: r.parked_at,
      }));
      return { rows, total: res?.total ?? rows.length };
    },
    refetchInterval: 30_000,
  });
}