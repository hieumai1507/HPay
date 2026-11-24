// Demo data. Only loaded via a dynamic import guarded by
// `import.meta.env.DEV && MOCKS_ENABLED` (VITE_USE_MOCKS=true).
import type { AuditLogItem } from '@/queries/useAuditQueries';

export const MOCK_AUDIT: AuditLogItem[] = [
  { id: 'aud-101', timestamp: '14:23:01', actor: 'admin.098812', actorRole: 'super_admin', action: 'POLICY_UPDATE', actionType: 'warning', resource: 'OrgPolicy #3', details: 'Payroll Limit changed', ip: '118.69.182.10' },
  { id: 'aud-102', timestamp: '14:15:44', actor: 'system', actorRole: 'system', action: 'RECON_MATCH', actionType: 'success', resource: 'ReconJob #992', details: 'Matched 100% of 1,420 Bank Transactions', ip: '127.0.0.1' },
  { id: 'aud-103', timestamp: '13:58:12', actor: 'user.091234', actorRole: 'customer', action: 'TRANSFER_P2P', actionType: 'info', resource: 'Txn #TX8812', details: 'P2P Transfer 500,000 ₫', ip: '14.225.21.19' },
];
