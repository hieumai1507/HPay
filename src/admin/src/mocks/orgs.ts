// Demo data. Only loaded via a dynamic import guarded by
// `import.meta.env.DEV && MOCKS_ENABLED` (VITE_USE_MOCKS=true).
import type { AdminOrgItem } from '@/queries/useOrgsQueries';

export const MOCK_ORGS: AdminOrgItem[] = [
  {
    org_id: 'org-acme-corp',
    name: 'Acme Corp',
    memberCount: 12,
    policiesCount: 2,
    pendingApprovalsCount: 3,
    policies: [
      { id: 'pol-1', name: 'Large payments', version: 3, isActive: true, minAmount: 10000000, tiers: [{ level: 1, min_amount: 10000000, required_approvals: 1 }, { level: 2, min_amount: 40000000, required_approvals: 1 }] },
      { id: 'pol-2', name: 'Regular payments', version: 1, isActive: true, minAmount: 0, tiers: [{ level: 1, min_amount: 0, required_approvals: 1 }] },
    ],
  },
  {
    org_id: 'org-sandbox-studio',
    name: 'Sandbox Studio',
    memberCount: 4,
    policiesCount: 1,
    pendingApprovalsCount: 0,
    policies: [{ id: 'pol-3', name: 'General policy', version: 1, isActive: true, minAmount: 0, tiers: [{ level: 1, min_amount: 0, required_approvals: 1 }] }],
  },
];
