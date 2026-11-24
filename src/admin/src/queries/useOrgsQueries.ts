import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { idempotencyHeaders } from '@/lib/idempotency';
import { type AdminQueryParams, type Paged, buildQueryString } from './common';

/** One approval level of a policy, exactly as stored by the backend. */
export interface OrgPolicyTier {
  level: number;
  required_approvals: number;
}

/** One version of an organization's approval policy (GET /v1/admin/orgs/{id}/policies). */
export interface OrgPolicy {
  id: string;
  name: string;
  version: number;
  isActive: boolean;
  /** Smallest amount the policy applies to, in cents. */
  minAmount: number;
  /** Largest amount it applies to; null means no upper limit. */
  maxAmount: number | null;
  tiers: OrgPolicyTier[];
}

export interface AdminOrgItem {
  org_id: string;
  name: string;
  role?: string;
  memberCount: number;
  policiesCount: number;
  pendingApprovalsCount: number;
}

interface BackendPolicyRow {
  id: string;
  name: string;
  version: number;
  is_active: boolean;
  min_amount: number;
  max_amount?: number | null;
  /** jsonb comes over the wire as base64 text (Go []byte). */
  tiers?: string | OrgPolicyTier[] | null;
}

function decodeTiers(v: BackendPolicyRow['tiers']): OrgPolicyTier[] {
  let raw: unknown = v;
  if (typeof v === 'string') {
    try {
      raw = JSON.parse(new TextDecoder().decode(Uint8Array.from(atob(v), (c) => c.charCodeAt(0))));
    } catch {
      raw = [];
    }
  }
  if (!Array.isArray(raw)) return [];
  return raw
    .map((t) => ({ level: Number((t as OrgPolicyTier).level), required_approvals: Number((t as OrgPolicyTier).required_approvals) }))
    .filter((t) => Number.isInteger(t.level) && Number.isInteger(t.required_approvals));
}

export function mapPolicyRow(r: BackendPolicyRow): OrgPolicy {
  return {
    id: r.id,
    name: r.name,
    version: r.version,
    isActive: r.is_active,
    minAmount: r.min_amount,
    maxAmount: r.max_amount ?? null,
    tiers: decodeTiers(r.tiers),
  };
}

/** The real policy versions of one organization. Disabled until an organization is chosen. */
export function useOrgPolicies(orgId: string | undefined) {
  return useQuery({
    queryKey: ['admin', 'org-policies', orgId],
    enabled: Boolean(orgId),
    queryFn: async (): Promise<OrgPolicy[]> => {
      const res = await apiRequest<{ policies: BackendPolicyRow[] | null }>(`/v1/admin/orgs/${orgId}/policies`);
      return (res?.policies ?? []).map(mapPolicyRow);
    },
  });
}
export function useOrgs(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ['admin', 'orgs', params],
    queryFn: async (): Promise<Paged<AdminOrgItem>> => {
      // GET /v1/admin/orgs (not /v1/org, which is scoped to the caller's own
      // org membership via a user-audience JWT and rejects an admin token).
      const res = await apiRequest<{
        organizations: {
          org_id: string;
          name: string;
          member_count: number;
          policies_count: number;
          pending_approvals_count: number;
        }[] | null;
        total?: number;
      }>(`/v1/admin/orgs${buildQueryString(params)}`);
      const items: AdminOrgItem[] = (res?.organizations ?? []).map((o) => ({
        org_id: o.org_id,
        name: o.name,
        memberCount: o.member_count,
        policiesCount: o.policies_count,
        pendingApprovalsCount: o.pending_approvals_count,
      }));
      return { rows: items, total: res?.total ?? items.length };
    },
  });
}
export function useSaveOrgPolicy() {
  const qc = useQueryClient();
  return useMutation({
    // PUT /v1/admin/orgs/{id}/policies creates a NEW policy version (201 {policy_id, version}) and
    // retires the previous one. Errors propagate: a failed save must never look like a success
    // (for example a real 409 ERR_POLICY_VERSION_CONFLICT).
    mutationFn: ({
      orgId,
      name,
      minAmount,
      maxAmount,
      tiers,
    }: {
      orgId: string;
      name: string;
      minAmount: number;
      maxAmount: number | null;
      tiers: OrgPolicyTier[];
    }) =>
      apiRequest<{ policy_id: string; version: number }>(`/v1/admin/orgs/${orgId}/policies`, {
        method: 'PUT',
        headers: idempotencyHeaders(),
        body: { name, min_amount: minAmount, max_amount: maxAmount, tiers },
      }),
    onSuccess: (_res, v) => {
      qc.invalidateQueries({ queryKey: ['admin', 'orgs'] });
      qc.invalidateQueries({ queryKey: ['admin', 'org-policies', v.orgId] });
    },
  });
}