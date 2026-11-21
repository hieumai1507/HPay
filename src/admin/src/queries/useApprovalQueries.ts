import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { idempotencyHeaders } from '@/lib/idempotency';
import { type AdminQueryParams, type Paged, buildQueryString } from './common';

/** One row of the admin approval inbox (reversal / account_lock / policy_change). */
export interface AdminApprovalItem {
  id: string;
  subjectType: string;
  subjectId: string;
  currentLevel: number;
  createdBy: string;
  reason: string;
  /** Money facts, present only when subject_type is a reversal. */
  refCode: string;
  amount: number;
  fee: number;
  expiresAt: string;
  createdAt: string;
}

interface BackendApprovalRow {
  id: string;
  subject_type: string;
  subject_id: string;
  current_level: number;
  created_by: string;
  reason?: string;
  ref_code?: string;
  amount?: number;
  fee?: number;
  expires_at: string;
  created_at: string;
}

export const approvalKeys = {
  all: ['admin', 'approvals'] as const,
  list: (params?: AdminQueryParams) => ['admin', 'approvals', params] as const,
};

export function useAdminApprovals(params?: AdminQueryParams) {
  return useQuery({
    queryKey: approvalKeys.list(params),
    queryFn: async (): Promise<Paged<AdminApprovalItem>> => {
      const res = await apiRequest<{ approvals: BackendApprovalRow[] | null; total?: number }>(
        `/v1/admin/approvals${buildQueryString({ status: 'pending', ...params })}`
      );
      const rows = (res?.approvals ?? []).map((row) => ({
        id: row.id,
        subjectType: row.subject_type,
        subjectId: row.subject_id,
        currentLevel: row.current_level,
        createdBy: row.created_by,
        reason: row.reason ?? '',
        refCode: row.ref_code ?? '',
        amount: row.amount ?? 0,
        fee: row.fee ?? 0,
        expiresAt: row.expires_at,
        createdAt: row.created_at,
      }));
      return { rows, total: res?.total ?? rows.length };
    },
  });
}

export interface DecideApprovalInput {
  id: string;
  action: 'approve' | 'reject';
  reason: string;
}

/**
 * Finalises one approval. The backend serialises concurrent deciders and
 * answers 409 when someone finalised first (A01) — callers surface that as an
 * informational toast and refetch rather than treating it as an error.
 */
export function useDecideApproval() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, action, reason }: DecideApprovalInput) =>
      apiRequest(`/v1/admin/approvals/${id}/decide`, {
        method: 'POST',
        body: { action, reason },
        headers: idempotencyHeaders(),
      }),
    onSettled: () => {
      // Refetch either way: on 409 the row is already gone from the inbox.
      void qc.invalidateQueries({ queryKey: approvalKeys.all });
    },
  });
}
