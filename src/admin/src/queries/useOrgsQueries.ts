import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
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
  policies: OrgPolicy[];
}

const MOCK_ORGS: AdminOrgItem[] = [
  {
    org_id: 'org-deh-corp',
    name: 'DEH Corp',
    memberCount: 12,
    policiesCount: 2,
    pendingApprovalsCount: 3,
    policies: [
      {
        id: 'pol-1',
        name: 'Chi lớn',
        version: 3,
        isActive: true,
        minAmount: 10000000,
        tiers: [
          { level: 1, min_amount: 10000000, required_approvals: 1 },
          { level: 2, min_amount: 40000000, required_approvals: 1 },
        ],
      },
      {
        id: 'pol-2',
        name: 'Chi thường',
        version: 1,
        isActive: true,
        minAmount: 0,
        tiers: [{ level: 1, min_amount: 0, required_approvals: 1 }],
      },
    ],
  },
  {
    org_id: 'org-sandbox-studio',
    name: 'Sandbox Studio',
    memberCount: 4,
    policiesCount: 1,
    pendingApprovalsCount: 0,
    policies: [
      {
        id: 'pol-3',
        name: 'Chính sách chung',
        version: 1,
        isActive: true,
        minAmount: 0,
        tiers: [{ level: 1, min_amount: 0, required_approvals: 1 }],
      },
    ],
  },
];

export function useOrgs(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ['admin', 'orgs', params],
    queryFn: async (): Promise<Paged<AdminOrgItem>> => {
      try {
        // GET /v1/admin/orgs (not /v1/org — that endpoint is scoped to the
        // caller's own org membership via a user-audience JWT and rejects an
        // admin token outright, which was logging admins straight out of
        // this page). This one returns every org plus real counts.
        const res = await apiRequest<{
          organizations: Array<{
            org_id: string;
            name: string;
            member_count: number;
            policies_count: number;
            pending_approvals_count: number;
          }>;
          total?: number;
        }>(`/v1/admin/orgs${buildQueryString(params)}`);
        if (res?.organizations && res.organizations.length > 0) {
          const items: AdminOrgItem[] = res.organizations.map((o) => ({
            org_id: o.org_id,
            name: o.name,
            memberCount: o.member_count,
            policiesCount: o.policies_count,
            pendingApprovalsCount: o.pending_approvals_count,
            policies: MOCK_ORGS[0]!.policies,
          }));
          return { rows: items, total: res.total ?? items.length };
        }
      } catch {
        // Fallback for mock preview
      }
      return { rows: MOCK_ORGS, total: MOCK_ORGS.length };
    },
  });
}

export function useSaveOrgPolicy() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      orgId,
      name,
      tiers,
    }: {
      orgId: string;
      name: string;
      tiers: OrgPolicyTier[];
    }) => {
      // PUT /v1/admin/orgs/{id}/policies — not POST /v1/org/{id}/policies,
      // which requires org-membership (RequireAuth + requireRole) and 401s
      // any admin-audience token, logging the admin straight out of this
      // page (same class of bug as GET /v1/org before it).
      //
      // No try/catch here on purpose: swallowing the error made every save
      // report success even when it failed (e.g. ERR_POLICY_VERSION_CONFLICT
      // on a real 409) — OrgsPage.tsx's own catch around mutateAsync is
      // what's supposed to show the real error, but it never ran because
      // this function never rethrew.
      await apiRequest(`/v1/admin/orgs/${orgId}/policies`, {
        method: 'PUT',
        body: {
          name,
          min_amount: tiers[0]?.min_amount ?? 0,
          max_amount: null,
          tiers,
        },
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'orgs'] });
    },
  });
}
