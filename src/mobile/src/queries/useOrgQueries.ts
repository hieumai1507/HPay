import { useMutation, useQuery } from '@tanstack/react-query';
import { apiRequest } from '../api/client';
import { queryClient } from '../api/query-client';

export interface OrgDisbursementPayload {
  orgId: string;
  recipientPhone: string;
  amount: number;
  note?: string;
  idempotencyKey?: string;
}

export interface ApprovalItem {
  id: string;
  org_name: string;
  ref_code: string;
  amount: number;
  fee: number;
  maker_name: string;
  current_level: number;
  expires_at: string;
}

export interface ApprovalSignature {
  checker_id: string;
  checker_name: string;
  level: number;
  action: 'approve' | 'reject';
  acted_at: string;
}

// GET /v1/org/approvals/{id} — hpay-backend docs/plans/approval-detail-endpoint.
export interface ApprovalDetail {
  id: string;
  status: 'pending' | 'approved' | 'rejected' | 'expired' | 'cancelled';
  current_level: number;
  org_id: string;
  org_name: string;
  ref_code: string;
  amount: number;
  fee: number;
  maker_id: string;
  maker_name: string;
  created_at: string;
  expires_at: string;
  finalized_at?: string;
  policy_name: string;
  signatures_required: number;
  signatures: ApprovalSignature[];
}

export const approvalsKeys = {
  inbox: ['org', 'approvals', 'inbox'] as const,
  detail: (id: string) => ['org', 'approvals', 'detail', id] as const,
};

export function useApprovalInbox() {
  return useQuery({
    queryKey: approvalsKeys.inbox,
    queryFn: () => apiRequest<{ approvals: ApprovalItem[] }>('/v1/org/approvals'),
  });
}

export function useApprovalDetail(id: string) {
  return useQuery({
    queryKey: approvalsKeys.detail(id),
    queryFn: () => apiRequest<ApprovalDetail>(`/v1/org/approvals/${id}`),
    enabled: !!id,
  });
}

// For the maker: CreateDisbursement only returns ref_code synchronously — the
// approval_request itself is created later by the backend's pause hook, so
// this resolves it by ref_code and polls briefly until it exists.
export function useApprovalDetailByRefCode(refCode: string) {
  return useQuery({
    queryKey: ['org', 'approvals', 'by-ref', refCode],
    queryFn: () => apiRequest<ApprovalDetail>(`/v1/org/approvals/by-ref/${refCode}`),
    enabled: !!refCode,
    refetchInterval: (query) => (query.state.data?.status === 'pending' ? 4000 : false),
    retry: 5,
    retryDelay: 2000,
  });
}

// Matches hpay-backend approval.DecideResult.
export interface DecideResult {
  status: 'pending' | 'approved' | 'rejected';
  current_level?: number;
}

export function useDecideApproval() {
  return useMutation({
    mutationFn: (input: { id: string; action: 'approve' | 'reject'; otp: string }) =>
      apiRequest<DecideResult>(`/v1/org/approvals/${input.id}/decide`, {
        method: 'POST',
        body: JSON.stringify({ action: input.action, otp: input.otp }),
      }),
    onSuccess: (_data, input) => {
      queryClient.invalidateQueries({ queryKey: approvalsKeys.inbox });
      queryClient.invalidateQueries({ queryKey: approvalsKeys.detail(input.id) });
    },
  });
}

// Maker withdraws their own still-pending request (mockup "Hủy lệnh").
export function useCancelApproval() {
  return useMutation({
    mutationFn: (input: { id: string; refCode?: string }) =>
      apiRequest<{ status: string }>(`/v1/org/approvals/${input.id}/cancel`, { method: 'POST' }),
    onSuccess: (_data, input) => {
      queryClient.invalidateQueries({ queryKey: approvalsKeys.detail(input.id) });
      if (input.refCode) {
        queryClient.invalidateQueries({ queryKey: ['org', 'approvals', 'by-ref', input.refCode] });
      }
    },
  });
}

// Matches hpay-backend org.DisbursementResult.
export interface OrgDisbursementResult {
  ref_code: string;
  status: string;
  amount: number;
  fee: number;
  total: number;
  created_at: string;
  duplicate?: boolean;
}

export function useDisburseOrgMoney() {
  return useMutation({
    mutationFn: (input: OrgDisbursementPayload) =>
      apiRequest<OrgDisbursementResult>(`/v1/org/${input.orgId}/transactions`, {
        method: 'POST',
        headers: input.idempotencyKey ? { 'Idempotency-Key': input.idempotencyKey } : undefined,
        body: JSON.stringify({
          phone: input.recipientPhone,
          amount: input.amount,
          note: input.note,
        }),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: approvalsKeys.inbox });
      queryClient.invalidateQueries({ queryKey: ['wallets'] });
    },
  });
}
