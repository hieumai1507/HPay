import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { MOCKS_ENABLED } from '@/lib/env';
import { idempotencyHeaders } from '@/lib/idempotency';
import { type AdminQueryParams, type Paged, buildQueryString } from './common';

export interface OrgPolicyTier {
  level: number;
  min_amount: number;
  required_approvals: number;
}

export interface OrgPolicy {
  id: string;
  name: string;
  version: number;
  isActive: boolean;
  minAmount: number;
  tiers: OrgPolicyTier[];
}

export interface AdminOrgItem {
  org_id: string;
  name: string;
  role?: string;
  memberCount: number;
  policiesCount: number;
  pendingApprovalsCount: number;
  /** GET /v1/admin/orgs does not return policy bodies, so real data has an empty list here. */
  policies: OrgPolicy[];
}

export function useOrgs(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ['admin', 'orgs', params],
    queryFn: async (): Promise<Paged<AdminOrgItem>> => {
      try {
        // GET /v1/admin/orgs (not /v1/org, which is scoped to the caller's own
        // org membership via a user-audience JWT and rejects an admin token).
        const res = await apiRequest<{
          organizations: Array<{
            org_id: string;
            name: string;
            member_count: number;
            policies_count: number;
            pending_approvals_count: number;
          }> | null;
          total?: number;
        }>(`/v1/admin/orgs${buildQueryString(params)}`);
        const items: AdminOrgItem[] = (res?.organizations ?? []).map((o) => ({
          org_id: o.org_id,
          name: o.name,
          memberCount: o.member_count,
          policiesCount: o.policies_count,
          pendingApprovalsCount: o.pending_approvals_count,
          policies: [],
        }));
        return { rows: items, total: res.total ?? items.length };
      } catch (err) {
        // Mock data: only in a Vite dev server with VITE_USE_MOCKS=true.
        if (import.meta.env.DEV && MOCKS_ENABLED) {
          const { MOCK_ORGS } = await import('@/mocks/orgs');
          return { rows: MOCK_ORGS, total: MOCK_ORGS.length };
        }
        throw err;
      }
    },
  });
}

export function useSaveOrgPolicy() {
  const qc = useQueryClient();
  return useMutation({
    // PUT /v1/admin/orgs/{id}/policies creates a NEW policy version (201
    // {policy_id, version}). Errors propagate: a failed save must never look
    // like a success (e.g. a real 409 ERR_POLICY_VERSION_CONFLICT).
    mutationFn: ({ orgId, name, tiers }: { orgId: string; name: string; tiers: OrgPolicyTier[] }) =>
      apiRequest<{ policy_id: string; version: number }>(`/v1/admin/orgs/${orgId}/policies`, {
        method: 'PUT',
        headers: idempotencyHeaders(),
        body: {
          name,
          min_amount: tiers[0]?.min_amount ?? 0,
          max_amount: null,
          tiers,
        },
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'orgs'] });
    },
  });
}
